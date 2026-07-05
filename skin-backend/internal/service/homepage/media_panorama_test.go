package homepage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"element-skin/backend/internal/database"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
)

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
