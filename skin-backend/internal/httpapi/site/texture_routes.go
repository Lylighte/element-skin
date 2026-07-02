package site

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

var (
	textureReadOwnedPermission             = permission.MustDefinitionByCode("texture.read.owned")
	textureUpdateMetadataOwnedPermission   = permission.MustDefinitionByCode("texture.update_metadata.owned")
	textureUpdateVisibilityOwnedPermission = permission.MustDefinitionByCode("texture.update_visibility.owned")
	textureDeleteOwnedPermission           = permission.MustDefinitionByCode("texture.delete.owned")
	textureApplyOwnedPermission            = permission.MustDefinitionByCode("texture.apply.owned")
	wardrobeEntryAddOwnedPermission        = permission.MustDefinitionByCode("wardrobe_entry.add.owned")
)

func (h Handler) ListMyTextures(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, textureReadOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	limit := util.ClampLimit(req.URL.Query().Get("limit"))
	res, err := h.site.ListMyTextures(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("cursor"), limit, req.URL.Query().Get("texture_type"))
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
	if err := shared.RequirePermission(req, textureReadOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	res, err := h.site.TextureDetail(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type"))
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
	if _, ok := body["note"]; ok {
		if err := shared.RequirePermission(req, textureUpdateMetadataOwnedPermission); err != nil {
			util.Error(w, err)
			return
		}
	}
	if _, ok := body["model"]; ok {
		if err := shared.RequirePermission(req, textureUpdateMetadataOwnedPermission); err != nil {
			util.Error(w, err)
			return
		}
	}
	if _, ok := body["is_public"]; ok {
		if err := shared.RequirePermission(req, textureUpdateVisibilityOwnedPermission); err != nil {
			util.Error(w, err)
			return
		}
	}
	res, err := h.site.UpdateTexture(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type"), body)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (h Handler) DeleteTexture(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, textureDeleteOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.DeleteTexture(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.PathValue("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) AddTexture(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, wardrobeEntryAddOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.site.AddTextureToWardrobe(req.Context(), shared.CurrentActor(req), req.PathValue("hash"), req.URL.Query().Get("texture_type")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) ApplyTexture(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, textureApplyOwnedPermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.site.ApplyTextureToProfile(req.Context(), shared.CurrentActor(req), body["profile_id"], req.PathValue("hash"), body["texture_type"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
