package texture_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/testutil"
)

func TestAdminTextureUpdateListDeleteAndMissingSentinel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := texture.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-texture-admin@test.com", "Password123", "DomainTextureAdmin", false)
	if err := store.AddToLibrary(ctx, user.ID, "domain_texture_admin_hash", "skin", "Domain Admin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdateNote(ctx, "domain_texture_admin_hash", "Domain Admin Updated"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdateModel(ctx, "domain_texture_admin_hash", "default"); err != nil {
		t.Fatal(err)
	}
	if err := store.AdminUpdatePublic(ctx, "domain_texture_admin_hash", false); err != nil {
		t.Fatal(err)
	}
	page, err := store.ListAll(ctx, 1, nil, "", "Domain Admin Updated", "skin")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "Domain Admin Updated" || items[0]["model"] != "default" || items[0]["is_public"] != false {
		t.Fatalf("admin list mismatch: %#v", page)
	}
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", false); err == nil || err.Error() != "per-user deletion requires user_id" {
		t.Fatalf("expected per-user deletion validation, got %v", err)
	}
	if err := store.AdminDelete(ctx, "domain_texture_admin_hash", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := store.Exists(ctx, "domain_texture_admin_hash"); err != nil || exists {
		t.Fatalf("texture should be deleted: exists=%v err=%v", exists, err)
	}
	if err := store.AdminUpdateNote(ctx, "missing_domain_texture", "note"); !errors.Is(err, texture.ErrNotFound) {
		t.Fatalf("missing texture should return ErrNotFound, got %v", err)
	}
}
