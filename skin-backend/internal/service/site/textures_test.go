package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTexturesApplyUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-textures-service@test.com", "Password123", "SiteTexturesService", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_textures_profile", "SiteTexturesProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_skin", "skin", "Texture Service Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, user.ID, profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updatedProfile.SkinHash == nil || *updatedProfile.SkinHash != "texture_service_skin" || updatedProfile.TextureModel != "slim" {
		t.Fatalf("profile texture state mismatch: profile=%#v err=%v", updatedProfile, err)
	}
	detail, err := svc.UpdateTexture(ctx, user.ID, "texture_service_skin", "skin", map[string]any{"note": "Updated Texture Service", "is_public": false})
	if err != nil || detail["note"] != "Updated Texture Service" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: detail=%#v err=%v", detail, err)
	}
	if err := svc.DeleteTexture(ctx, user.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if info, err := db.Textures.GetInfo(ctx, user.ID, "texture_service_skin", "skin"); err != nil || info != nil {
		t.Fatalf("texture should be deleted: info=%#v err=%v", info, err)
	}
}

func TestUploaderDeleteRemovesWardrobeCopiesButKeepsAppliedProfileHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-delete-owner@test.com", "Password123", "DeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-delete-other@test.com", "Password123", "DeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_delete_profile", "SiteDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_delete_skin", "skin", "Texture Delete Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, other.ID, profile.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, owner.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if exists, err := db.Textures.Exists(ctx, "texture_service_delete_skin", "skin"); err != nil || exists {
		t.Fatalf("uploader delete should remove skin_library row: exists=%v err=%v", exists, err)
	}
	for _, userID := range []string{owner.ID, other.ID} {
		if info, err := db.Textures.GetInfo(ctx, userID, "texture_service_delete_skin", "skin"); err != nil || info != nil {
			t.Fatalf("uploader delete should remove personal library row for %s: info=%#v err=%v", userID, info, err)
		}
	}
	afterDelete, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || afterDelete == nil || afterDelete.SkinHash == nil || *afterDelete.SkinHash != "texture_service_delete_skin" {
		t.Fatalf("applied profile hash should remain until user clears it: profile=%#v err=%v", afterDelete, err)
	}
}

func TestNonUploaderDeleteOnlyDecrementsUsageCount(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-textures-count-owner@test.com", "Password123", "CountOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-count-other@test.com", "Password123", "CountOther", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_count_skin", "skin", "Texture Count Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, other.ID, "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	public, err := svc.PublicLibrary(ctx, "", 10, "skin", "Texture Count", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["usage_count"] != int64(1) {
		t.Fatalf("non-uploader delete should leave owner count only: %#v", public)
	}
	if exists, err := db.Textures.Exists(ctx, "texture_service_count_skin", "skin"); err != nil || !exists {
		t.Fatalf("non-uploader delete should keep library row: exists=%v err=%v", exists, err)
	}
}
