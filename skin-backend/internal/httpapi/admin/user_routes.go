package admin

import (
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

var manageProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")

var (
	userReadAnyPermission      = permission.MustDefinitionByCode("user.read.any")
	userUpdateAnyPermission    = permission.MustDefinitionByCode("user.update.any")
	accountReadAnyPermission   = permission.MustDefinitionByCode("account.read.any")
	accountUpdateAnyPermission = permission.MustDefinitionByCode("account.update.any")
	accountDeleteAnyPermission = permission.MustDefinitionByCode("account.delete.any")
	accountBanAnyPermission    = permission.MustDefinitionByCode("account.ban.any")
	accountUnbanAnyPermission  = permission.MustDefinitionByCode("account.unban.any")
	permissionGrantAny        = permission.MustDefinitionByCode("permission.grant.any")
	permissionRevokeAny       = permission.MustDefinitionByCode("permission.revoke.any")
)

func (h Handler) Users(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, userReadAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	rawCursor := req.URL.Query().Get("cursor")
	cursor, err := util.DecodeCursor(rawCursor)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	if rawCursor != "" && last == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Users.List(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) User(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, accountReadAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	user, err := h.db.Users.GetByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	out := userstore.PublicUser(*user)
	roles, err := h.db.Permissions.RoleIDsForUser(req.Context(), user.ID)
	if err != nil {
		util.Error(w, err)
		return
	}
	out["roles"] = roles
	util.JSON(w, 200, out)
}

func (h Handler) GrantUserRole(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, permissionGrantAny); err != nil {
		util.Error(w, err)
		return
	}
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if roleID == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "role_id required"})
		return
	}
	if roleID == permission.RoleSuperAdmin && targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot grant protected role to yourself"})
		return
	}
	if err := h.ensureRoleGrantAllowed(req, roleID); err != nil {
		util.Error(w, err)
		return
	}
	if ok, err := h.userExists(req, targetID); err != nil {
		util.Error(w, err)
		return
	} else if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.db.Permissions.GrantRole(req.Context(), targetID, roleID, shared.CurrentActor(req).SubjectID); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "role_id": roleID})
}

func (h Handler) RevokeUserRole(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, permissionRevokeAny); err != nil {
		util.Error(w, err)
		return
	}
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if roleID == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "role_id required"})
		return
	}
	if roleID == permission.RoleSuperAdmin && targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot revoke protected role from yourself"})
		return
	}
	if err := h.ensureRoleGrantAllowed(req, roleID); err != nil {
		util.Error(w, err)
		return
	}
	if ok, err := h.userExists(req, targetID); err != nil {
		util.Error(w, err)
		return
	} else if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	ok, err := h.db.Permissions.RevokeRole(req.Context(), targetID, roleID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "role assignment not found"})
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "role_id": roleID})
}

func (h Handler) DeleteUser(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, accountDeleteAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	targetID := req.PathValue("user_id")
	if targetID == shared.CurrentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot delete yourself"})
		return
	}
	if err := h.ensureTargetNotSuperAdmin(req, targetID); err != nil {
		util.Error(w, err)
		return
	}
	ok, err := h.site.DeleteUser(req.Context(), targetID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), targetID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) UserProfiles(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, profileReadAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	rawCursor := req.URL.Query().Get("cursor")
	cursor, err := util.DecodeCursor(rawCursor)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	lastID := ""
	if cursor != nil {
		lastID, _ = cursor["last_id"].(string)
	}
	if rawCursor != "" && lastID == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := h.db.Profiles.ListByUser(req.Context(), req.PathValue("user_id"), util.ClampLimit(req.URL.Query().Get("limit")), lastID)
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(shared.AsMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (h Handler) BanUser(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, accountBanAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]int64
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	until, ok := body["banned_until"]
	if !ok || until < time.Now().Add(-24*time.Hour).UnixMilli() {
		util.Error(w, util.HTTPError{Status: 400, Detail: "banned_until is required"})
		return
	}
	userID := req.PathValue("user_id")
	if err := h.ensureTargetNotSuperAdmin(req, userID); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.db.Users.Ban(req.Context(), userID, until); err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "banned_until": until})
}

func (h Handler) UnbanUser(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, accountUnbanAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	user, err := h.db.Users.GetByID(req.Context(), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	hasProtectedRole, err := h.db.Permissions.UserHasProtectedRole(req.Context(), user.ID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if hasProtectedRole && !shared.CurrentActor(req).Has(manageProtectedPermission) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "cannot modify super admin"})
		return
	}
	if err := h.db.Users.Unban(req.Context(), user.ID); err != nil {
		if database.IsNoRows(err) {
			util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
			return
		}
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), user.ID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ResetUserPassword(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, accountUpdateAnyPermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	userID := body["user_id"]
	newPassword := body["new_password"]
	if userID == "" || newPassword == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "user_id and new_password required"})
		return
	}
	if err := h.ensureTargetNotSuperAdmin(req, userID); err != nil {
		util.Error(w, err)
		return
	}
	hash, err := util.HashPassword(newPassword)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.DeleteYggTokensByUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	ok, err := h.db.Users.UpdatePasswordAndRevokeRefresh(req.Context(), userID, hash)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if err := h.redis.InvalidateAuthUser(req.Context(), userID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ensureTargetNotSuperAdmin(req *http.Request, targetID string) error {
	target, err := h.db.Users.GetByID(req.Context(), targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return util.HTTPError{Status: 404, Detail: "user not found"}
	}
	hasProtectedRole, err := h.db.Permissions.UserHasProtectedRole(req.Context(), targetID)
	if err != nil {
		return err
	}
	if hasProtectedRole && !shared.CurrentActor(req).Has(manageProtectedPermission) {
		return util.HTTPError{Status: 403, Detail: "cannot modify super admin"}
	}
	return nil
}

func (h Handler) ensureRoleGrantAllowed(req *http.Request, roleID string) error {
	if roleID == permission.RoleSuperAdmin || roleID == permission.RoleSystemMaintenance {
		if !shared.CurrentActor(req).Has(manageProtectedPermission) {
			return util.HTTPError{Status: 403, Detail: "protected role management required"}
		}
	}
	return nil
}

func (h Handler) userExists(req *http.Request, userID string) (bool, error) {
	user, err := h.db.Users.GetByID(req.Context(), userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}
