package admin

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"element-skin/backend/internal/util"
)

func (h Handler) UploadCarousel(w http.ResponseWriter, req *http.Request) {
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
	if err := os.MkdirAll(h.cfg.CarouselDir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		util.Error(w, err)
		return
	}
	filename := id + ext
	path := filepath.Join(h.cfg.CarouselDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicCarousel(req.Context()); err != nil {
		_ = os.Remove(path)
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"filename": filename})
}

func (h Handler) DeleteCarousel(w http.ResponseWriter, req *http.Request) {
	filename := filepath.Base(req.PathValue("filename"))
	if filename == "." || filename == string(filepath.Separator) || filename == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid filename"})
		return
	}
	path := filepath.Join(h.cfg.CarouselDir, filename)
	var original []byte
	var mode os.FileMode
	if info, err := os.Stat(path); err == nil {
		original, err = os.ReadFile(path)
		if err != nil {
			util.Error(w, err)
			return
		}
		mode = info.Mode().Perm()
	} else if !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicCarousel(req.Context()); err != nil {
		if original != nil {
			if restoreErr := os.WriteFile(path, original, mode); restoreErr != nil {
				util.Error(w, restoreErr)
				return
			}
		}
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}
