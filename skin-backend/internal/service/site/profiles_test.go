package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, user.ID, "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, user.ID, "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}
}

func TestDeleteUserRecountsSharedLibraryButDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	owner := testutil.CreateUser(t, db, "site-profile-delete-owner@test.com", "Password123", "ProfileDeleteOwner", false)
	target := testutil.CreateUser(t, db, "site-profile-delete-target@test.com", "Password123", "ProfileDeleteTarget", false)
	other := testutil.CreateUser(t, db, "site-profile-delete-other@test.com", "Password123", "ProfileDeleteOther", false)

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, target.ID, "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil {
		t.Fatal(err)
	}

	ok, err := svc.DeleteUser(ctx, target.ID)
	if err != nil || !ok {
		t.Fatalf("DeleteUser returned ok=%v err=%v", ok, err)
	}
	assertServicePublicUsage(t, svc, "delete_user_shared_skin", int64(2))
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}

func assertServicePublicUsage(t *testing.T, svc anyPublicLibrary, hash string, want int64) {
	t.Helper()
	page, err := svc.PublicLibrary(context.Background(), "", 10, "skin", "", "most_used")
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

type anyPublicLibrary interface {
	PublicLibrary(context.Context, string, int, string, string, string) (map[string]any, error)
}
