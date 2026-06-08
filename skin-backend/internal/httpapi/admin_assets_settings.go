package httpapi

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) adminUploadCarousel(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(6 << 20); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "file is required"})
		return
	}
	defer file.Close()
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		util.Error(w, util.HTTPError{Status: 400, Detail: "Unsupported file format"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > 5*1024*1024 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	if err := os.MkdirAll(r.cfg.CarouselDir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		util.Error(w, err)
		return
	}
	filename := id + ext
	if err := os.WriteFile(filepath.Join(r.cfg.CarouselDir, filename), data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"filename": filename})
}

func (r *Router) adminDeleteCarousel(w http.ResponseWriter, req *http.Request) {
	filename := filepath.Base(req.PathValue("filename"))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid filename"})
		return
	}
	err := os.Remove(filepath.Join(r.cfg.CarouselDir, filename))
	if err != nil && !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSiteSettings(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), "site")
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSiteSettings(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), "site", body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) adminGetSettingsGroup(w http.ResponseWriter, req *http.Request) {
	res, err := (service.Settings{DB: r.db}).GetGroup(req.Context(), req.PathValue("group"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) adminSaveSettingsGroup(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := (service.Settings{DB: r.db}).SaveGroup(req.Context(), req.PathValue("group"), body); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
