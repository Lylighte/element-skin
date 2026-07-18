package homepage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
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

	second, err := svc.UploadImage(ctx, actor, newMultipartSource("file", "second.webp", tinyWebPBytes(t), nil))
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
