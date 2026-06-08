package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func (r *Router) adminProfiles(w http.ResponseWriter, req *http.Request) {
	cursor, err := util.DecodeCursor(req.URL.Query().Get("cursor"))
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	last := ""
	if cursor != nil {
		last, _ = cursor["last_id"].(string)
	}
	res, err := r.db.ListAllProfiles(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), last, strings.TrimSpace(req.URL.Query().Get("q")))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminUpdateProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	p, err := r.db.GetProfileByID(req.Context(), profileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if body["name"] != "" {
		if !util.ValidProfileName(body["name"]) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid profile name"})
			return
		}
		ok, err := r.db.UpdateProfileName(req.Context(), profileID, body["name"])
		if err != nil {
			if database.IsProfileNameConflict(err) {
				util.Error(w, util.HTTPError{Status: 409, Detail: "profile name already exists"})
				return
			}
			util.Error(w, err)
			return
		}
		if !ok {
			util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminDeleteProfile(w http.ResponseWriter, req *http.Request) {
	ok, err := r.db.DeleteProfileCascade(req.Context(), req.PathValue("profile_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	if !ok {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminUpdateProfileSkin(w http.ResponseWriter, req *http.Request) {
	r.adminSetProfileTexture(w, req, "skin")
}

func (r *Router) adminUpdateProfileCape(w http.ResponseWriter, req *http.Request) {
	r.adminSetProfileTexture(w, req, "cape")
}

func (r *Router) adminSetProfileTexture(w http.ResponseWriter, req *http.Request, typ string) {
	var body map[string]*string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := req.PathValue("profile_id")
	if p, err := r.db.GetProfileByID(req.Context(), profileID); err != nil {
		util.Error(w, err)
		return
	} else if p == nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "profile not found"})
		return
	}
	if typ == "skin" {
		if err := r.db.UpdateProfileSkin(req.Context(), profileID, body["hash"]); err != nil {
			util.Error(w, err)
			return
		}
	} else {
		if err := r.db.UpdateProfileCape(req.Context(), profileID, body["hash"]); err != nil {
			util.Error(w, err)
			return
		}
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
