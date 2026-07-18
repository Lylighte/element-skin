package database_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestDBInitResetAndCoreHelpersExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Init(ctx); err != nil {
		t.Fatalf("Init should be idempotent: %v", err)
	}
	if siteName, err := db.Settings.Get(ctx, "site_name", ""); err != nil || siteName != "皮肤站" {
		t.Fatalf("Init should seed site_name: site_name=%q err=%v", siteName, err)
	}
	testutil.CreateUser(t, db, "db-reset@test.com", "Password123", "DBReset", false)
	if err := db.ResetPublicSchema(ctx); err != nil {
		t.Fatal(err)
	}
	if count, err := db.Users.Count(ctx); err != nil || count != 0 {
		t.Fatalf("reset should remove users: count=%d err=%v", count, err)
	}
	if !database.IsNoRows(pgx.ErrNoRows) || database.IsNoRows(nil) {
		t.Fatal("IsNoRows should match pgx.ErrNoRows only")
	}
	if now := database.NowMS(); now <= 0 {
		t.Fatalf("NowMS should be positive: %d", now)
	}
}

func TestMigrateHomepageMediaFilesCreatesExactImageRowsAndSkipsWhenPopulated(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	dir := t.TempDir()
	files := map[string]string{
		"b.webp":   "webp",
		"a.png":    "png",
		"c.jpeg":   "jpeg",
		"skip.txt": "txt",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "nested.jpg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateHomepageMediaFiles(ctx, dir); err != nil {
		t.Fatal(err)
	}
	items, err := db.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("migrated homepage media count mismatch: got=%d items=%#v", len(items), items)
	}
	expected := []struct {
		id        string
		title     string
		sortOrder int
	}{
		{"a", "a.png", 0},
		{"b", "b.webp", 1},
		{"c", "c.jpeg", 2},
	}
	for i, want := range expected {
		got := items[i]
		if got.ID != want.id ||
			got.Type != "image" ||
			got.Title != want.title ||
			got.StoragePath != want.title ||
			got.OverlayOpacityLight != 0.45 ||
			got.OverlayOpacityDark != 0.45 ||
			got.YawSpeedDPS != 4 ||
			got.SortOrder != want.sortOrder ||
			!got.Enabled ||
			got.DurationMS != 6000 ||
			got.CreatedAt <= 0 ||
			got.UpdatedAt != got.CreatedAt {
			t.Fatalf("migrated homepage item %d mismatch: %#v", i, got)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "d.jpg"), []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateHomepageMediaFiles(ctx, dir); err != nil {
		t.Fatal(err)
	}
	items, err = db.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].ID != "a" || items[1].ID != "b" || items[2].ID != "c" {
		t.Fatalf("second migration should skip populated table exactly: %#v", items)
	}

	emptyDB, _ := testutil.NewTestApp(t)
	if err := emptyDB.MigrateHomepageMediaFiles(ctx, filepath.Join(dir, "missing")); err != nil {
		t.Fatalf("missing homepage directory should be ignored, got %v", err)
	}
	if emptyItems, err := emptyDB.HomepageMedia.List(ctx, false); err != nil || len(emptyItems) != 0 {
		t.Fatalf("missing directory should not create rows: items=%#v err=%v", emptyItems, err)
	}

	existingDB, _ := testutil.NewTestApp(t)
	now := database.NowMS()
	if err := existingDB.HomepageMedia.Create(ctx, model.HomepageMedia{
		ID:                  "existing",
		Type:                "image",
		Title:               "Existing",
		StoragePath:         "existing.png",
		OverlayOpacityLight: 0.2,
		OverlayOpacityDark:  0.3,
		YawSpeedDPS:         1,
		SortOrder:           9,
		Enabled:             true,
		DurationMS:          4000,
		CreatedAt:           now,
		UpdatedAt:           now,
	}); err != nil {
		t.Fatal(err)
	}
	if err := existingDB.MigrateHomepageMediaFiles(ctx, dir); err != nil {
		t.Fatal(err)
	}
	existingItems, err := existingDB.HomepageMedia.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(existingItems) != 1 || existingItems[0].ID != "existing" || existingItems[0].Title != "Existing" {
		t.Fatalf("populated table migration should keep exact existing row: %#v", existingItems)
	}
}
