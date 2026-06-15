package admin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

const (
	maxHomepageImageBytes    = 5 * 1024 * 1024
	maxHomepagePanoramaBytes = 50 * 1024 * 1024
)

func (h Handler) ListHomepageMedia(w http.ResponseWriter, req *http.Request) {
	items, err := h.db.HomepageMedia.List(req.Context(), false)
	if err != nil {
		util.Error(w, err)
		return
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	util.JSON(w, 200, items)
}

func (h Handler) UploadHomepageImage(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(maxHomepageImageBytes + 1); err != nil {
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
	data, err := io.ReadAll(io.LimitReader(file, maxHomepageImageBytes+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > maxHomepageImageBytes {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	if ext != ".webp" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(data)); err != nil {
			util.Error(w, util.HTTPError{Status: 400, Detail: "invalid image"})
			return
		}
	}
	cfg, err := baseHomepageMediaConfig(req)
	if err != nil {
		util.Error(w, err)
		return
	}
	item, path, err := h.newHomepageMedia(req, "image", header.Filename, ext, cfg)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		util.Error(w, err)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.db.HomepageMedia.Create(req.Context(), item); err != nil {
		_ = os.Remove(path)
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicHomepageMedia(req.Context()); err != nil {
		_, _ = h.db.HomepageMedia.Delete(req.Context(), item.ID)
		_ = os.Remove(path)
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, item)
}

func (h Handler) UploadHomepagePanorama(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseMultipartForm(maxHomepagePanoramaBytes + 1); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid multipart form"})
		return
	}
	file, header, err := req.FormFile("file")
	if err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "file is required"})
		return
	}
	defer file.Close()
	if strings.ToLower(filepath.Ext(header.Filename)) != ".zip" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Unsupported file format"})
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxHomepagePanoramaBytes+1))
	if err != nil {
		util.Error(w, err)
		return
	}
	if len(data) > maxHomepagePanoramaBytes {
		util.Error(w, util.HTTPError{Status: 400, Detail: "File too large"})
		return
	}
	faces, err := readPanoramaZip(data)
	if err != nil {
		util.Error(w, err)
		return
	}
	cfg, err := panoramaConfigFromForm(req)
	if err != nil {
		util.Error(w, err)
		return
	}
	item, dir, err := h.newHomepageMedia(req, "panorama", header.Filename, "", cfg)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		util.Error(w, err)
		return
	}
	for name, content := range faces {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			util.Error(w, err)
			return
		}
	}
	if err := h.db.HomepageMedia.Create(req.Context(), item); err != nil {
		_ = os.RemoveAll(dir)
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicHomepageMedia(req.Context()); err != nil {
		_, _ = h.db.HomepageMedia.Delete(req.Context(), item.ID)
		_ = os.RemoveAll(dir)
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, item)
}

func (h Handler) PatchHomepageMedia(w http.ResponseWriter, req *http.Request) {
	var body struct {
		Title      *string        `json:"title"`
		Config     map[string]any `json:"config"`
		Enabled    *bool          `json:"enabled"`
		DurationMS *int           `json:"duration_ms"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json body"})
		return
	}
	if body.DurationMS != nil && (*body.DurationMS < 1000 || *body.DurationMS > 60000) {
		util.Error(w, util.HTTPError{Status: 400, Detail: "duration_ms out of range"})
		return
	}
	id := req.PathValue("id")
	item, err := h.db.HomepageMedia.Get(req.Context(), id)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "homepage media not found"})
		return
	}
	patch := homepage.Patch{Title: body.Title, Enabled: body.Enabled, DurationMS: body.DurationMS, UpdatedAt: database.NowMS()}
	if body.Config != nil {
		cfg, err := normalizeHomepageMediaConfig(item.Type, body.Config)
		if err != nil {
			util.Error(w, err)
			return
		}
		patch.Config = cfg
		patch.HasConfig = true
	}
	item, err = h.db.HomepageMedia.Patch(req.Context(), id, patch)
	if err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicHomepageMedia(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, item)
}

func (h Handler) ReorderHomepageMedia(w http.ResponseWriter, req *http.Request) {
	var body struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json body"})
		return
	}
	if len(body.IDs) == 0 {
		util.Error(w, util.HTTPError{Status: 400, Detail: "ids is required"})
		return
	}
	seen := map[string]bool{}
	for _, id := range body.IDs {
		if id == "" || seen[id] {
			util.Error(w, util.HTTPError{Status: 400, Detail: "ids must be unique non-empty strings"})
			return
		}
		seen[id] = true
	}
	if err := h.db.HomepageMedia.Reorder(req.Context(), body.IDs, database.NowMS()); err != nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "homepage media not found"})
		return
	}
	if err := h.redis.InvalidatePublicHomepageMedia(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) DeleteHomepageMedia(w http.ResponseWriter, req *http.Request) {
	id := req.PathValue("id")
	item, err := h.db.HomepageMedia.Delete(req.Context(), id)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "homepage media not found"})
		return
	}
	path := filepath.Join(h.cfg.CarouselDir, item.StoragePath)
	if item.Type == "panorama" || strings.Contains(item.StoragePath, "/") {
		path = filepath.Join(h.cfg.CarouselDir, item.ID)
	}
	if err := os.RemoveAll(path); err != nil {
		util.Error(w, err)
		return
	}
	if err := h.redis.InvalidatePublicHomepageMedia(req.Context()); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (h Handler) newHomepageMedia(req *http.Request, typ, title, ext string, cfg map[string]any) (model.HomepageMedia, string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	order, err := h.db.HomepageMedia.NextSortOrder(req.Context())
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	duration := intForm(req, "duration_ms", 6000)
	if typ == "panorama" && req.FormValue("duration_ms") == "" {
		duration = 9000
	}
	if duration < 1000 || duration > 60000 {
		return model.HomepageMedia{}, "", util.HTTPError{Status: 400, Detail: "duration_ms out of range"}
	}
	now := database.NowMS()
	storagePath := id + ext
	path := filepath.Join(h.cfg.CarouselDir, storagePath)
	if typ == "panorama" {
		storagePath = id
		path = filepath.Join(h.cfg.CarouselDir, id)
	}
	if title == "" {
		title = typ
	}
	return model.HomepageMedia{
		ID:          id,
		Type:        typ,
		Title:       title,
		StoragePath: filepath.ToSlash(storagePath),
		Config:      cfg,
		SortOrder:   order,
		Enabled:     true,
		DurationMS:  duration,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, path, nil
}

func readPanoramaZip(data []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "invalid panorama zip"}
	}
	required := map[string]bool{}
	for i := 0; i < 6; i++ {
		required["panorama_"+strconv.Itoa(i)+".png"] = false
	}
	out := map[string][]byte{}
	for _, f := range reader.File {
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "/") || strings.Contains(name, `\`) {
			return nil, util.HTTPError{Status: 400, Detail: "panorama files must be at zip root"}
		}
		if _, ok := required[name]; !ok {
			return nil, util.HTTPError{Status: 400, Detail: "panorama zip must contain only panorama_0.png through panorama_5.png"}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxHomepageImageBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if len(content) > maxHomepageImageBytes {
			return nil, util.HTTPError{Status: 400, Detail: "panorama face too large"}
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
			return nil, util.HTTPError{Status: 400, Detail: "invalid panorama face image"}
		}
		required[name] = true
		out[name] = content
	}
	for name, ok := range required {
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "missing " + name}
		}
	}
	return out, nil
}

func panoramaConfigFromForm(req *http.Request) (map[string]any, error) {
	return normalizeHomepageMediaConfig("panorama", map[string]any{
		"overlay_opacity": req.FormValue("overlay_opacity"),
		"start_yaw":       req.FormValue("start_yaw"),
		"start_pitch":     req.FormValue("start_pitch"),
		"yaw_speed_dps":   req.FormValue("yaw_speed_dps"),
		"pitch_speed_dps": req.FormValue("pitch_speed_dps"),
	})
}

func baseHomepageMediaConfig(req *http.Request) (map[string]any, error) {
	return normalizeHomepageMediaConfig("image", map[string]any{
		"overlay_opacity": req.FormValue("overlay_opacity"),
	})
}

func normalizeHomepageMediaConfig(typ string, raw map[string]any) (map[string]any, error) {
	out := map[string]any{}
	overlay, err := numberField(raw["overlay_opacity"], 0.45)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "overlay_opacity must be a number"}
	}
	if overlay < 0 || overlay > 0.9 {
		return nil, util.HTTPError{Status: 400, Detail: "overlay_opacity out of range"}
	}
	out["overlay_opacity"] = overlay
	if typ != "panorama" {
		return out, nil
	}
	defaults := map[string]float64{
		"start_yaw":       0,
		"start_pitch":     0,
		"yaw_speed_dps":   4,
		"pitch_speed_dps": 0,
	}
	for key, fallback := range defaults {
		v, err := numberField(raw[key], fallback)
		if err != nil {
			return nil, util.HTTPError{Status: 400, Detail: key + " must be a number"}
		}
		if key == "start_pitch" && (v < -89 || v > 89) {
			return nil, util.HTTPError{Status: 400, Detail: key + " out of range"}
		}
		if key == "start_yaw" && (v < -360 || v > 360) {
			return nil, util.HTTPError{Status: 400, Detail: key + " out of range"}
		}
		if strings.Contains(key, "speed") && (v < -90 || v > 90) {
			return nil, util.HTTPError{Status: 400, Detail: key + " out of range"}
		}
		out[key] = v
	}
	return out, nil
}

func numberField(v any, fallback float64) (float64, error) {
	switch x := v.(type) {
	case nil:
		return fallback, nil
	case float64:
		return x, nil
	case int:
		return float64(x), nil
	case string:
		if strings.TrimSpace(x) == "" {
			return fallback, nil
		}
		return strconv.ParseFloat(x, 64)
	default:
		return 0, strconv.ErrSyntax
	}
}

func intForm(req *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(req.FormValue(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
