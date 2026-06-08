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

func (r *Router) adminTextures(w http.ResponseWriter, req *http.Request) {
	lastCreated, lastHash, err := cursorCreatedHash(req.URL.Query().Get("cursor"), "last_skin_hash")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid cursor"})
		return
	}
	res, err := r.db.ListAllTextures(req.Context(), util.ClampLimit(req.URL.Query().Get("limit")), lastCreated, lastHash, strings.TrimSpace(req.URL.Query().Get("q")), req.URL.Query().Get("type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	res["next_cursor"] = util.EncodeCursor(asMap(res["next_key"]))
	delete(res, "next_key")
	util.JSON(w, 200, res)
}

func (r *Router) adminUpdateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	hash := req.PathValue("hash")
	updated := false
	if v, ok := body["note"].(string); ok {
		if err := r.db.AdminUpdateTextureNote(req.Context(), hash, v); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["model"].(string); ok {
		if v != "default" && v != "slim" {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid model"})
			return
		}
		if err := r.db.AdminUpdateTextureModel(req.Context(), hash, v); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if v, ok := body["is_public"]; ok {
		if !validPublicValue(v) {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid is_public"})
			return
		}
		pub := publicBool(v)
		if err := r.db.AdminUpdateTexturePublic(req.Context(), hash, pub); err != nil {
			if err == database.ErrNotFound {
				util.Error(w, util.HTTPError{Status: 404, Detail: "Texture not found"})
				return
			}
			util.Error(w, err)
			return
		}
		updated = true
	}
	if !updated {
		util.Error(w, util.HTTPError{Status: 400, Detail: "至少需要一个更新字段: model, note, is_public"})
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminDeleteTexture(w http.ResponseWriter, req *http.Request) {
	force := req.URL.Query().Get("force") == "true"
	typ := req.URL.Query().Get("type")
	if typ == "" {
		typ = "skin"
	}
	if err := r.db.AdminDeleteTexture(req.Context(), req.PathValue("hash"), typ, req.URL.Query().Get("user_id"), force); err != nil {
		if strings.Contains(err.Error(), "user_id") {
			util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
			return
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"success": true})
}
