package homepage

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"element-skin/backend/internal/database"
	dbhomepage "element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

const (
	MaxImageBytes    = 5 * 1024 * 1024
	MaxPanoramaBytes = 50 * 1024 * 1024
)

var (
	homepageMediaReadPermission   = permission.MustDefinitionByCode("homepage_media.read.any")
	homepageMediaCreatePermission = permission.MustDefinitionByCode("homepage_media.create.any")
	homepageMediaUpdatePermission = permission.MustDefinitionByCode("homepage_media.update.any")
	homepageMediaDeletePermission = permission.MustDefinitionByCode("homepage_media.delete.any")
)

type Service struct {
	DB          *database.DB
	Redis       redisstore.Store
	CarouselDir string
}

type MultipartSource interface {
	MultipartReader() (*multipart.Reader, error)
}

type MediaValues struct {
	OverlayOpacityLight float64
	OverlayOpacityDark  float64
	StartYaw            float64
	StartPitch          float64
	YawSpeedDPS         float64
	PitchSpeedDPS       float64
	DurationMS          int
}

type PatchInput struct {
	Title               *string
	OverlayOpacityLight *float64
	OverlayOpacityDark  *float64
	StartYaw            *float64
	StartPitch          *float64
	YawSpeedDPS         *float64
	PitchSpeedDPS       *float64
	Enabled             *bool
	DurationMS          *int
}

func (s Service) List(ctx context.Context, actor permission.Actor) ([]model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaReadPermission); err != nil {
		return nil, err
	}
	items, err := s.DB.HomepageMedia.List(ctx, false)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	return items, nil
}

func (s Service) UploadImage(ctx context.Context, actor permission.Actor, source MultipartSource) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaCreatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	upload, err := readMultipartUpload(source, MaxImageBytes)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	ext := strings.ToLower(filepath.Ext(upload.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
	default:
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "Unsupported file format"}
	}
	if ext != ".webp" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(upload.Data)); err != nil {
			return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid image"}
		}
	}
	values, err := ParseMediaValues(upload.Fields, "image")
	if err != nil {
		return model.HomepageMedia{}, err
	}
	return s.createImage(ctx, upload.Filename, ext, upload.Data, values)
}

func (s Service) UploadPanorama(ctx context.Context, actor permission.Actor, source MultipartSource) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaCreatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	upload, err := readMultipartUpload(source, MaxPanoramaBytes)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if strings.ToLower(filepath.Ext(upload.Filename)) != ".zip" {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "Unsupported file format"}
	}
	faces, err := ReadPanoramaZip(upload.Data)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	values, err := ParseMediaValues(upload.Fields, "panorama")
	if err != nil {
		return model.HomepageMedia{}, err
	}
	return s.createPanorama(ctx, upload.Filename, faces, values)
}

func (s Service) createImage(ctx context.Context, filename, ext string, data []byte, values MediaValues) (model.HomepageMedia, error) {
	item, path, err := s.newMedia(ctx, "image", filename, ext, values)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := s.DB.HomepageMedia.Create(ctx, item); err != nil {
		_ = os.Remove(path)
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		_, _ = s.DB.HomepageMedia.Delete(ctx, item.ID)
		_ = os.Remove(path)
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) createPanorama(ctx context.Context, filename string, faces map[string][]byte, values MediaValues) (model.HomepageMedia, error) {
	item, dir, err := s.newMedia(ctx, "panorama", filename, "", values)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return model.HomepageMedia{}, err
	}
	for name, content := range faces {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return model.HomepageMedia{}, err
		}
	}
	if err := s.DB.HomepageMedia.Create(ctx, item); err != nil {
		_ = os.RemoveAll(dir)
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		_, _ = s.DB.HomepageMedia.Delete(ctx, item.ID)
		_ = os.RemoveAll(dir)
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) Patch(ctx context.Context, actor permission.Actor, id string, input PatchInput) (model.HomepageMedia, error) {
	if err := requirePermission(actor, homepageMediaUpdatePermission); err != nil {
		return model.HomepageMedia{}, err
	}
	if input.DurationMS != nil && (*input.DurationMS < 1000 || *input.DurationMS > 60000) {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "duration_ms out of range"}
	}
	if err := ValidateOpacity("overlay_opacity_light", input.OverlayOpacityLight); err != nil {
		return model.HomepageMedia{}, err
	}
	if err := ValidateOpacity("overlay_opacity_dark", input.OverlayOpacityDark); err != nil {
		return model.HomepageMedia{}, err
	}
	item, err := s.DB.HomepageMedia.Get(ctx, id)
	if err != nil {
		return model.HomepageMedia{}, util.HTTPError{Status: http.StatusNotFound, Detail: "homepage media not found"}
	}
	patch := dbhomepage.Patch{
		Title:               input.Title,
		OverlayOpacityLight: input.OverlayOpacityLight,
		OverlayOpacityDark:  input.OverlayOpacityDark,
		Enabled:             input.Enabled,
		DurationMS:          input.DurationMS,
		UpdatedAt:           database.NowMS(),
	}
	if item.Type == "panorama" {
		if err := ValidatePanoramaValues(input.StartYaw, input.StartPitch, input.YawSpeedDPS, input.PitchSpeedDPS); err != nil {
			return model.HomepageMedia{}, err
		}
		patch.StartYaw = input.StartYaw
		patch.StartPitch = input.StartPitch
		patch.YawSpeedDPS = input.YawSpeedDPS
		patch.PitchSpeedDPS = input.PitchSpeedDPS
	}
	item, err = s.DB.HomepageMedia.Patch(ctx, id, patch)
	if err != nil {
		return model.HomepageMedia{}, err
	}
	if err := s.Redis.InvalidatePublicHomepageMedia(ctx); err != nil {
		return model.HomepageMedia{}, err
	}
	return item, nil
}

func (s Service) Reorder(ctx context.Context, actor permission.Actor, ids []string) error {
	if err := requirePermission(actor, homepageMediaUpdatePermission); err != nil {
		return err
	}
	if len(ids) == 0 {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "ids is required"}
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			return util.HTTPError{Status: http.StatusBadRequest, Detail: "ids must be unique non-empty strings"}
		}
		seen[id] = true
	}
	if err := s.DB.HomepageMedia.Reorder(ctx, ids, database.NowMS()); err != nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "homepage media not found"}
	}
	return s.Redis.InvalidatePublicHomepageMedia(ctx)
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, id string) error {
	if err := requirePermission(actor, homepageMediaDeletePermission); err != nil {
		return err
	}
	item, err := s.DB.HomepageMedia.Delete(ctx, id)
	if err != nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "homepage media not found"}
	}
	path := filepath.Join(s.CarouselDir, item.StoragePath)
	if item.Type == "panorama" || strings.Contains(item.StoragePath, "/") {
		path = filepath.Join(s.CarouselDir, item.ID)
	}
	if err := os.RemoveAll(path); err != nil {
		return err
	}
	return s.Redis.InvalidatePublicHomepageMedia(ctx)
}

func (s Service) newMedia(ctx context.Context, typ, title, ext string, values MediaValues) (model.HomepageMedia, string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	order, err := s.DB.HomepageMedia.NextSortOrder(ctx)
	if err != nil {
		return model.HomepageMedia{}, "", err
	}
	duration := values.DurationMS
	if duration == 0 {
		duration = 6000
	}
	if typ == "panorama" && values.DurationMS == 0 {
		duration = 9000
	}
	if duration < 1000 || duration > 60000 {
		return model.HomepageMedia{}, "", util.HTTPError{Status: http.StatusBadRequest, Detail: "duration_ms out of range"}
	}
	now := database.NowMS()
	storagePath := id + ext
	path := filepath.Join(s.CarouselDir, storagePath)
	if typ == "panorama" {
		storagePath = id
		path = filepath.Join(s.CarouselDir, id)
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

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func ParseMediaValues(fields map[string]string, typ string) (MediaValues, error) {
	values := MediaValues{
		StartYaw:      0,
		StartPitch:    0,
		YawSpeedDPS:   4,
		PitchSpeedDPS: 0,
		DurationMS:    intField(fields, "duration_ms", 0),
	}
	var err error
	values.OverlayOpacityLight, err = floatField(fields, "overlay_opacity_light", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "overlay_opacity_light must be a number"}
	}
	values.OverlayOpacityDark, err = floatField(fields, "overlay_opacity_dark", 0.45)
	if err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "overlay_opacity_dark must be a number"}
	}
	if err := validateOpacityValue("overlay_opacity_light", values.OverlayOpacityLight); err != nil {
		return MediaValues{}, err
	}
	if err := validateOpacityValue("overlay_opacity_dark", values.OverlayOpacityDark); err != nil {
		return MediaValues{}, err
	}
	if typ != "panorama" {
		return values, nil
	}
	if values.StartYaw, err = floatField(fields, "start_yaw", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "start_yaw must be a number"}
	}
	if values.StartPitch, err = floatField(fields, "start_pitch", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "start_pitch must be a number"}
	}
	if values.YawSpeedDPS, err = floatField(fields, "yaw_speed_dps", 4); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "yaw_speed_dps must be a number"}
	}
	if values.PitchSpeedDPS, err = floatField(fields, "pitch_speed_dps", 0); err != nil {
		return MediaValues{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "pitch_speed_dps must be a number"}
	}
	if err := ValidatePanoramaValues(&values.StartYaw, &values.StartPitch, &values.YawSpeedDPS, &values.PitchSpeedDPS); err != nil {
		return MediaValues{}, err
	}
	return values, nil
}

func ReadPanoramaZip(data []byte) (map[string][]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid panorama zip"}
	}
	required := map[string]bool{}
	for i := 0; i < 6; i++ {
		required["panorama_"+strconv.Itoa(i)+".png"] = false
	}
	out := map[string][]byte{}
	for _, f := range reader.File {
		name := filepath.ToSlash(f.Name)
		if strings.Contains(name, "/") || strings.Contains(name, `\`) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "panorama files must be at zip root"}
		}
		if _, ok := required[name]; !ok {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "panorama zip must contain only panorama_0.png through panorama_5.png"}
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(io.LimitReader(rc, MaxImageBytes+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if len(content) > MaxImageBytes {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "panorama face too large"}
		}
		if _, _, err := image.DecodeConfig(bytes.NewReader(content)); err != nil {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid panorama face image"}
		}
		required[name] = true
		out[name] = content
	}
	for name, ok := range required {
		if !ok {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "missing " + name}
		}
	}
	return out, nil
}

func ValidateOpacity(name string, v *float64) error {
	if v == nil {
		return nil
	}
	return validateOpacityValue(name, *v)
}

func validateOpacityValue(name string, v float64) error {
	if v < 0 || v > 0.9 {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: name + " out of range"}
	}
	return nil
}

func ValidatePanoramaValues(startYaw, startPitch, yawSpeedDPS, pitchSpeedDPS *float64) error {
	if startYaw != nil && (*startYaw < -360 || *startYaw > 360) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "start_yaw out of range"}
	}
	if startPitch != nil && (*startPitch < -89 || *startPitch > 89) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "start_pitch out of range"}
	}
	if yawSpeedDPS != nil && (*yawSpeedDPS < -90 || *yawSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "yaw_speed_dps out of range"}
	}
	if pitchSpeedDPS != nil && (*pitchSpeedDPS < -90 || *pitchSpeedDPS > 90) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "pitch_speed_dps out of range"}
	}
	return nil
}

type multipartUpload struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func readMultipartUpload(source MultipartSource, maxBytes int64) (multipartUpload, error) {
	reader, err := source.MultipartReader()
	if err != nil {
		return multipartUpload{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid multipart form"}
	}
	out := multipartUpload{Fields: map[string]string{}}
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return multipartUpload{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid multipart form"}
		}
		formName := part.FormName()
		if formName == "" {
			_ = part.Close()
			continue
		}
		if formName == "file" {
			out.Filename = part.FileName()
			data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
			_ = part.Close()
			if err != nil {
				return multipartUpload{}, err
			}
			if int64(len(data)) > maxBytes {
				if maxBytes == MaxPanoramaBytes {
					return multipartUpload{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "File too large"}
				}
				return multipartUpload{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "File too large"}
			}
			out.Data = data
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, 4097))
		_ = part.Close()
		if err != nil {
			return multipartUpload{}, err
		}
		out.Fields[formName] = string(data)
	}
	if out.Filename == "" {
		return multipartUpload{}, util.HTTPError{Status: http.StatusBadRequest, Detail: "file is required"}
	}
	return out, nil
}

func floatField(fields map[string]string, key string, fallback float64) (float64, error) {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseFloat(raw, 64)
}

func intField(fields map[string]string, key string, fallback int) int {
	raw := strings.TrimSpace(fields[key])
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return n
}
