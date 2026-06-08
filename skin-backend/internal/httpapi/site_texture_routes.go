package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) listMyTextures(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := r.site.ListMyTextures(req.Context(), currentUserID(req), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) uploadMyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if textureType == "" {
		textureType = "skin"
	}
	if textureType != "skin" && textureType != "cape" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid texture_type"})
		return
	}
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
	if err := r.db.AddTextureToLibrary(req.Context(), currentUserID(req), hash, textureType, req.FormValue("note"), formBool(req.FormValue("is_public")), database.NormalizeProfileModel(req.FormValue("model"))); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"hash": hash, "texture_type": textureType})
}

func (r *Router) uploadAndApplyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	profileID := strings.TrimSpace(req.FormValue("uuid"))
	textureType := strings.ToLower(strings.TrimSpace(req.FormValue("texture_type")))
	if profileID == "" || textureType == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "uuid and texture_type are required"})
		return
	}
	data, err := multipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
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
	model := database.NormalizeProfileModel(req.FormValue("model"))
	if err := r.db.AddTextureToLibrary(req.Context(), currentUserID(req), hash, textureType, "", formBool(req.FormValue("is_public")), model); err != nil {
		util.Error(w, err)
		return
	}
	if err := r.site.ApplyTextureToProfile(req.Context(), currentUserID(req), profileID, hash, textureType); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true, "hash": hash, "type": textureType})
}

func (r *Router) textureDetail(w http.ResponseWriter, req *http.Request) {
	res, err := r.site.TextureDetail(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) updateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.UpdateTexture(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) deleteTexture(w http.ResponseWriter, req *http.Request) {
	if err := r.site.DeleteTexture(req.Context(), currentUserID(req), req.PathValue("hash"), req.PathValue("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) addTexture(w http.ResponseWriter, req *http.Request) {
	if err := r.site.AddTextureToWardrobe(req.Context(), currentUserID(req), req.PathValue("hash")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) applyTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := r.site.ApplyTextureToProfile(req.Context(), currentUserID(req), body["profile_id"], req.PathValue("hash"), body["texture_type"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
