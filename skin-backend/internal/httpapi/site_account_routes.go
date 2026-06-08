package httpapi

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func (r *Router) me(w http.ResponseWriter, req *http.Request) {
	res, err := r.site.Me(req.Context(), currentUserID(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateMe(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.UpdateMe(req.Context(), currentUserID(req), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) deleteMe(w http.ResponseWriter, req *http.Request) {
	userID := currentUserID(req)
	user, err := r.db.GetUserByID(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if user == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	if user.IsAdmin {
		util.Error(w, util.HTTPError{Status: 403, Detail: "管理员不能删除自己的账号"})
		return
	}
	ok, err := r.db.DeleteUser(req.Context(), userID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "user not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) changePassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.ChangePassword(req.Context(), currentUserID(req), body["old_password"], body["new_password"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "message": "密码修改成功"})
}
