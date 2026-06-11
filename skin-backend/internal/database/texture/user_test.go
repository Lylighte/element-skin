package texture_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestUserTextureLibraryCRUDAndPagination(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-texture-user@test.com", "Password123", "DomainTextureUser", false)
	if err := store.AddToLibrary(ctx, user.ID, "domain_texture_user_hash", "skin", "Domain Texture", true, "slim"); err != nil {
		t.Fatal(err)
	}
	info, err := store.GetInfo(ctx, user.ID, "domain_texture_user_hash", "skin")
	if err != nil || info["note"] != "Domain Texture" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("info mismatch: info=%#v err=%v", info, err)
	}
	if ok, err := store.VerifyOwnership(ctx, user.ID, "domain_texture_user_hash", "skin"); err != nil || !ok {
		t.Fatalf("ownership mismatch: ok=%v err=%v", ok, err)
	}
	if count, err := store.CountForUser(ctx, user.ID); err != nil || count != 1 {
		t.Fatalf("count mismatch: count=%d err=%v", count, err)
	}
	page, err := store.ListForUser(ctx, user.ID, "skin", 1, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "domain_texture_user_hash" || page["has_next"] != false {
		t.Fatalf("page mismatch: %#v", page)
	}
	if err := store.UpdateNote(ctx, user.ID, "domain_texture_user_hash", "skin", "Domain Updated"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateModel(ctx, user.ID, "domain_texture_user_hash", "skin", "default"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdatePublic(ctx, user.ID, "domain_texture_user_hash", "skin", false); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"missing note", func() error { return store.UpdateNote(ctx, user.ID, "missing_texture", "skin", "note") }},
		{"missing model", func() error { return store.UpdateModel(ctx, user.ID, "missing_texture", "skin", "slim") }},
		{"missing public", func() error { return store.UpdatePublic(ctx, user.ID, "missing_texture", "skin", true) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !errors.Is(err, texture.ErrNotFound) {
				t.Fatalf("%s should return ErrNotFound, got %v", tc.name, err)
			}
		})
	}
	info, err = store.GetInfo(ctx, user.ID, "domain_texture_user_hash", "skin")
	if err != nil || info["note"] != "Domain Updated" || info["model"] != "default" || info["is_public"] != 0 {
		t.Fatalf("updated info mismatch: info=%#v err=%v", info, err)
	}
	if uploader, exists, err := store.LibraryUploader(ctx, "domain_texture_user_hash", "skin"); err != nil || !exists || uploader != user.ID {
		t.Fatalf("LibraryUploader should return uploader: uploader=%q exists=%v err=%v", uploader, exists, err)
	}
	if uploader, exists, err := store.LibraryUploader(ctx, "missing_texture", "skin"); err != nil || exists || uploader != "" {
		t.Fatalf("missing LibraryUploader should return exists=false: uploader=%q exists=%v err=%v", uploader, exists, err)
	}
	if err := store.RecountUsage(ctx, "domain_texture_user_hash", "elytra"); err == nil || err.Error() != "invalid texture_type" {
		t.Fatalf("invalid recount texture type should reject, got %v", err)
	}
	deleted, err := store.DeleteFromLibrary(ctx, user.ID, "domain_texture_user_hash", "skin")
	if err != nil || !deleted {
		t.Fatalf("delete mismatch: deleted=%v err=%v", deleted, err)
	}
	if deleted, err := store.DeleteFromLibrary(ctx, user.ID, "domain_texture_user_hash", "skin"); err != nil || deleted {
		t.Fatalf("delete missing personal texture should return false: deleted=%v err=%v", deleted, err)
	}
}

func TestUserTextureListCursorAdvancesAcrossEqualTimestampsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "texture-user-cursor@test.com", "Password123", "TextureUserCursor", false)
	for _, item := range []struct {
		hash        string
		textureType string
		note        string
	}{
		{hash: "cursor_skin_a", textureType: "skin", note: "Skin A"},
		{hash: "cursor_skin_b", textureType: "skin", note: "Skin B"},
		{hash: "cursor_skin_c", textureType: "skin", note: "Skin C"},
		{hash: "cursor_cape_z", textureType: "cape", note: "Cape Z"},
	} {
		if err := store.AddToLibrary(ctx, user.ID, item.hash, item.textureType, item.note, false, "default"); err != nil {
			t.Fatal(err)
		}
	}
	const createdAt int64 = 1700000000123
	if _, err := db.Pool.Exec(ctx, `UPDATE user_textures SET created_at=$1 WHERE user_id=$2`, createdAt, user.ID); err != nil {
		t.Fatal(err)
	}

	first, err := store.ListForUser(ctx, user.ID, "skin", 2, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := first["items"].([]map[string]any)
	next := first["next_key"].(map[string]any)
	if len(firstItems) != 2 ||
		firstItems[0]["hash"] != "cursor_skin_c" ||
		firstItems[0]["note"] != "Skin C" ||
		firstItems[0]["created_at"] != createdAt ||
		firstItems[1]["hash"] != "cursor_skin_b" ||
		first["has_next"] != true ||
		first["page_size"] != 2 ||
		next["last_created_at"] != createdAt ||
		next["last_hash"] != "cursor_skin_b" ||
		first["next_cursor"] != util.EncodeCursor(next) {
		t.Fatalf("first texture cursor page mismatch: %#v", first)
	}

	cursorCreated := next["last_created_at"].(int64)
	second, err := store.ListForUser(
		ctx,
		user.ID,
		"skin",
		2,
		&cursorCreated,
		next["last_hash"].(string),
	)
	if err != nil {
		t.Fatal(err)
	}
	secondItems := second["items"].([]map[string]any)
	secondNext, nextOK := second["next_key"].(map[string]any)
	if len(secondItems) != 1 ||
		secondItems[0]["hash"] != "cursor_skin_a" ||
		secondItems[0]["note"] != "Skin A" ||
		second["has_next"] != false ||
		!nextOK ||
		secondNext != nil ||
		second["next_cursor"] != "" ||
		second["page_size"] != 1 {
		t.Fatalf("second texture cursor page mismatch: %#v", second)
	}
}

func TestUserTextureDeleteOnlyRemovesOnePersonalLibraryRow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "domain-texture-delete-owner@test.com", "Password123", "DeleteOwner", false)
	other := testutil.CreateUser(t, db, "domain-texture-delete-other@test.com", "Password123", "DeleteOther", false)
	if err := store.AddToLibrary(ctx, owner.ID, "domain_texture_delete_hash", "skin", "Delete Count", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := store.AddToWardrobe(ctx, other.ID, "domain_texture_delete_hash", "skin"); err != nil || !added {
		t.Fatalf("wardrobe add mismatch: added=%v err=%v", added, err)
	}
	deleted, err := store.DeleteFromLibrary(ctx, other.ID, "domain_texture_delete_hash", "skin")
	if err != nil || !deleted {
		t.Fatalf("delete mismatch: deleted=%v err=%v", deleted, err)
	}
	page, err := store.ListPublic(ctx, texture.PublicListOptions{Limit: 1, Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["usage_count"] != int64(1) {
		t.Fatalf("usage_count should remain owner-only after non-uploader deletion: %#v", page)
	}
	if ok, err := store.VerifyOwnership(ctx, owner.ID, "domain_texture_delete_hash", "skin"); err != nil || !ok {
		t.Fatalf("owner row should remain: ok=%v err=%v", ok, err)
	}
	if exists, err := store.Exists(ctx, "domain_texture_delete_hash", "skin"); err != nil || !exists {
		t.Fatalf("skin_library row should remain: exists=%v err=%v", exists, err)
	}
}

func TestDeleteFromLibraryRollsBackRowWhenUsageRecountFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "texture-delete-rollback-owner@test.com", "Password123", "DeleteRollbackOwner", false)
	other := testutil.CreateUser(t, db, "texture-delete-rollback-other@test.com", "Password123", "DeleteRollbackOther", false)
	const hash = "texture_delete_recount_rollback"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Delete Rollback", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := store.AddToWardrobe(ctx, other.ID, hash, "skin"); err != nil || !added {
		t.Fatalf("AddToWardrobe = %v, %v; want true, nil", added, err)
	}
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE skin_library
		ADD CONSTRAINT usage_count_at_least_two CHECK (usage_count >= 2)
	`); err != nil {
		t.Fatal(err)
	}

	deleted, err := store.DeleteFromLibrary(ctx, other.ID, hash, "skin")
	var pgErr *pgconn.PgError
	if deleted || !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("DeleteFromLibrary = %v, %#v; want false and PostgreSQL 23514", deleted, err)
	}
	if owned, err := store.VerifyOwnership(ctx, other.ID, hash, "skin"); err != nil || !owned {
		t.Fatalf("failed delete must preserve wardrobe row: owned=%v err=%v", owned, err)
	}
	var usage int64
	if err := db.Pool.QueryRow(ctx,
		`SELECT usage_count FROM skin_library WHERE skin_hash=$1 AND texture_type='skin'`,
		hash,
	).Scan(&usage); err != nil {
		t.Fatal(err)
	}
	if usage != 2 {
		t.Fatalf("failed delete changed usage_count: got=%d want=2", usage)
	}
}

func TestDeleteLibraryTextureRollsBackWardrobeRowsWhenLibraryDeleteFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "delete-library-rollback-owner@test.com", "Password123", "DeleteLibraryRollbackOwner", false)
	collector := testutil.CreateUser(t, db, "delete-library-rollback-collector@test.com", "Password123", "DeleteLibraryRollbackCollector", false)
	const hash = "delete_library_rollback"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Rollback", true, "default"); err != nil {
		t.Fatal(err)
	}
	if added, err := store.AddToWardrobe(ctx, collector.ID, hash, "skin"); err != nil || !added {
		t.Fatalf("wardrobe add = %v, %v; want true, nil", added, err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_test_library_delete() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'test library delete rejected'
				USING ERRCODE = '23514', CONSTRAINT = 'skin_library_delete_guard';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_test_library_delete
		BEFORE DELETE ON skin_library
		FOR EACH ROW EXECUTE FUNCTION reject_test_library_delete();
	`); err != nil {
		t.Fatal(err)
	}

	err := store.DeleteLibraryTexture(ctx, hash, "skin")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "skin_library_delete_guard" {
		t.Fatalf("library delete error=%#v, want exact skin_library_delete_guard violation", err)
	}
	for _, userID := range []string{owner.ID, collector.ID} {
		if info, err := store.GetInfo(ctx, userID, hash, "skin"); err != nil || info == nil {
			t.Fatalf("failed library delete must preserve user texture %q: info=%#v err=%v", userID, info, err)
		}
	}
	var usage int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT usage_count FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash).Scan(&usage); err != nil {
		t.Fatal(err)
	}
	if usage != 2 {
		t.Fatalf("failed library delete changed usage_count=%d, want 2", usage)
	}
}

func TestDeleteLibraryTextureReturnsNotFoundForMissingRow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	store := texture.Store{Pool: db.Pool}
	if err := store.DeleteLibraryTexture(t.Context(), "missing_library_texture", "skin"); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing library delete error=%v, want ErrNotFound", err)
	}
}

func TestConcurrentDeleteFromLibraryKeepsExactUsageCount(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	owner := testutil.CreateUser(t, db, "delete-concurrent-owner@test.com", "Password123", "DeleteConcurrentOwner", false)
	first := testutil.CreateUser(t, db, "delete-concurrent-first@test.com", "Password123", "DeleteConcurrentFirst", false)
	second := testutil.CreateUser(t, db, "delete-concurrent-second@test.com", "Password123", "DeleteConcurrentSecond", false)
	const hash = "delete_concurrent_usage"
	if err := store.AddToLibrary(ctx, owner.ID, hash, "skin", "Concurrent Usage", true, "default"); err != nil {
		t.Fatal(err)
	}
	for _, userID := range []string{first.ID, second.ID} {
		if added, err := store.AddToWardrobe(ctx, userID, hash, "skin"); err != nil || !added {
			t.Fatalf("add wardrobe for %q = %v, %v; want true, nil", userID, added, err)
		}
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_concurrent_wardrobe_delete() RETURNS trigger AS $$
		BEGIN
			IF OLD.hash = 'delete_concurrent_usage' THEN
				PERFORM pg_sleep(0.2);
			END IF;
			RETURN OLD;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_concurrent_wardrobe_delete
		BEFORE DELETE ON user_textures
		FOR EACH ROW EXECUTE FUNCTION delay_concurrent_wardrobe_delete();
	`); err != nil {
		t.Fatal(err)
	}

	type result struct {
		deleted bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, userID := range []string{first.ID, second.ID} {
		userID := userID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			deleted, err := store.DeleteFromLibrary(context.Background(), userID, hash, "skin")
			results <- result{deleted: deleted, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for result := range results {
		if result.err != nil || !result.deleted {
			t.Fatalf("concurrent wardrobe delete = %v, %v; want true, nil", result.deleted, result.err)
		}
	}
	for _, userID := range []string{first.ID, second.ID} {
		if owned, err := store.VerifyOwnership(ctx, userID, hash, "skin"); err != nil || owned {
			t.Fatalf("deleted user %q ownership=%v err=%v; want false, nil", userID, owned, err)
		}
	}
	var usage int64
	if err := db.Pool.QueryRow(ctx, `
		SELECT usage_count FROM skin_library
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash).Scan(&usage); err != nil {
		t.Fatal(err)
	}
	if usage != 1 {
		t.Fatalf("usage_count after concurrent wardrobe deletes=%d; want exact owner-only count 1", usage)
	}
}
