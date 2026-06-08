package httpapi

import (
	"net/http"

	"element-skin/backend/internal/util"
)

func (r *Router) createProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.CreateProfile(req.Context(), currentUserID(req), body["name"], body["model"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.UpdateProfile(req.Context(), currentUserID(req), req.PathValue("pid"), body["name"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) deleteProfile(w http.ResponseWriter, req *http.Request) {
	if err := r.site.DeleteProfile(req.Context(), currentUserID(req), req.PathValue("pid")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) clearProfileSkin(w http.ResponseWriter, req *http.Request) {
	if err := r.site.ClearProfileTexture(req.Context(), currentUserID(req), req.PathValue("pid"), "skin"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) clearProfileCape(w http.ResponseWriter, req *http.Request) {
	if err := r.site.ClearProfileTexture(req.Context(), currentUserID(req), req.PathValue("pid"), "cape"); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) listMyProfiles(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.ListMyProfiles(req.Context(), currentUserID(req), req.URL.Query().Get("cursor"), limit)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}
