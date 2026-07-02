package admin

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/util"
)

const (
	accountBanReasonMaxRunes = 500
	accountBanNoticeTTL      = 30 * 24 * time.Hour
)

var (
	accountBanPermission   = permission.MustDefinitionByCode("account.ban.any")
	accountUnbanPermission = permission.MustDefinitionByCode("account.unban.any")
)

type AccountService struct {
	DB    *database.DB
	Redis redisstore.Store
}

type BanUserInput struct {
	BannedUntil int64
	Reason      string
}

func (s AccountService) BanUser(ctx context.Context, actor permission.Actor, targetID string, input BanUserInput) (int64, error) {
	if err := actor.Require(accountBanPermission); err != nil {
		return 0, permissionDenied()
	}
	if input.BannedUntil < time.Now().Add(-24*time.Hour).UnixMilli() {
		return 0, util.HTTPError{Status: http.StatusBadRequest, Detail: "banned_until is required"}
	}
	reason, err := normalizedBanReason(input.Reason)
	if err != nil {
		return 0, err
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return 0, err
	}
	if err := s.DB.Users.Ban(ctx, target.ID, input.BannedUntil); err != nil {
		if database.IsNoRows(err) {
			return 0, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
		}
		return 0, err
	}
	if err := s.Redis.InvalidateAuthUser(ctx, target.ID); err != nil {
		return 0, err
	}
	if err := s.createBanNotice(ctx, actor.UserID, target.ID, input.BannedUntil, reason); err != nil {
		return 0, err
	}
	return input.BannedUntil, nil
}

func (s AccountService) UnbanUser(ctx context.Context, actor permission.Actor, targetID string) error {
	if err := actor.Require(accountUnbanPermission); err != nil {
		return permissionDenied()
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return err
	}
	if err := s.DB.Users.Unban(ctx, target.ID); err != nil {
		if database.IsNoRows(err) {
			return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
		}
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, target.ID)
}

func (s AccountService) modifiableUser(ctx context.Context, actor permission.Actor, targetID string) (*model.User, error) {
	target, err := s.DB.Users.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	hasProtectedRole, err := s.DB.Permissions.UserHasProtectedRole(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	if hasProtectedRole && !actor.Has(manageProtectedPermission) {
		return nil, util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify super admin"}
	}
	return target, nil
}

func (s AccountService) createBanNotice(ctx context.Context, actorID, targetID string, bannedUntil int64, reason string) error {
	endsAt := database.NowMS() + accountBanNoticeTTL.Milliseconds()
	content := fmt.Sprintf("你的账号已被管理员封禁。\n\n封禁截止时间：%d\n\n原因：\n\n%s", bannedUntil, reason)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "账号已被封禁",
		Summary:         "你的账号已被管理员封禁，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelDanger,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	}, actorID)
	return err
}

func normalizedBanReason(raw string) (string, error) {
	reason := strings.TrimSpace(raw)
	if reason == "" {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason is required"}
	}
	if len([]rune(reason)) > accountBanReasonMaxRunes {
		return "", util.HTTPError{Status: http.StatusBadRequest, Detail: "reason too long"}
	}
	return reason, nil
}
