package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

func (h Handler) ListMyTextures(w http.ResponseWriter, req *http.Request) {
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.textures.ListMyTextures(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UploadMyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := shared.MultipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	res, err := h.uploads.UploadToLibrary(req.Context(), texturesvc.UploadInput{
		Actor:       shared.CurrentActor(req),
		Data:        data,
		TextureType: req.FormValue("texture_type"),
		Note:        req.FormValue("note"),
		IsPublic:    shared.FormBool(req.FormValue("is_public")),
		Model:       req.FormValue("model"),
	})
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UploadAndApplyTexture(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(16 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	data, err := shared.MultipartFileBytes(req, "file", 16<<20)
	if err != nil {
		util.Error(w, err)
		return
	}
	res, err := h.uploads.UploadAndApply(req.Context(), texturesvc.UploadInput{
		Actor:       shared.CurrentActor(req),
		Data:        data,
		TextureType: req.FormValue("texture_type"),
		IsPublic:    shared.FormBool(req.FormValue("is_public")),
		Model:       req.FormValue("model"),
	}, req.FormValue("uuid"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) TextureDetail(w http.ResponseWriter, req *http.Request) {
	res, err := h.textures.TextureDetail(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) UpdateTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.textures.UpdateTexture(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	if err := h.textures.DeleteTexture(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) AddTexture(w http.ResponseWriter, req *http.Request) {
	if err := h.textures.AddTextureToWardrobe(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.URL.Query().Get("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ApplyTexture(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.textures.ApplyTextureToProfile(req.Context(), shared.CurrentActor(req), body["profile_id"], req.PathValue("hash"), body["texture_type"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
