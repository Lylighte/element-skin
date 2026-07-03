package homepage_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestHomepageServiceImageLifecycleExactState(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	dir := t.TempDir()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: dir}
	actor := homepageActor(
		"homepage_media.read.any",
		"homepage_media.create.any",
		"homepage_media.update.any",
		"homepage_media.delete.any",
	)
	if err := redis.SetPublicHomepageMedia(ctx, databaseModelHomepageMediaForTest("stale"), time.Minute); err != nil {
		t.Fatal(err)
	}

	first, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "hero.png", tinyPNGBytes(t), map[string]string{
		"overlay_opacity_light": "0.2",
		"overlay_opacity_dark":  "0.6",
		"duration_ms":           "7000",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != "image" || first.Title != "hero.png" || first.StoragePath != first.ID+".png" ||
		first.OverlayOpacityLight != 0.2 || first.OverlayOpacityDark != 0.6 || first.DurationMS != 7000 ||
		first.Enabled != true || first.SortOrder != 0 {
		t.Fatalf("uploaded image item mismatch: %#v", first)
	}
	if _, err := os.Stat(filepath.Join(dir, first.StoragePath)); err != nil {
		t.Fatalf("uploaded image file should exist: %v", err)
	}
	if _, err := redis.GetPublicHomepageMedia(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("upload should invalidate public homepage cache, err=%v", err)
	}

	second, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "second.webp", []byte("webp bytes"), nil))
	if err != nil {
		t.Fatal(err)
	}
	if second.Type != "image" || second.StoragePath != second.ID+".webp" || second.SortOrder != 1 || second.DurationMS != 6000 {
		t.Fatalf("uploaded webp item mismatch: %#v", second)
	}

	listed, err := svc.List(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].ID != first.ID || listed[1].ID != second.ID {
		t.Fatalf("List should return two media rows in sort order: %#v", listed)
	}

	title := "Updated Hero"
	light := 0.1
	enabled := false
	duration := 8000
	patched, err := svc.Patch(ctx, actor, first.ID, homepagesvc.PatchInput{
		Title: &title, OverlayOpacityLight: &light, Enabled: &enabled, DurationMS: &duration,
	})
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != title || patched.OverlayOpacityLight != light || patched.Enabled != false || patched.DurationMS != duration {
		t.Fatalf("patched image mismatch: %#v", patched)
	}

	if err := svc.Reorder(ctx, actor, []string{second.ID, first.ID}); err != nil {
		t.Fatal(err)
	}
	reordered, err := db.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if reordered[0].ID != second.ID || reordered[0].SortOrder != 0 || reordered[1].ID != first.ID || reordered[1].SortOrder != 1 {
		t.Fatalf("reordered media mismatch: %#v", reordered)
	}

	if err := svc.Delete(ctx, actor, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, first.StoragePath)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted image file should be absent, err=%v", err)
	}
	if _, err := db.HomepageMedia.Get(ctx, first.ID); !database.IsNoRows(err) {
		t.Fatalf("deleted image row should be absent, err=%v", err)
	}
}

func TestHomepageServicePanoramaUploadWritesSixFacesExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	dir := t.TempDir()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: dir}
	actor := homepageActor("homepage_media.create.any")

	item, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "sky.zip", validPanoramaZip(t), map[string]string{
		"start_yaw":     "15",
		"start_pitch":   "-10",
		"yaw_speed_dps": "8",
		"duration_ms":   "12000",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if item.Type != "panorama" || item.Title != "sky.zip" || item.StoragePath != item.ID ||
		item.StartYaw != 15 || item.StartPitch != -10 || item.YawSpeedDPS != 8 || item.PitchSpeedDPS != 0 ||
		item.DurationMS != 12000 || item.Enabled != true {
		t.Fatalf("uploaded panorama item mismatch: %#v", item)
	}
	for i := 0; i < 6; i++ {
		path := filepath.Join(dir, item.ID, "panorama_"+string(rune('0'+i))+".png")
		if info, err := os.Stat(path); err != nil || info.Size() != int64(len(tinyPNGBytes(t))) {
			t.Fatalf("panorama face %d mismatch: info=%#v err=%v", i, info, err)
		}
	}
}

func TestHomepageServiceUploadRollsBackDatabaseAndFilesWhenCacheInvalidationFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	dir := t.TempDir()
	redis := redisstore.NewMemoryStore()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: dir}
	actor := homepageActor("homepage_media.create.any")
	boom := errors.New("redis cache failure")
	redis.Err = boom

	image, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "rollback.png", tinyPNGBytes(t), nil))
	if image != (model.HomepageMedia{}) || !errors.Is(err, boom) {
		t.Fatalf("UploadImage cache failure = item=%#v err=%v; want empty item and redis error", image, err)
	}
	if items, err := db.HomepageMedia.List(ctx, false); err != nil || len(items) != 0 {
		t.Fatalf("rolled back image upload should leave no rows: items=%#v err=%v", items, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("rolled back image upload should leave no files: entries=%#v err=%v", entries, err)
	}

	panorama, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "rollback.zip", validPanoramaZip(t), nil))
	if panorama != (model.HomepageMedia{}) || !errors.Is(err, boom) {
		t.Fatalf("UploadPanorama cache failure = item=%#v err=%v; want empty item and redis error", panorama, err)
	}
	if items, err := db.HomepageMedia.List(ctx, false); err != nil || len(items) != 0 {
		t.Fatalf("rolled back panorama upload should leave no rows: items=%#v err=%v", items, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("rolled back panorama upload should leave no directories: entries=%#v err=%v", entries, err)
	}
}

func TestHomepageServiceUploadRollsBackFilesWhenDatabaseInsertFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	dir := t.TempDir()
	svc := homepagesvc.Service{DB: db, Redis: redisstore.NewMemoryStore(), CarouselDir: dir}
	actor := homepageActor("homepage_media.create.any")
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE homepage_media ADD CONSTRAINT reject_homepage_media_insert CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	image, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "db-fail.png", tinyPNGBytes(t), nil))
	if image != (model.HomepageMedia{}) {
		t.Fatalf("failed image insert returned item=%#v want empty", image)
	}
	assertPgCode(t, err, "23514")
	if items, err := db.HomepageMedia.List(ctx, false); err != nil || len(items) != 0 {
		t.Fatalf("failed image insert should leave no rows: items=%#v err=%v", items, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("failed image insert should remove written file: entries=%#v err=%v", entries, err)
	}

	panorama, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "db-fail.zip", validPanoramaZip(t), nil))
	if panorama != (model.HomepageMedia{}) {
		t.Fatalf("failed panorama insert returned item=%#v want empty", panorama)
	}
	assertPgCode(t, err, "23514")
	if items, err := db.HomepageMedia.List(ctx, false); err != nil || len(items) != 0 {
		t.Fatalf("failed panorama insert should leave no rows: items=%#v err=%v", items, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("failed panorama insert should remove written directory: entries=%#v err=%v", entries, err)
	}
}

func TestHomepageServiceUploadReportsFilesystemCreateErrorsBeforeDatabaseInsert(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	carouselFile := filepath.Join(t.TempDir(), "carousel-file")
	if err := os.WriteFile(carouselFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	svc := homepagesvc.Service{DB: db, Redis: redisstore.NewMemoryStore(), CarouselDir: carouselFile}
	actor := homepageActor("homepage_media.create.any")

	image, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "fs-fail.png", tinyPNGBytes(t), nil))
	if image != (model.HomepageMedia{}) || err == nil {
		t.Fatalf("image filesystem failure item=%#v err=%#v; want empty non-nil error", image, err)
	}
	panorama, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "fs-fail.zip", validPanoramaZip(t), nil))
	if panorama != (model.HomepageMedia{}) || err == nil {
		t.Fatalf("panorama filesystem failure item=%#v err=%#v; want empty non-nil error", panorama, err)
	}
	if items, err := db.HomepageMedia.List(ctx, false); err != nil || len(items) != 0 {
		t.Fatalf("filesystem failures must not create rows: items=%#v err=%v", items, err)
	}
}

func TestHomepageServicePatchPanoramaAndDeleteDirectoryExactState(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	dir := t.TempDir()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: dir}
	actor := homepageActor(
		"homepage_media.create.any",
		"homepage_media.update.any",
		"homepage_media.delete.any",
	)
	item, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "mutable.zip", validPanoramaZip(t), nil))
	if err != nil {
		t.Fatal(err)
	}
	panoramaDir := filepath.Join(dir, item.ID)
	if entries, err := os.ReadDir(panoramaDir); err != nil || len(entries) != 6 {
		t.Fatalf("created panorama directory mismatch: entries=%#v err=%v", entries, err)
	}

	title := "Mutable panorama"
	light := 0.12
	dark := 0.34
	startYaw := -120.5
	startPitch := 22.25
	yawSpeed := 12.75
	pitchSpeed := -4.5
	enabled := false
	duration := 15000
	patched, err := svc.Patch(ctx, actor, item.ID, homepagesvc.PatchInput{
		Title:               &title,
		OverlayOpacityLight: &light,
		OverlayOpacityDark:  &dark,
		StartYaw:            &startYaw,
		StartPitch:          &startPitch,
		YawSpeedDPS:         &yawSpeed,
		PitchSpeedDPS:       &pitchSpeed,
		Enabled:             &enabled,
		DurationMS:          &duration,
	})
	if err != nil {
		t.Fatal(err)
	}
	if patched.ID != item.ID || patched.Type != "panorama" || patched.Title != title ||
		patched.OverlayOpacityLight != light || patched.OverlayOpacityDark != dark ||
		patched.StartYaw != startYaw || patched.StartPitch != startPitch ||
		patched.YawSpeedDPS != yawSpeed || patched.PitchSpeedDPS != pitchSpeed ||
		patched.Enabled != false || patched.DurationMS != duration {
		t.Fatalf("patched panorama mismatch: %#v", patched)
	}

	if err := svc.Delete(ctx, actor, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(panoramaDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted panorama directory should be absent, err=%v", err)
	}
	if _, err := db.HomepageMedia.Get(ctx, item.ID); !database.IsNoRows(err) {
		t.Fatalf("deleted panorama row should be absent, err=%v", err)
	}
}

func TestHomepageServiceRejectsPermissionsAndInvalidInputsExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	ctx := context.Background()
	svc := homepagesvc.Service{DB: db, Redis: redis, CarouselDir: t.TempDir()}
	actor := homepageActor("homepage_media.create.any", "homepage_media.update.any")

	if _, err := svc.List(ctx, permission.Actor{}); !homepageHTTPError(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("List without permission mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, permission.Actor{}, newMultipartSource("file", "hero.png", tinyPNGBytes(t), nil)); !homepageHTTPError(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("UploadImage without permission mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "notes.txt", []byte("not image"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "Unsupported file format") {
		t.Fatalf("unsupported image extension mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "broken.png", []byte("not image"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "invalid image") {
		t.Fatalf("invalid image mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newFieldsOnlyMultipartSource(map[string]string{"title": "missing"})); !homepageHTTPError(err, http.StatusBadRequest, "file is required") {
		t.Fatalf("missing image file mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, failingMultipartSource{}); !homepageHTTPError(err, http.StatusBadRequest, "invalid multipart form") {
		t.Fatalf("invalid multipart image form mismatch: %#v", err)
	}
	if _, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "huge.webp", bytes.Repeat([]byte("x"), homepagesvc.MaxImageBytes+1), nil)); !homepageHTTPError(err, http.StatusBadRequest, "File too large") {
		t.Fatalf("oversized image mismatch: %#v", err)
	}
	if _, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "sky.txt", []byte("zip"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "Unsupported file format") {
		t.Fatalf("unsupported panorama extension mismatch: %#v", err)
	}
	if _, err := svc.UploadPanorama(ctx, actor, failingMultipartSource{}); !homepageHTTPError(err, http.StatusBadRequest, "invalid multipart form") {
		t.Fatalf("invalid multipart panorama form mismatch: %#v", err)
	}
	if _, err := svc.UploadPanorama(ctx, actor, newMultipartSource("file", "sky.zip", []byte("zip"), nil)); !homepageHTTPError(err, http.StatusBadRequest, "invalid panorama zip") {
		t.Fatalf("invalid panorama zip mismatch: %#v", err)
	}
	if _, err := svc.Patch(ctx, actor, "missing", homepagesvc.PatchInput{DurationMS: intPtr(999)}); !homepageHTTPError(err, http.StatusBadRequest, "duration_ms out of range") {
		t.Fatalf("invalid patch duration mismatch: %#v", err)
	}
	if err := svc.Reorder(ctx, actor, nil); !homepageHTTPError(err, http.StatusBadRequest, "ids is required") {
		t.Fatalf("empty reorder mismatch: %#v", err)
	}
	if err := svc.Reorder(ctx, actor, []string{"same", "same"}); !homepageHTTPError(err, http.StatusBadRequest, "ids must be unique non-empty strings") {
		t.Fatalf("duplicate reorder mismatch: %#v", err)
	}
	if err := svc.Delete(ctx, permission.Actor{}, "missing"); !homepageHTTPError(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("Delete without permission mismatch: %#v", err)
	}
}

func TestHomepageServiceClosedDatabaseReturnsDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := homepagesvc.Service{DB: db, Redis: redisstore.NewMemoryStore(), CarouselDir: t.TempDir()}
	readActor := homepageActor("homepage_media.read.any")
	writeActor := homepageActor(
		"homepage_media.create.any",
		"homepage_media.update.any",
		"homepage_media.delete.any",
	)
	db.Close()

	if items, err := svc.List(ctx, readActor); items != nil || !closedPool(err) {
		t.Fatalf("List closed database = items=%#v err=%v; want nil closed pool", items, err)
	}
	if item, err := svc.UploadImage(ctx, writeActor, newMultipartSource("file", "closed.png", tinyPNGBytes(t), nil)); item != (model.HomepageMedia{}) || !closedPool(err) {
		t.Fatalf("UploadImage closed database = item=%#v err=%v; want empty closed pool", item, err)
	}
	if item, err := svc.UploadPanorama(ctx, writeActor, newMultipartSource("file", "closed.zip", validPanoramaZip(t), nil)); item != (model.HomepageMedia{}) || !closedPool(err) {
		t.Fatalf("UploadPanorama closed database = item=%#v err=%v; want empty closed pool", item, err)
	}
	title := "Closed"
	if item, err := svc.Patch(ctx, writeActor, "missing", homepagesvc.PatchInput{Title: &title}); item != (model.HomepageMedia{}) || !homepageHTTPError(err, http.StatusNotFound, "homepage media not found") {
		t.Fatalf("Patch closed database = item=%#v err=%#v; want exact not found mapping", item, err)
	}
	if err := svc.Reorder(ctx, writeActor, []string{"closed"}); !homepageHTTPError(err, http.StatusNotFound, "homepage media not found") {
		t.Fatalf("Reorder closed database err=%#v; want exact not found mapping", err)
	}
	if err := svc.Delete(ctx, writeActor, "closed"); !homepageHTTPError(err, http.StatusNotFound, "homepage media not found") {
		t.Fatalf("Delete closed database err=%#v; want exact not found mapping", err)
	}
}

func TestHomepageMediaParsingAndPanoramaZipValidationExactErrors(t *testing.T) {
	values, err := homepagesvc.ParseMediaValues(map[string]string{
		"overlay_opacity_light": "0.3",
		"overlay_opacity_dark":  "0.7",
		"start_yaw":             "45",
		"start_pitch":           "-20",
		"yaw_speed_dps":         "-5",
		"pitch_speed_dps":       "6",
		"duration_ms":           "9000",
	}, "panorama")
	if err != nil {
		t.Fatal(err)
	}
	if values.OverlayOpacityLight != 0.3 || values.OverlayOpacityDark != 0.7 || values.StartYaw != 45 ||
		values.StartPitch != -20 || values.YawSpeedDPS != -5 || values.PitchSpeedDPS != 6 || values.DurationMS != 9000 {
		t.Fatalf("parsed media values mismatch: %#v", values)
	}

	for _, tc := range []struct {
		name   string
		fields map[string]string
		typ    string
		detail string
	}{
		{"bad light opacity", map[string]string{"overlay_opacity_light": "bad"}, "image", "overlay_opacity_light must be a number"},
		{"dark opacity range", map[string]string{"overlay_opacity_dark": "1"}, "image", "overlay_opacity_dark out of range"},
		{"bad start yaw", map[string]string{"start_yaw": "bad"}, "panorama", "start_yaw must be a number"},
		{"start pitch range", map[string]string{"start_pitch": "-90"}, "panorama", "start_pitch out of range"},
		{"yaw speed range", map[string]string{"yaw_speed_dps": "91"}, "panorama", "yaw_speed_dps out of range"},
		{"pitch speed range", map[string]string{"pitch_speed_dps": "-91"}, "panorama", "pitch_speed_dps out of range"},
		{"bad duration fallback", map[string]string{"duration_ms": "not-number"}, "image", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := homepagesvc.ParseMediaValues(tc.fields, tc.typ)
			if tc.detail == "" {
				if err != nil || got.DurationMS != 0 {
					t.Fatalf("ParseMediaValues fallback mismatch: values=%#v err=%#v", got, err)
				}
				return
			}
			if !homepageHTTPError(err, http.StatusBadRequest, tc.detail) || got != (homepagesvc.MediaValues{}) {
				t.Fatalf("ParseMediaValues error mismatch: values=%#v err=%#v", got, err)
			}
		})
	}

	if faces, err := homepagesvc.ReadPanoramaZip(validPanoramaZip(t)); err != nil || len(faces) != 6 || !bytes.Equal(faces["panorama_0.png"], tinyPNGBytes(t)) {
		t.Fatalf("valid panorama zip mismatch: faces=%#v err=%v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"nested/panorama_0.png": tinyPNGBytes(t)})); !homepageHTTPError(err, http.StatusBadRequest, "panorama files must be at zip root") || faces != nil {
		t.Fatalf("nested panorama zip mismatch: faces=%#v err=%#v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"panorama_6.png": tinyPNGBytes(t)})); !homepageHTTPError(err, http.StatusBadRequest, "panorama zip must contain only panorama_0.png through panorama_5.png") || faces != nil {
		t.Fatalf("extra panorama zip mismatch: faces=%#v err=%#v", faces, err)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, map[string][]byte{"panorama_0.png": []byte("not image")})); !homepageHTTPError(err, http.StatusBadRequest, "invalid panorama face image") || faces != nil {
		t.Fatalf("invalid panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
	missingZero := map[string][]byte{}
	for i := 1; i < 6; i++ {
		missingZero["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, missingZero)); !homepageHTTPError(err, http.StatusBadRequest, "missing panorama_0.png") || faces != nil {
		t.Fatalf("missing panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
	oversized := map[string][]byte{}
	for i := 0; i < 6; i++ {
		oversized["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	oversized["panorama_3.png"] = bytes.Repeat([]byte("x"), homepagesvc.MaxImageBytes+1)
	if faces, err := homepagesvc.ReadPanoramaZip(zipWithFiles(t, oversized)); !homepageHTTPError(err, http.StatusBadRequest, "panorama face too large") || faces != nil {
		t.Fatalf("oversized panorama face mismatch: faces=%#v err=%#v", faces, err)
	}
}

type testMultipartSource struct {
	body     []byte
	boundary string
}

func (s testMultipartSource) MultipartReader() (*multipart.Reader, error) {
	return multipart.NewReader(bytes.NewReader(s.body), s.boundary), nil
}

type failingMultipartSource struct{}

func (failingMultipartSource) MultipartReader() (*multipart.Reader, error) {
	return nil, errors.New("broken multipart")
}

func newMultipartSource(fieldName, filename string, content []byte, fields map[string]string) testMultipartSource {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	part, _ := writer.CreateFormFile(fieldName, filename)
	_, _ = part.Write(content)
	_ = writer.Close()
	return testMultipartSource{body: body.Bytes(), boundary: writer.Boundary()}
}

func newFieldsOnlyMultipartSource(fields map[string]string) testMultipartSource {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range fields {
		_ = writer.WriteField(key, value)
	}
	_ = writer.Close()
	return testMultipartSource{body: body.Bytes(), boundary: writer.Boundary()}
}

func homepageActor(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "homepage-test",
		UserID:      "homepage-test-user",
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func homepageHTTPError(err error, status int, detail string) bool {
	httpErr, ok := err.(util.HTTPError)
	return ok && httpErr.Status == status && httpErr.Detail == detail
}

func closedPool(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func validPanoramaZip(t *testing.T) []byte {
	files := map[string][]byte{}
	for i := 0; i < 6; i++ {
		files["panorama_"+string(rune('0'+i))+".png"] = tinyPNGBytes(t)
	}
	return zipWithFiles(t, files)
}

func zipWithFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body.Bytes()
}

func intPtr(v int) *int {
	return &v
}

func databaseModelHomepageMediaForTest(id string) []model.HomepageMedia {
	return []model.HomepageMedia{{
		ID: id, Type: "image", Title: id, StoragePath: id + ".png",
		OverlayOpacityLight: 0.45, OverlayOpacityDark: 0.45, YawSpeedDPS: 4,
		SortOrder: 0, Enabled: true, DurationMS: 6000, CreatedAt: 1, UpdatedAt: 1,
	}}
}
