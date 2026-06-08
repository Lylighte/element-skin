package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) yggUploadTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := bearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := r.db.GetToken(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok == nil || tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	profile, err := r.db.GetProfileByID(req.Context(), *tok.ProfileID)
	if err != nil {
		util.Error(w, err)
		return
	}
	if profile == nil || profile.UserID != tok.UserID {
		util.Error(w, util.HTTPError{Status: 403, Detail: "Profile not yours"})
		return
	}
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(req.PathValue("texture_type"))
	storage, err := service.NewTextureStorage(r.cfg.TexturesDir)
	if err != nil {
		util.Error(w, err)
		return
	}
	hash, err := storage.ProcessAndSave(data, textureType)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: err.Error()})
		return
	}
	if err := r.db.AddTextureToLibrary(req.Context(), tok.UserID, hash, textureType, "", false, database.NormalizeProfileModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	if textureType == "skin" {
		if err := r.db.UpdateProfileSkin(req.Context(), profile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
		_ = r.db.UpdateProfileModel(req.Context(), profile.ID, database.NormalizeProfileModel(req.FormValue("model")))
	} else if textureType == "cape" {
		if err := r.db.UpdateProfileCape(req.Context(), profile.ID, &hash); err != nil {
			util.Error(w, err)
			return
		}
	} else {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash})
}

func (r *Router) yggDeleteTexture(w http.ResponseWriter, req *http.Request) {
	token, ok := bearerToken(req)
	if !ok {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Bearer token required"})
		return
	}
	tok, err := r.db.GetToken(req.Context(), token)
	if err != nil {
		util.Error(w, err)
		return
	}
	if tok == nil || tok.ProfileID == nil || *tok.ProfileID != req.PathValue("uuid") {
		util.Error(w, util.HTTPError{Status: 401, Detail: "Invalid token"})
		return
	}
	switch strings.ToLower(req.PathValue("texture_type")) {
	case "skin":
		err = r.db.UpdateProfileSkin(req.Context(), *tok.ProfileID, nil)
	case "cape":
		err = r.db.UpdateProfileCape(req.Context(), *tok.ProfileID, nil)
	default:
		err = util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	if err != nil {
		util.Error(w, err)
		return
	}
	w.WriteHeader(204)
}
