package homepage_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/homepage"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestMigrateHomepageMediaFilesCreatesHomepageMediaOnceInFilenameOrder(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	dir := t.TempDir()
	for _, name := range []string{"zeta.webp", "notes.txt", "alpha.png"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.MigrateHomepageMediaFiles(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].StoragePath != "alpha.png" || items[0].SortOrder != 0 ||
		items[1].StoragePath != "zeta.webp" || items[1].SortOrder != 1 {
		t.Fatalf("migrated homepage media mismatch: %#v", items)
	}
	if err := os.WriteFile(filepath.Join(dir, "later.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateHomepageMediaFiles(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	again, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 2 {
		t.Fatalf("migration must not append when table is already initialized: %#v", again)
	}
}

func TestNextSortOrderReturnsZeroForEmptyTableAndMaxPlusOne(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := homepage.Store{Pool: db.Pool}
	next, err := store.NextSortOrder(ctx)
	if err != nil || next != 0 {
		t.Fatalf("NextSortOrder empty table mismatch: got=%d err=%v", next, err)
	}
	now := database.NowMS()
	if err := store.Create(ctx, testMedia("first", 0, now)); err != nil {
		t.Fatal(err)
	}
	if err := store.Create(ctx, testMedia("second", 1, now)); err != nil {
		t.Fatal(err)
	}
	next, err = store.NextSortOrder(ctx)
	if err != nil || next != 2 {
		t.Fatalf("NextSortOrder after two items mismatch: got=%d err=%v", next, err)
	}
}

func TestHomepageStorePatchReorderAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	now := database.NowMS()
	item := testMedia("one", 0, now)
	if err := db.HomepageMedia.Create(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	item2 := testMedia("two", 1, now)
	if err := db.HomepageMedia.Create(context.Background(), item2); err != nil {
		t.Fatal(err)
	}
	title := "Updated"
	enabled := false
	duration := 7000
	patched, err := db.HomepageMedia.Patch(context.Background(), "one", homepagePatch(title, enabled, duration, now+1))
	if err != nil {
		t.Fatal(err)
	}
	if patched.Title != title || patched.Enabled || patched.DurationMS != duration || patched.UpdatedAt != now+1 {
		t.Fatalf("patch mismatch: %#v", patched)
	}
	if err := db.HomepageMedia.Reorder(context.Background(), []string{"two", "one"}, now+2); err != nil {
		t.Fatal(err)
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if items[0].ID != "two" || items[0].SortOrder != 0 || items[1].ID != "one" || items[1].SortOrder != 1 {
		t.Fatalf("reorder mismatch: %#v", items)
	}
	deleted, err := db.HomepageMedia.Delete(context.Background(), "two")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.ID != "two" {
		t.Fatalf("deleted item mismatch: %#v", deleted)
	}
}

func testMedia(id string, order int, now int64) model.HomepageMedia {
	return model.HomepageMedia{
		ID: id, Type: "image", Title: id, StoragePath: id + ".png",
		OverlayOpacityLight: 0.45, OverlayOpacityDark: 0.45, YawSpeedDPS: 4,
		SortOrder: order, Enabled: true, DurationMS: 6000, CreatedAt: now, UpdatedAt: now,
	}
}

func homepagePatch(title string, enabled bool, duration int, updatedAt int64) homepage.Patch {
	return homepage.Patch{Title: &title, Enabled: &enabled, DurationMS: &duration, UpdatedAt: updatedAt}
}
