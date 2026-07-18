package texture_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestPublicLibraryAndWardrobeCopyVisibilityRules(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-texture-library-owner@test.com", "Password123", "DomainTextureLibraryOwner", false)
	other := testutil.CreateUser(t, db, "domain-texture-library-other@test.com", "Password123", "DomainTextureLibraryOther", false)
	if err := store.AddToLibrary(ctx, owner.ID, "domain_texture_library_hash", "skin", "Domain Library", true, "default"); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 1, TextureType: "skin", Query: "Domain Library"})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "domain_texture_library_hash" || items[0]["uploader_display_name"] != "DomainTextureLibraryOwner" {
		t.Fatalf("public library mismatch: %#v", page)
	}
	added, err := store.AddToWardrobe(ctx, other.ID, "domain_texture_library_hash", "skin")
	if err != nil || !added {
		t.Fatalf("wardrobe add mismatch: added=%v err=%v", added, err)
	}
	info, err := store.GetInfo(ctx, other.ID, "domain_texture_library_hash", "skin")
	if err != nil || info["note"] != "Domain Library" || info["is_public"] != 2 {
		t.Fatalf("wardrobe copy mismatch: info=%#v err=%v", info, err)
	}
	if err := store.AddToLibrary(ctx, owner.ID, "domain_private_library_hash", "skin", "Private Library", false, "default"); err != nil {
		t.Fatal(err)
	}
	added, err = store.AddToWardrobe(ctx, other.ID, "domain_private_library_hash", "skin")
	if err != nil || added {
		t.Fatalf("private library texture should not be wardrobe-addable: added=%v err=%v", added, err)
	}
}

func TestParsePublicLibrarySortNormalizesSupportedValueAndFallsBack(t *testing.T) {
	for _, raw := range []string{"most_used", " MOST_USED ", "Most_Used"} {
		if got := texture.ParsePublicLibrarySort(raw); got != texture.PublicLibrarySortMostUsed {
			t.Fatalf("ParsePublicLibrarySort(%q)=%q, want %q", raw, got, texture.PublicLibrarySortMostUsed)
		}
	}
	for _, raw := range []string{"", "latest", " Latest ", "popular", "most-used"} {
		if got := texture.ParsePublicLibrarySort(raw); got != texture.PublicLibrarySortLatest {
			t.Fatalf("ParsePublicLibrarySort(%q)=%q, want %q", raw, got, texture.PublicLibrarySortLatest)
		}
	}
}

func TestPublicLibrarySortsByLatestAndMostUsed(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-library-sort-owner@test.com", "Password123", "SortOwner", false)
	userA := testutil.CreateUser(t, db, "domain-library-sort-a@test.com", "Password123", "SortUserA", false)
	userB := testutil.CreateUser(t, db, "domain-library-sort-b@test.com", "Password123", "SortUserB", false)

	for _, item := range []struct {
		hash string
		name string
	}{
		{hash: "sort_old", name: "Sort Old"},
		{hash: "sort_middle", name: "Sort Middle"},
		{hash: "sort_new", name: "Sort New"},
	} {
		if err := store.AddToLibrary(ctx, owner.ID, item.hash, "skin", item.name, true, "default"); err != nil {
			t.Fatal(err)
		}
	}
	addWardrobe := func(userID, hash string) {
		t.Helper()
		added, err := store.AddToWardrobe(ctx, userID, hash, "skin")
		if err != nil || !added {
			t.Fatalf("AddToWardrobe user=%s hash=%s added=%v err=%v", userID, hash, added, err)
		}
	}
	addWardrobe(userA.ID, "sort_old")
	addWardrobe(userB.ID, "sort_old")
	addWardrobe(userA.ID, "sort_middle")
	duplicate, err := store.AddToWardrobe(ctx, userA.ID, "sort_old", "skin")
	if err != nil || !duplicate {
		t.Fatalf("duplicate AddToWardrobe should return found=true without changing count: added=%v err=%v", duplicate, err)
	}

	latest, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 3, Sort: texture.PublicLibrarySortLatest})
	if err != nil {
		t.Fatal(err)
	}
	latestItems := latest["items"].([]map[string]any)
	assertHashes(t, latestItems, []string{"sort_new", "sort_middle", "sort_old"})

	mostUsed, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 2, Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	mostUsedItems := mostUsed["items"].([]map[string]any)
	assertHashes(t, mostUsedItems, []string{"sort_old", "sort_middle"})
	if mostUsedItems[0]["usage_count"] != int64(3) || mostUsedItems[1]["usage_count"] != int64(2) {
		t.Fatalf("usage_count should count users with personal library rows: %#v", mostUsedItems)
	}
	cursor, _ := mostUsed["next_cursor"].(string)
	if cursor == "" || mostUsed["has_next"] != true {
		t.Fatalf("most_used first page should have cursor: %#v", mostUsed)
	}
	next, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 2, Sort: texture.PublicLibrarySortMostUsed, LastUsage: int64Ptr(mostUsedItems[1]["usage_count"].(int64)), LastCreated: int64Ptr(mostUsedItems[1]["created_at"].(int64)), LastHash: mostUsedItems[1]["hash"].(string)})
	if err != nil {
		t.Fatal(err)
	}
	assertHashes(t, next["items"].([]map[string]any), []string{"sort_new"})
}

func TestUsageCountRecountAndUploaderDeleteSemantics(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-library-usage-owner@test.com", "Password123", "UsageOwner", false)
	userA := testutil.CreateUser(t, db, "domain-library-usage-a@test.com", "Password123", "UsageUserA", false)
	userB := testutil.CreateUser(t, db, "domain-library-usage-b@test.com", "Password123", "UsageUserB", false)

	if err := store.AddToLibrary(ctx, owner.ID, "usage_recount_hash", "skin", "Usage Recount", true, "slim"); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{userA.ID, userB.ID} {
		added, err := store.AddToWardrobe(ctx, userID, "usage_recount_hash", "skin")
		if err != nil || !added {
			t.Fatalf("AddToWardrobe user=%s added=%v err=%v", userID, added, err)
		}
	}
	duplicate, err := store.AddToWardrobe(ctx, userA.ID, "usage_recount_hash", "skin")
	if err != nil || !duplicate {
		t.Fatalf("duplicate wardrobe add should still report found texture: added=%v err=%v", duplicate, err)
	}
	assertPublicUsage(t, store, "usage_recount_hash", int64(3))

	deleted, err := store.DeleteFromLibrary(ctx, userA.ID, "usage_recount_hash", "skin")
	if err != nil || !deleted {
		t.Fatalf("DeleteFromLibrary userA deleted=%v err=%v", deleted, err)
	}
	if err := store.RecountUsage(ctx, "usage_recount_hash", "skin"); err != nil {
		t.Fatal(err)
	}
	assertPublicUsage(t, store, "usage_recount_hash", int64(2))

	if err := store.DeleteLibraryTexture(ctx, "usage_recount_hash", "skin"); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "usage_recount_hash", "skin"); err != nil || exists {
		t.Fatalf("DeleteLibraryTexture should remove public library row: exists=%v err=%v", exists, err)
	}
	for _, userID := range []string{owner.ID, userB.ID} {
		info, err := store.GetInfo(ctx, userID, "usage_recount_hash", "skin")
		if err != nil || info != nil {
			t.Fatalf("DeleteLibraryTexture should remove wardrobe row for %s: info=%#v err=%v", userID, info, err)
		}
	}
}

func TestAddToWardrobeReturnsNotAddedAfterConcurrentLibraryDelete(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "wardrobe-delete-race-owner@test.com", "Password123", "WardrobeDeleteRaceOwner", false)
	collector := testutil.CreateUser(t, db, "wardrobe-delete-race-collector@test.com", "Password123", "WardrobeDeleteRaceCollector", false)
	const hash = "wardrobe_delete_race"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Delete Race", true, "default"); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `
		SELECT 1, pg_backend_pid()
		FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
		FOR UPDATE
	`, hash).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}
	type result struct {
		added bool
		err   error
	}
	results := make(chan result, 1)
	go func() {
		added, err := store.AddToWardrobe(context.Background(), collector.ID, hash, "skin")
		results <- result{added: added, err: err}
	}()
	waitForLibraryMutationBlock(t, db.Pool, lockHolderPID, results)
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	got := <-results
	if got.err != nil || got.added {
		t.Fatalf("wardrobe add after concurrent delete=%v, %v; want false, nil", got.added, got.err)
	}
	if info, err := store.GetInfo(ctx, collector.ID, hash, "skin"); err != nil || info != nil {
		t.Fatalf("failed concurrent wardrobe add left personal row: info=%#v err=%v", info, err)
	}
	if exists, err := store.Exists(ctx, hash, "skin"); err != nil || exists {
		t.Fatalf("deleted public library row exists=%v err=%v; want false, nil", exists, err)
	}
}

func TestAddToLibraryRecreatesExactStateAfterConcurrentDelete(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "upload-delete-race-owner@test.com", "Password123", "UploadDeleteRaceOwner", false)
	const hash = "upload_delete_race"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Old Upload", true, "default"); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `
		SELECT 1, pg_backend_pid()
		FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
		FOR UPDATE
	`, hash).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 1)
	go func() {
		results <- store.AddToLibrary(
			context.Background(),
			owner.ID,
			hash,
			"skin",
			"Reuploaded",
			false,
			"slim",
		)
	}()
	waitForLibraryMutationBlock(t, db.Pool, lockHolderPID, results)
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`, hash); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := <-results; err != nil {
		t.Fatalf("reupload after concurrent delete failed: %v", err)
	}
	info, err := store.GetInfo(ctx, owner.ID, hash, "skin")
	if err != nil || info == nil ||
		info["note"] != "Reuploaded" ||
		info["model"] != "slim" ||
		info["is_public"] != 0 {
		t.Fatalf("recreated personal texture=%#v err=%v; want exact reupload state", info, err)
	}
	var name, model string
	var public int
	var usage int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT name,model,is_public,usage_count
		FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash).Scan(&name, &model, &public, &usage); err != nil {
		t.Fatal(err)
	}
	if name != "Reuploaded" || model != "slim" || public != 0 || usage != 1 {
		t.Fatalf("recreated library state=(%q,%q,%d,%d); want (Reuploaded,slim,0,1)", name, model, public, usage)
	}
}

func waitForLibraryMutationBlock[T any](
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	lockHolderPID int,
	result <-chan T,
) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case <-result:
			t.Fatal("texture library mutation completed before row-lock release")
		default:
		}
		var waiting bool
		if err := db.QueryRow(t.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("texture library mutation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertHashes(t *testing.T, items []map[string]any, want []string) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("item count mismatch want=%v got=%#v", want, items)
	}
	for i, hash := range want {
		if items[i]["hash"] != hash {
			t.Fatalf("item %d hash mismatch want=%s got=%#v", i, hash, items)
		}
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}

func assertPublicUsage(t *testing.T, store texture.Store, hash string, want int64) {
	t.Helper()
	page, err := store.ListPublic(context.Background(), texture.PublicListOptions{Limit: 5, Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range page["items"].([]map[string]any) {
		if item["hash"] == hash {
			if item["usage_count"] != want {
				t.Fatalf("usage_count mismatch for %s want=%d got=%#v", hash, want, item)
			}
			return
		}
	}
	t.Fatalf("missing public library item %s in %#v", hash, page)
}
