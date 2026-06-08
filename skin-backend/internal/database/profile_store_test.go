package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestProfileStoreCRUDPaginationAndCascade(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "profiles@test.com", "Password123", "ProfileOwner", false)
	skin := "skin_hash"
	cape := "cape_hash"
	if err := db.CreateProfile(ctx, model.Profile{ID: "profile_a", UserID: user.ID, Name: "Alpha", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateProfile(ctx, model.Profile{ID: "profile_b", UserID: user.ID, Name: "Bravo", TextureModel: "default"}); err != nil {
		t.Fatal(err)
	}

	byID, err := db.GetProfileByID(ctx, "profile_a")
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.ID != "profile_a" || byID.UserID != user.ID || byID.Name != "Alpha" || byID.TextureModel != "slim" ||
		byID.SkinHash == nil || *byID.SkinHash != "skin_hash" || byID.CapeHash == nil || *byID.CapeHash != "cape_hash" {
		t.Fatalf("GetProfileByID returned wrong profile: %#v", byID)
	}
	byName, err := db.GetProfileByName(ctx, "Bravo")
	if err != nil {
		t.Fatal(err)
	}
	if byName == nil || byName.ID != "profile_b" {
		t.Fatalf("GetProfileByName should find profile_b, got %#v", byName)
	}
	if ok, err := db.VerifyProfileOwnership(ctx, user.ID, "profile_a"); err != nil || !ok {
		t.Fatalf("owner should own profile_a: ok=%v err=%v", ok, err)
	}
	if count, err := db.CountProfilesByUser(ctx, user.ID); err != nil || count != 2 {
		t.Fatalf("expected two profiles, count=%d err=%v", count, err)
	}

	firstPage, err := db.ListProfilesByUser(ctx, user.ID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	firstNext := firstPage["next_key"].(map[string]any)
	if len(firstItems) != 1 || firstItems[0]["id"] != "profile_a" || firstPage["has_next"] != true || firstNext["last_id"] != "profile_a" {
		t.Fatalf("unexpected first page: %#v", firstPage)
	}
	secondPage, err := db.ListProfilesByUser(ctx, user.ID, 1, "profile_a")
	if err != nil {
		t.Fatal(err)
	}
	secondItems := secondPage["items"].([]map[string]any)
	if len(secondItems) != 1 || secondItems[0]["id"] != "profile_b" || secondPage["has_next"] != false || secondPage["next_key"] != nil {
		t.Fatalf("unexpected second page: %#v", secondPage)
	}

	if ok, err := db.UpdateProfileName(ctx, "profile_a", "Renamed"); err != nil || !ok {
		t.Fatalf("UpdateProfileName failed: ok=%v err=%v", ok, err)
	}
	if err := db.UpdateProfileSkin(ctx, "profile_a", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileCape(ctx, "profile_a", nil); err != nil {
		t.Fatal(err)
	}
	if err := db.UpdateProfileModel(ctx, "profile_a", "default"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.GetProfileByID(ctx, "profile_a")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "Renamed" || updated.TextureModel != "default" || updated.SkinHash != nil || updated.CapeHash != nil {
		t.Fatalf("profile updates did not persist exactly: %#v", updated)
	}

	if err := db.AddToken(ctx, model.Token{AccessToken: "access_profile_a", ClientToken: "client_profile_a", UserID: user.ID, ProfileID: &updated.ID, CreatedAt: database.NowMS()}); err != nil {
		t.Fatal(err)
	}
	deleted, err := db.DeleteProfileCascade(ctx, "profile_a")
	if err != nil || !deleted {
		t.Fatalf("DeleteProfileCascade failed: deleted=%v err=%v", deleted, err)
	}
	if tok, err := db.GetToken(ctx, "access_profile_a"); err != nil || tok != nil {
		t.Fatalf("profile cascade should delete bound token, token=%#v err=%v", tok, err)
	}
	if missing, err := db.GetProfileByID(ctx, "profile_a"); err != nil || missing != nil {
		t.Fatalf("profile_a should be gone, got %#v err=%v", missing, err)
	}
}

func TestProfileStoreAdminListAndSearchExactFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	first := testutil.CreateUser(t, db, "first-owner@test.com", "Password123", "FirstOwner", false)
	second := testutil.CreateUser(t, db, "second-owner@test.com", "Password123", "SecondOwner", false)
	testutil.CreateProfile(t, db, first.ID, "admin_profile_a", "AlphaAdmin")
	testutil.CreateProfile(t, db, second.ID, "admin_profile_b", "BetaAdmin")

	search, err := db.SearchProfilesByNames(ctx, []string{"AlphaAdmin", "Missing"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(search) != 1 || search[0].ID != "admin_profile_a" || search[0].Name != "AlphaAdmin" {
		t.Fatalf("SearchProfilesByNames returned wrong rows: %#v", search)
	}

	all, err := db.ListAllProfiles(ctx, 1, "", "owner@test.com")
	if err != nil {
		t.Fatal(err)
	}
	items := all["items"].([]map[string]any)
	next := all["next_key"].(map[string]any)
	if len(items) != 1 || items[0]["id"] != "admin_profile_a" || items[0]["user_id"] != first.ID ||
		items[0]["owner_email"] != "first-owner@test.com" || items[0]["owner_display_name"] != "FirstOwner" ||
		all["has_next"] != true || next["last_id"] != "admin_profile_a" {
		t.Fatalf("unexpected first admin profile page: %#v", all)
	}
	rest, err := db.ListAllProfiles(ctx, 1, "admin_profile_a", "owner@test.com")
	if err != nil {
		t.Fatal(err)
	}
	restItems := rest["items"].([]map[string]any)
	if len(restItems) != 1 || restItems[0]["id"] != "admin_profile_b" || restItems[0]["user_id"] != second.ID ||
		restItems[0]["owner_email"] != "second-owner@test.com" || rest["has_next"] != false {
		t.Fatalf("unexpected second admin profile page: %#v", rest)
	}
}
