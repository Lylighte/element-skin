package integration_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/testutil"
)

func TestDatabaseUserProfileTokenAndTextureCRUD(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "crud@test.com", "Password123", "CrudUser", false)

	if count, err := db.Users.Count(ctx); err != nil || count != 1 {
		t.Fatalf("CountUsers=%d err=%v", count, err)
	}
	if taken, err := db.Users.IsDisplayNameTaken(ctx, "CrudUser", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: %v", err)
	}
	if err := db.Users.Update(ctx, user.ID, map[string]any{"email": "new@crud.com", "display_name": "NewCrud", "preferred_language": "en_US"}); err != nil {
		t.Fatal(err)
	}
	updated, _ := db.Users.GetByID(ctx, user.ID)
	if updated.Email != "new@crud.com" || updated.DisplayName != "NewCrud" || updated.PreferredLanguage != "en_US" {
		t.Fatalf("unexpected updated user: %#v", updated)
	}
	if err := db.Users.Ban(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.Users.IsBanned(ctx, user.ID); err != nil || !banned {
		t.Fatalf("expected banned user: %v", err)
	}
	if err := db.Users.Unban(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if banned, _ := db.Users.IsBanned(ctx, user.ID); banned {
		t.Fatal("expected unbanned user")
	}

	profile := testutil.CreateProfile(t, db, user.ID, "crud_profile", "CrudPlayer")
	skin := "skin_hash"
	cape := "cape_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(ctx, profile.ID, &cape); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateModel(ctx, profile.ID, "slim"); err != nil {
		t.Fatal(err)
	}
	gotProfile, _ := db.Profiles.GetByID(ctx, profile.ID)
	if *gotProfile.SkinHash != skin || *gotProfile.CapeHash != cape || gotProfile.TextureModel != "slim" {
		t.Fatalf("unexpected profile: %#v", gotProfile)
	}

	if ok, err := db.Profiles.DeleteCascade(ctx, profile.ID); err != nil || !ok {
		t.Fatalf("DeleteProfileCascade ok=%v err=%v", ok, err)
	}

	if err := db.Textures.AddToLibrary(ctx, user.ID, "texhash", "skin", "MySkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if info, _ := db.Textures.GetInfo(ctx, user.ID, "texhash", "skin"); info["note"] != "MySkin" || info["is_public"].(int) != 1 {
		t.Fatalf("unexpected texture info: %#v", info)
	}
	if err := db.Textures.UpdateNote(ctx, user.ID, "texhash", "skin", "NewNote"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.UpdatePublic(ctx, user.ID, "texhash", "skin", false); err != nil {
		t.Fatal(err)
	}
	other := testutil.CreateUser(t, db, "other@test.com", "Password123", "Other", false)
	ok, err := db.Textures.AddToWardrobe(ctx, other.ID, "texhash", "skin")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("other user should not add private texture")
	}
	if err := db.Textures.UpdatePublic(ctx, user.ID, "texhash", "skin", true); err != nil {
		t.Fatal(err)
	}
	ok, err = db.Textures.AddToWardrobe(ctx, other.ID, "texhash", "skin")
	if err != nil || !ok {
		t.Fatalf("public wardrobe add ok=%v err=%v", ok, err)
	}
	if info, _ := db.Textures.GetInfo(ctx, other.ID, "texhash", "skin"); info == nil || info["is_public"].(int) != 2 {
		t.Fatalf("wardrobe copy should use is_public=2, got %#v", info)
	}

	modelHash := "modelhash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, modelHash, "skin", "ModelSkin", true, "default"); err != nil {
		t.Fatal(err)
	}
	modelProfile := testutil.CreateProfile(t, db, user.ID, "model_profile", "ModelTester")
	if err := db.Profiles.UpdateSkin(ctx, modelProfile.ID, &modelHash); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.UpdateModel(ctx, user.ID, modelHash, "skin", "slim"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ := db.Profiles.GetByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("owner model update should cascade to profile, got %#v", updatedModelProfile)
	}
	otherModelUser := testutil.CreateUser(t, db, "other-model@test.com", "Password123", "OtherModel", false)
	if ok, err := db.Textures.AddToWardrobe(ctx, otherModelUser.ID, modelHash, "skin"); err != nil || !ok {
		t.Fatalf("other model wardrobe add ok=%v err=%v", ok, err)
	}
	if err := db.Textures.UpdateModel(ctx, otherModelUser.ID, modelHash, "skin", "default"); err != nil {
		t.Fatal(err)
	}
	updatedModelProfile, _ = db.Profiles.GetByID(ctx, modelProfile.ID)
	if updatedModelProfile.TextureModel != "slim" {
		t.Fatalf("non-uploader model update should not cascade owner profile, got %#v", updatedModelProfile)
	}

	privateHash := "private-readd-hash"
	if err := db.Textures.AddToLibrary(ctx, user.ID, privateHash, "skin", "PrivateSkin", false, "default"); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Textures.AddToWardrobe(ctx, other.ID, privateHash, "skin"); err != nil || ok {
		t.Fatalf("private library texture should not be addable through public wardrobe flow ok=%v err=%v", ok, err)
	}
}
