package homepage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	homepagesvc "element-skin/backend/internal/service/homepage"
	"element-skin/backend/internal/testutil"
)

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
