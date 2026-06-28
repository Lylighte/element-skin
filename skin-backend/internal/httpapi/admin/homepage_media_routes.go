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
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

const (
	maxHomepageImageBytes    = 5 * 1024 * 1024
	maxHomepagePanoramaBytes = 50 * 1024 * 1024
)

var (
	homepageMediaReadPermission   = permission.MustDefinitionByCode("homepage_media.read.any")
	homepageMediaCreatePermission = permission.MustDefinitionByCode("homepage_media.create.any")
	homepageMediaUpdatePermission = permission.MustDefinitionByCode("homepage_media.update.any")
	homepageMediaDeletePermission = permission.MustDefinitionByCode("homepage_media.delete.any")
)

func (h Handler) ListHomepageMedia(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, homepageMediaReadPermission); err != nil {
		util.Error(w, err)
		return
	}
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
	if err := shared.RequirePermission(req, homepageMediaCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
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
	values, err := homepageMediaValuesFromForm(req, "image")
	if err != nil {
		util.Error(w, err)
		return
	}
	item, path, err := h.newHomepageMedia(req, "image", header.Filename, ext, values)
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
	if err := shared.RequirePermission(req, homepageMediaCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
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
	values, err := homepageMediaValuesFromForm(req, "panorama")
	if err != nil {
		util.Error(w, err)
		return
	}
	item, dir, err := h.newHomepageMedia(req, "panorama", header.Filename, "", values)
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
	if err := shared.RequirePermission(req, homepageMediaUpdatePermission); err != nil {
		util.Error(w, err)
		return
	}
	var body struct {
		Title               *string  `json:"title"`
		OverlayOpacityLight *float64 `json:"overlay_opacity_light"`
		OverlayOpacityDark  *float64 `json:"overlay_opacity_dark"`
		StartYaw            *float64 `json:"start_yaw"`
		StartPitch          *float64 `json:"start_pitch"`
		YawSpeedDPS         *float64 `json:"yaw_speed_dps"`
		PitchSpeedDPS       *float64 `json:"pitch_speed_dps"`
		Enabled             *bool    `json:"enabled"`
		DurationMS          *int     `json:"duration_ms"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json body"})
		return
	}
	if body.DurationMS != nil && (*body.DurationMS < 1000 || *body.DurationMS > 60000) {
		util.Error(w, util.HTTPError{Status: 400, Detail: "duration_ms out of range"})
		return
	}
	if err := validateOpacity("overlay_opacity_light", body.OverlayOpacityLight); err != nil {
		util.Error(w, err)
		return
	}
	if err := validateOpacity("overlay_opacity_dark", body.OverlayOpacityDark); err != nil {
		util.Error(w, err)
		return
	}
	id := req.PathValue("id")
	item, err := h.db.HomepageMedia.Get(req.Context(), id)
	if err != nil {
		util.Error(w, util.HTTPError{Status: 404, Detail: "homepage media not found"})
		return
	}
	patch := homepage.Patch{
		Title:               body.Title,
		OverlayOpacityLight: body.OverlayOpacityLight,
		OverlayOpacityDark:  body.OverlayOpacityDark,
		Enabled:             body.Enabled,
		DurationMS:          body.DurationMS,
		UpdatedAt:           database.NowMS(),
	}
	if item.Type == "panorama" {
		if err := validatePanoramaValues(body.StartYaw, body.StartPitch, body.YawSpeedDPS, body.PitchSpeedDPS); err != nil {
			util.Error(w, err)
			return
		}
		patch.StartYaw = body.StartYaw
		patch.StartPitch = body.StartPitch
		patch.YawSpeedDPS = body.YawSpeedDPS
		patch.PitchSpeedDPS = body.PitchSpeedDPS
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
	if err := shared.RequirePermission(req, homepageMediaUpdatePermission); err != nil {
		util.Error(w, err)
		return
	}
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
	if err := shared.RequirePermission(req, homepageMediaDeletePermission); err != nil {
		util.Error(w, err)
		return
	}
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

type homepageMediaValues struct {
	OverlayOpacityLight float64
	OverlayOpacityDark  float64
	StartYaw            float64
	StartPitch          float64
	YawSpeedDPS         float64
	PitchSpeedDPS       float64
}

func (h Handler) newHomepageMedia(req *http.Request, typ, title, ext string, values homepageMediaValues) (model.HomepageMedia, string, error) {
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
		ID:                  id,
		Type:                typ,
		Title:               title,
		StoragePath:         filepath.ToSlash(storagePath),
		OverlayOpacityLight: values.OverlayOpacityLight,
		OverlayOpacityDark:  values.OverlayOpacityDark,
		StartYaw:            values.StartYaw,
		StartPitch:          values.StartPitch,
		YawSpeedDPS:         values.YawSpeedDPS,
		PitchSpeedDPS:       values.PitchSpeedDPS,
		SortOrder:           order,
		Enabled:             true,
		DurationMS:          duration,
		CreatedAt:           now,
		UpdatedAt:           now,
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

func homepageMediaValuesFromForm(req *http.Request, typ string) (homepageMediaValues, error) {
	values := homepageMediaValues{
		StartYaw:      0,
		StartPitch:    0,
		YawSpeedDPS:   4,
		PitchSpeedDPS: 0,
	}
	var err error
	values.OverlayOpacityLight, err = floatForm(req, "overlay_opacity_light", 0.45)
	if err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "overlay_opacity_light must be a number"}
	}
	values.OverlayOpacityDark, err = floatForm(req, "overlay_opacity_dark", 0.45)
	if err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "overlay_opacity_dark must be a number"}
	}
	if err := validateOpacityValue("overlay_opacity_light", values.OverlayOpacityLight); err != nil {
		return homepageMediaValues{}, err
	}
	if err := validateOpacityValue("overlay_opacity_dark", values.OverlayOpacityDark); err != nil {
		return homepageMediaValues{}, err
	}
	if typ != "panorama" {
		return values, nil
	}
	if values.StartYaw, err = floatForm(req, "start_yaw", 0); err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "start_yaw must be a number"}
	}
	if values.StartPitch, err = floatForm(req, "start_pitch", 0); err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "start_pitch must be a number"}
	}
	if values.YawSpeedDPS, err = floatForm(req, "yaw_speed_dps", 4); err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "yaw_speed_dps must be a number"}
	}
	if values.PitchSpeedDPS, err = floatForm(req, "pitch_speed_dps", 0); err != nil {
		return homepageMediaValues{}, util.HTTPError{Status: 400, Detail: "pitch_speed_dps must be a number"}
	}
	if err := validatePanoramaValues(&values.StartYaw, &values.StartPitch, &values.YawSpeedDPS, &values.PitchSpeedDPS); err != nil {
		return homepageMediaValues{}, err
	}
	return values, nil
}

func validateOpacity(name string, v *float64) error {
	if v == nil {
		return nil
	}
	return validateOpacityValue(name, *v)
}

func validateOpacityValue(name string, v float64) error {
	if v < 0 || v > 0.9 {
		return util.HTTPError{Status: 400, Detail: name + " out of range"}
	}
	return nil
}

func validatePanoramaValues(startYaw, startPitch, yawSpeedDPS, pitchSpeedDPS *float64) error {
	if startYaw != nil && (*startYaw < -360 || *startYaw > 360) {
		return util.HTTPError{Status: 400, Detail: "start_yaw out of range"}
	}
	if startPitch != nil && (*startPitch < -89 || *startPitch > 89) {
		return util.HTTPError{Status: 400, Detail: "start_pitch out of range"}
	}
	if yawSpeedDPS != nil && (*yawSpeedDPS < -90 || *yawSpeedDPS > 90) {
		return util.HTTPError{Status: 400, Detail: "yaw_speed_dps out of range"}
	}
	if pitchSpeedDPS != nil && (*pitchSpeedDPS < -90 || *pitchSpeedDPS > 90) {
		return util.HTTPError{Status: 400, Detail: "pitch_speed_dps out of range"}
	}
	return nil
}

func floatForm(req *http.Request, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(req.FormValue(key))
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(raw, 64)
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
