package admin

import (
	"net/http"
	"strings"

	userstore "element-skin/backend/internal/database/user"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/util"
)

var (
	userReadAnyPermission    = permission.MustDefinitionByCode("user.read.any")
	accountReadAnyPermission = permission.MustDefinitionByCode("account.read.any")
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
	if err := h.attachRolesToUserItems(req, res["items"]); err != nil {
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
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if err := h.accounts.GrantUserRole(req.Context(), shared.CurrentActor(req), targetID, roleID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "role_id": roleID})
}

func (h Handler) RevokeUserRole(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	roleID := req.PathValue("role_id")
	if err := h.accounts.RevokeUserRole(req.Context(), shared.CurrentActor(req), targetID, roleID); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "role_id": roleID})
}

func (h Handler) UserPermissions(w http.ResponseWriter, req *http.Request) {
	res, err := h.perms.UserPermissions(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) SetUserPermissionOverride(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Effect string `json:"effect"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	code := req.PathValue("permission_code")
	if err := h.perms.SetUserPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), code, body.Effect); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "permission_code": code, "effect": body.Effect})
}

func (h Handler) ClearUserPermissionOverride(w http.ResponseWriter, req *http.Request) {
	code := req.PathValue("permission_code")
	if err := h.perms.ClearUserPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), code); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "permission_code": code})
}

func (h Handler) DeleteUser(w http.ResponseWriter, req *http.Request) {
	targetID := req.PathValue("user_id")
	if err := h.accounts.DeleteUser(req.Context(), shared.CurrentActor(req), targetID); err != nil {
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
	var body struct {
		BannedUntil int64  `json:"banned_until"`
		Reason      string `json:"reason"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	until, err := h.accounts.BanUser(req.Context(), shared.CurrentActor(req), req.PathValue("user_id"), accountsvc.BanUserInput{
		BannedUntil: body.BannedUntil,
		Reason:      body.Reason,
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "banned_until": until})
}

func (h Handler) UnbanUser(w http.ResponseWriter, req *http.Request) {
	if err := h.accounts.UnbanUser(req.Context(), shared.CurrentActor(req), req.PathValue("user_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ResetUserPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.accounts.ResetPassword(req.Context(), shared.CurrentActor(req), accountsvc.ResetPasswordInput{
		UserID:      body["user_id"],
		NewPassword: body["new_password"],
	}); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) attachRolesToUserItems(req *http.Request, rawItems any) error {
	items, _ := rawItems.([]map[string]any)
	for _, item := range items {
		userID, _ := item["id"].(string)
		if userID == "" {
			continue
		}
		roles, err := h.db.Permissions.RoleIDsForUser(req.Context(), userID)
		if err != nil {
			return err
		}
		item["roles"] = roles
	}
	return nil
}
