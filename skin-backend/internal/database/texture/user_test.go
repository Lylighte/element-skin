package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
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
	if err := store.RecountUsage(ctx, "domain_texture_delete_hash", "skin"); err != nil {
		t.Fatal(err)
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
