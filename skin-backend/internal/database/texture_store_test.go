package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestTextureStoreUserPublicAndAdminFlows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "texture-owner@test.com", "Password123", "TextureOwner", false)
	other := testutil.CreateUser(t, db, "texture-other@test.com", "Password123", "TextureOther", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "texture_profile", "TextureProfile")

	if err := db.AddTextureToLibrary(ctx, owner.ID, "hash_skin", "skin", "Original Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileSkin(ctx, profile.ID, &[]string{"hash_skin"}[0]); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}

	info, err := db.GetTextureInfo(ctx, owner.ID, "hash_skin", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if info["hash"] != "hash_skin" || info["type"] != "skin" || info["note"] != "Original Skin" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("unexpected owner texture info: %#v", info)
	}
	if ok, err := db.VerifyTextureOwnership(ctx, owner.ID, "hash_skin", "skin"); err != nil || !ok {
		t.Fatalf("owner should own hash_skin: ok=%v err=%v", ok, err)
	}
	if count, err := db.CountTexturesForUser(ctx, owner.ID); err != nil || count != 1 {
		t.Fatalf("owner should have one texture: count=%d err=%v", count, err)
	}

	added, err := db.AddTextureToWardrobe(ctx, other.ID, "hash_skin")
	if err != nil || !added {
		t.Fatalf("other user should add public texture to wardrobe: added=%v err=%v", added, err)
	}
	otherInfo, err := db.GetTextureInfo(ctx, other.ID, "hash_skin", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if otherInfo["note"] != "Original Skin" || otherInfo["is_public"] != 2 {
		t.Fatalf("wardrobe copy should preserve note and mark borrowed public state: %#v", otherInfo)
	}

	userList, err := db.ListUserTextures(ctx, owner.ID, "skin", 1, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	userItems := userList["items"].([]map[string]any)
	if len(userItems) != 1 || userItems[0]["hash"] != "hash_skin" || userItems[0]["note"] != "Original Skin" || userList["has_next"] != false {
		t.Fatalf("unexpected user texture list: %#v", userList)
	}

	publicList, err := db.ListPublicLibrary(ctx, 1, "skin", "Original", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := publicList["items"].([]map[string]any)
	if len(publicItems) != 1 || publicItems[0]["hash"] != "hash_skin" || publicItems[0]["uploader_display_name"] != "TextureOwner" ||
		publicItems[0]["name"] != "Original Skin" || publicList["has_next"] != false {
		t.Fatalf("unexpected public library list: %#v", publicList)
	}

	if err := db.AdminUpdateTextureNote(ctx, "hash_skin", "Admin Note"); err != nil {
		t.Fatal(err)
	}
	if err := db.AdminUpdateTextureModel(ctx, "hash_skin", "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.AdminUpdateTexturePublic(ctx, "hash_skin", false); err != nil {
		t.Fatal(err)
	}
	adminList, err := db.ListAllTextures(ctx, 1, nil, "", "Admin", "skin")
	if err != nil {
		t.Fatal(err)
	}
	adminItems := adminList["items"].([]map[string]any)
	if len(adminItems) != 1 || adminItems[0]["hash"] != "hash_skin" || adminItems[0]["name"] != "Admin Note" ||
		adminItems[0]["model"] != "default" || adminItems[0]["is_public"] != false || adminItems[0]["uploader_email"] != "texture-owner@test.com" {
		t.Fatalf("unexpected admin texture list: %#v", adminList)
	}
	updatedOwnerInfo, err := db.GetTextureInfo(ctx, owner.ID, "hash_skin", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if updatedOwnerInfo["note"] != "Admin Note" || updatedOwnerInfo["model"] != "default" || updatedOwnerInfo["is_public"] != 0 {
		t.Fatalf("admin updates should propagate to owner texture: %#v", updatedOwnerInfo)
	}

	if err := db.AdminDeleteTexture(ctx, "hash_skin", "skin", other.ID, false); err != nil {
		t.Fatal(err)
	}
	if otherInfo, err := db.GetTextureInfo(ctx, other.ID, "hash_skin", "skin"); err != nil || otherInfo != nil {
		t.Fatalf("per-user admin delete should remove only other user's copy: info=%#v err=%v", otherInfo, err)
	}
	if ownerInfo, err := db.GetTextureInfo(ctx, owner.ID, "hash_skin", "skin"); err != nil || ownerInfo == nil {
		t.Fatalf("per-user admin delete should keep owner copy: info=%#v err=%v", ownerInfo, err)
	}
	if err := db.AdminDeleteTexture(ctx, "hash_skin", "skin", "", true); err != nil {
		t.Fatal(err)
	}
	if exists, err := db.TextureExists(ctx, "hash_skin"); err != nil || exists {
		t.Fatalf("force delete should remove skin library row: exists=%v err=%v", exists, err)
	}
}
