package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/util"
)

func (h Handler) Me(w http.ResponseWriter, req *http.Request) {
	res, err := h.accounts.Me(req.Context(), shared.CurrentActor(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateMe(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.accounts.UpdateSelf(req.Context(), shared.CurrentActor(req), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteMe(w http.ResponseWriter, req *http.Request) {
	if err := h.accounts.DeleteSelf(req.Context(), shared.CurrentActor(req)); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ChangePassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.accounts.ChangePasswordSelf(req.Context(), shared.CurrentActor(req), body["old_password"], body["new_password"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "message": "密码修改成功"})
}
