package account

import (
	"context"
	"errors"
	"net/http"
	"strings"

	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

var (
	accountReadSelfPermission    = permission.MustDefinitionByCode("account.read.self")
	accountUpdateSelfPermission  = permission.MustDefinitionByCode("account.update.self")
	accountDeleteSelfPermission  = permission.MustDefinitionByCode("account.delete.self")
	passwordUpdateSelfPermission = permission.MustDefinitionByCode("account_password.update.self")
)

func (s AccountService) Me(ctx context.Context, actor permission.Actor) (map[string]any, error) {
	if err := actor.Require(accountReadSelfPermission); err != nil {
		return nil, permissionDenied()
	}
	u, err := s.DB.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	profileCount, err := s.DB.Profiles.CountByUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	textureCount, err := s.DB.Textures.CountForUser(ctx, actor.UserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":            u.ID,
		"email":         u.Email,
		"lang":          u.PreferredLanguage,
		"display_name":  u.DisplayName,
		"banned_until":  u.BannedUntil,
		"avatar_hash":   u.AvatarHash,
		"permissions":   actor.PermissionCodes(),
		"profile_count": profileCount,
		"texture_count": textureCount,
	}, nil
}

func (s AccountService) UpdateSelf(ctx context.Context, actor permission.Actor, body map[string]any) error {
	if err := actor.Require(accountUpdateSelfPermission); err != nil {
		return permissionDenied()
	}
	fields, err := s.normalizedSelfUpdateFields(ctx, actor.UserID, body)
	if err != nil {
		return err
	}
	if err := s.DB.Users.Update(ctx, actor.UserID, fields); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
		}
		if userstore.IsEmailConflict(err) {
			return util.HTTPError{Status: http.StatusBadRequest, Detail: "Email already in use"}
		}
		if errors.Is(err, userstore.ErrDisplayNameConflict) {
			return util.HTTPError{Status: http.StatusBadRequest, Detail: "Username already exists"}
		}
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) ChangePasswordSelf(ctx context.Context, actor permission.Actor, oldPassword, newPassword string) error {
	if err := actor.Require(passwordUpdateSelfPermission); err != nil {
		return permissionDenied()
	}
	u, err := s.DB.Users.GetByID(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if u == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "用户不存在"}
	}
	if !util.VerifyPassword(oldPassword, u.Password) {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "旧密码错误"}
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, actor.UserID); err != nil {
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, actor.UserID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "用户不存在"}
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) DeleteSelf(ctx context.Context, actor permission.Actor) error {
	if err := actor.Require(accountDeleteSelfPermission); err != nil {
		return permissionDenied()
	}
	hasProtectedRole, err := s.DB.Permissions.UserHasProtectedRole(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if hasProtectedRole {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected role holders cannot delete their own account"}
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, actor.UserID); err != nil {
		return err
	}
	ok, err := s.DB.Users.Delete(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	return s.Redis.InvalidateAuthUser(ctx, actor.UserID)
}

func (s AccountService) normalizedSelfUpdateFields(ctx context.Context, userID string, body map[string]any) (map[string]any, error) {
	fields := map[string]any{}
	if v, ok := body["email"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if !util.ValidEmail(v) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid email format"}
		}
		existing, err := s.DB.Users.GetByEmail(ctx, v)
		if err != nil {
			return nil, err
		}
		if existing != nil && existing.ID != userID {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Email already in use"}
		}
		fields["email"] = v
	}
	if v, ok := body["display_name"].(string); ok && v != "" {
		v = strings.TrimSpace(v)
		if v == "" {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Username cannot be empty"}
		}
		taken, err := s.DB.Users.IsDisplayNameTaken(ctx, v, userID)
		if err != nil {
			return nil, err
		}
		if taken {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Username already exists"}
		}
		fields["display_name"] = v
	}
	if v, ok := body["preferred_language"].(string); ok && v != "" {
		fields["preferred_language"] = v
	}
	if v, ok := body["avatar_hash"]; ok {
		fields["avatar_hash"] = v
	}
	return fields, nil
}
