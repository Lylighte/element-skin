package account

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	noticesvc "element-skin/backend/internal/service/notice"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

const (
	accountBanReasonMaxRunes = 500
	accountBanNoticeTTL      = 30 * 24 * time.Hour
	accountPermissionTTL     = 30 * 24 * time.Hour
)

var (
	manageProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")
	permissionGrantAny        = permission.MustDefinitionByCode("permission.grant.any")
	permissionRevokeAny       = permission.MustDefinitionByCode("permission.revoke.any")
	accountBanPermission      = permission.MustDefinitionByCode("account.ban.any")
	accountUnbanPermission    = permission.MustDefinitionByCode("account.unban.any")
	accountDeletePermission   = permission.MustDefinitionByCode("account.delete.any")
	accountUpdatePermission   = permission.MustDefinitionByCode("account.update.any")
	userReadAnyPermission     = permission.MustDefinitionByCode("user.read.any")
	accountReadAnyPermission  = permission.MustDefinitionByCode("account.read.any")
)

type AccountService struct {
	DB    *database.DB
	Redis redisstore.Store
}

type BanUserInput struct {
	BannedUntil int64
	Reason      string
}

type ResetPasswordInput struct {
	UserID      string
	NewPassword string
}

func (s AccountService) ListUsers(ctx context.Context, actor permission.Actor, cursor string, limit int, query string) (map[string]any, error) {
	if err := actor.Require(userReadAnyPermission); err != nil {
		return nil, permissionDenied()
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	res, err := s.DB.Users.List(ctx, limit, last, strings.TrimSpace(query))
	if err != nil {
		return nil, err
	}
	if err := s.attachRoles(ctx, res["items"]); err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s AccountService) UserDetail(ctx context.Context, actor permission.Actor, userID string) (map[string]any, error) {
	if err := actor.Require(accountReadAnyPermission); err != nil {
		return nil, permissionDenied()
	}
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	out := userstore.PublicUser(*user)
	roles, err := s.DB.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	protected, err := s.DB.Permissions.UserIsProtected(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	out["roles"] = roles
	out["protected"] = protected
	return out, nil
}

func (s AccountService) GrantUserRole(ctx context.Context, actor permission.Actor, targetID, roleID string) error {
	if roleID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "role_id required"}
	}
	if err := actor.Require(permissionGrantAny); err != nil {
		return permissionDenied()
	}
	if err := ensureRoleMutationAllowed(actor, roleID); err != nil {
		return err
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	if err := s.ensureProtectedSubjectMutationAllowed(ctx, actor, targetID); err != nil {
		return err
	}
	if err := s.DB.Permissions.GrantRole(ctx, targetID, roleID, actor.SubjectID); err != nil {
		return err
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createRoleChangeNotice(ctx, targetID, roleID, "grant"); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s AccountService) RevokeUserRole(ctx context.Context, actor permission.Actor, targetID, roleID string) error {
	if roleID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "role_id required"}
	}
	if err := actor.Require(permissionRevokeAny); err != nil {
		return permissionDenied()
	}
	if err := ensureRoleMutationAllowed(actor, roleID); err != nil {
		return err
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	if err := s.ensureProtectedSubjectMutationAllowed(ctx, actor, targetID); err != nil {
		return err
	}
	ok, err := s.DB.Permissions.RevokeRole(ctx, targetID, roleID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "role assignment not found"}
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createRoleChangeNotice(ctx, targetID, roleID, "revoke"); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s AccountService) TransferProtectedSubject(ctx context.Context, actor permission.Actor, targetID string) error {
	if targetID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "user_id required"}
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot transfer protected subject to yourself"}
	}
	if err := actor.Require(manageProtectedPermission); err != nil {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected subject management required"}
	}
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isProtected {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected subject ownership required"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	affectedUserIDs, err := s.DB.Permissions.TransferProtectedSubject(ctx, actor.UserID, targetID, actor.SubjectID)
	if err != nil {
		return err
	}
	for _, userID := range affectedUserIDs {
		if err := s.Redis.InvalidateAuthUser(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s AccountService) DeleteUser(ctx context.Context, actor permission.Actor, targetID string) error {
	if err := actor.Require(accountDeletePermission); err != nil {
		return permissionDenied()
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot delete yourself"}
	}
	target, err := s.modifiableUser(ctx, actor, targetID)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, target.ID); err != nil {
		return err
	}
	if err := s.deleteUserOAuthData(ctx, target.ID); err != nil {
		return err
	}
	ok, err := s.DB.Users.Delete(ctx, target.ID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	return s.Redis.InvalidateAuthUser(ctx, target.ID)
}

func (s AccountService) reconcileOAuthAfterUserPermissionChange(ctx context.Context, userID string) error {
	_, err := (oauthsvc.Service{DB: s.DB, Redis: s.Redis}).ReconcileUserPermissionDependents(ctx, userID)
	return err
}

func (s AccountService) deleteUserOAuthData(ctx context.Context, userID string) error {
	_, err := (oauthsvc.Service{DB: s.DB, Redis: s.Redis}).DeleteUserOAuthData(ctx, userID)
	return err
}

func (s AccountService) ResetPassword(ctx context.Context, actor permission.Actor, input ResetPasswordInput) error {
	if err := actor.Require(accountUpdatePermission); err != nil {
		return permissionDenied()
	}
	userID := strings.TrimSpace(input.UserID)
	newPassword := input.NewPassword
	if userID == "" || newPassword == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "user_id and new_password required"}
	}
	target, err := s.modifiableUser(ctx, actor, userID)
	if err != nil {
		return err
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.Redis.DeleteYggTokensByUser(ctx, target.ID); err != nil {
		return err
	}
	updated, err := s.DB.Users.UpdatePasswordAndRevokeRefresh(ctx, target.ID, hash)
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	return s.Redis.InvalidateAuthUser(ctx, target.ID)
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
	if err := s.createBanNotice(ctx, target.ID, input.BannedUntil, reason); err != nil {
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
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return nil, util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify protected subject"}
	}
	return target, nil
}

func (s AccountService) ensureProtectedSubjectMutationAllowed(ctx context.Context, actor permission.Actor, targetID string) error {
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, targetID)
	if err != nil {
		return err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify protected subject"}
	}
	return nil
}

func (s AccountService) userExists(ctx context.Context, userID string) (bool, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func ensureRoleMutationAllowed(actor permission.Actor, roleID string) error {
	if roleID == permission.RoleSystemMaintenance {
		if !actor.Has(manageProtectedPermission) {
			return util.HTTPError{Status: http.StatusForbidden, Detail: "protected role management required"}
		}
	}
	return nil
}

func (s AccountService) createBanNotice(ctx context.Context, targetID string, bannedUntil int64, reason string) error {
	endsAt := database.NowMS() + accountBanNoticeTTL.Milliseconds()
	content := fmt.Sprintf("你的账号已被管理员封禁。\n\n封禁截止时间：%d\n\n原因：\n\n%s", bannedUntil, reason)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "账号已被封禁",
		Summary:         "你的账号已被管理员封禁，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelDanger,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func (s AccountService) createRoleChangeNotice(ctx context.Context, targetID, roleID, action string) error {
	roleName := roleDisplayName(roleID)
	title := "权限已更新：角色已授予"
	content := fmt.Sprintf("你的站点角色已被授予：%s。", roleName)
	if action == "revoke" {
		title = "权限已更新：角色已撤销"
		content = fmt.Sprintf("你的站点角色已被撤销：%s。", roleName)
	}
	endsAt := database.NowMS() + accountPermissionTTL.Milliseconds()
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           title,
		Summary:         "你的站点角色已更新，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelInfo,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func roleDisplayName(roleID string) string {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return fmt.Sprintf("%s（%s）", role.Name, role.ID)
		}
	}
	return roleID
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

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func (s AccountService) attachRoles(ctx context.Context, rawItems any) error {
	items, _ := rawItems.([]map[string]any)
	for _, item := range items {
		userID, _ := item["id"].(string)
		if userID == "" {
			continue
		}
		roles, err := s.DB.Permissions.RoleIDsForUser(ctx, userID)
		if err != nil {
			return err
		}
		protected, err := s.DB.Permissions.UserIsProtected(ctx, userID)
		if err != nil {
			return err
		}
		item["roles"] = roles
		item["protected"] = protected
	}
	return nil
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
