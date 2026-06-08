package httpapi

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func (r *Router) adminInvites(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastCode, err := cursorCreatedHash(req.URL.Query().Get("cursor"), "last_code")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := r.db.ListInvites(req.Context(), util.ClampLimit(req.URL.Query().Get("limit"), 15), lastCreated, lastCode)
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminCreateInvite(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	code, _ := body["code"].(string)
	if code == "" {
		id, err := util.GenerateUUIDNoDash()
		if err != nil {
			util.Error(w, err)
			return
		}
		code = id + id[:8]
	}
	if len(code) < 4 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invite code too short"})
		return
	}
	total := 1
	if v, ok := body["total_uses"].(float64); ok {
		total = int(v)
	}
	note, _ := body["note"].(string)
	if err := r.db.CreateInvite(req.Context(), code, total, note); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"code": code, "total_uses": total, "note": note})
}

func (r *Router) adminDeleteInvite(w http.ResponseWriter, req *http.Request) {
	if err := r.db.DeleteInvite(req.Context(), req.PathValue("code")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
