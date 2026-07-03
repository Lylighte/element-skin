package texture_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestTexturesApplyUpdateAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-textures-service@test.com", "Password123", "SiteTexturesService", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_textures_profile", "SiteTexturesProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_skin", "skin", "Texture Service Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updatedProfile, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updatedProfile.SkinHash == nil || *updatedProfile.SkinHash != "texture_service_skin" || updatedProfile.TextureModel != "slim" {
		t.Fatalf("profile texture state mismatch: profile=%#v err=%v", updatedProfile, err)
	}
	detail, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "texture_service_skin", "skin", map[string]any{"note": "Updated Texture Service", "is_public": false})
	if err != nil || detail["note"] != "Updated Texture Service" || detail["is_public"] != 0 {
		t.Fatalf("UpdateTexture detail mismatch: detail=%#v err=%v", detail, err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(user.ID), "texture_service_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if info, err := db.Textures.GetInfo(ctx, user.ID, "texture_service_skin", "skin"); err != nil || info != nil {
		t.Fatalf("texture should be deleted: info=%#v err=%v", info, err)
	}
}

func TestTextureLibraryWardrobeAndPatchParsingExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-wardrobe-owner@test.com", "Password123", "WardrobeOwner", false)
	collector := testutil.CreateUser(t, db, "site-textures-wardrobe-collector@test.com", "Password123", "WardrobeCollector", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_wardrobe_skin", "skin", "Wardrobe Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(collector.ID), "missing_wardrobe_skin", "skin"); !httpErrorIs(err, 404, "Texture not found in library") {
		t.Fatalf("missing wardrobe add mismatch: %#v", err)
	}
	if count, err := db.Textures.CountForUser(ctx, collector.ID); err != nil || count != 0 {
		t.Fatalf("missing wardrobe add must not create user rows: count=%d err=%v", count, err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(collector.ID), "texture_service_wardrobe_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	updated, err := svc.UpdateTexture(ctx, textureUserActor(collector.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"model": "slim",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["ok"] != true || updated["model"] != "slim" || updated["is_public"] != 2 {
		t.Fatalf("wardrobe model patch mismatch: %#v", updated)
	}
	updated, err = svc.UpdateTexture(ctx, textureUserActor(owner.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"is_public": 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["ok"] != true || updated["model"] != "default" || updated["is_public"] != 1 {
		t.Fatalf("integer public patch mismatch: %#v", updated)
	}
	updated, err = svc.UpdateTexture(ctx, textureUserActor(owner.ID), "texture_service_wardrobe_skin", "skin", map[string]any{
		"is_public": float64(0),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["ok"] != true || updated["model"] != "default" || updated["is_public"] != 0 {
		t.Fatalf("float public patch mismatch: %#v", updated)
	}
	ownerInfo, err := db.Textures.GetInfo(ctx, owner.ID, "texture_service_wardrobe_skin", "skin")
	if err != nil || ownerInfo == nil || ownerInfo["model"] != "default" || ownerInfo["is_public"] != 0 {
		t.Fatalf("wardrobe patch must not mutate owner row: info=%#v err=%v", ownerInfo, err)
	}
}

func TestApplyTextureRejectsMissingForeignAndInvalidTypeWithoutMutatingProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	profileSvc := profilesvc.Service{DB: db}
	owner := testutil.CreateUser(t, db, "site-textures-apply-owner@test.com", "Password123", "ApplyOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-apply-other@test.com", "Password123", "ApplyOther", false)
	profile := testutil.CreateProfile(t, db, owner.ID, "site_apply_profile", "SiteApplyProfile")
	foreign := testutil.CreateProfile(t, db, other.ID, "site_apply_foreign", "SiteApplyForeign")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_apply_skin", "skin", "Texture Apply Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		call func() error
		code int
		want string
	}{
		{"missing texture ownership", func() error {
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), profile.ID, "missing_apply_texture", "skin")
		}, 403, "Texture not found in your library"},
		{"foreign profile", func() error {
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), foreign.ID, "texture_service_apply_skin", "skin")
		}, 403, "Profile not yours"},
		{"invalid type", func() error {
			return svc.ApplyTextureToProfile(ctx, textureUserActor(owner.ID), profile.ID, "texture_service_apply_skin", "elytra")
		}, 403, "Texture not found in your library"},
		{"set invalid type", func() error {
			return profileSvc.SetProfileTexture(ctx, textureActor("texture-service-admin", "profile.update.any"), profile.ID, "elytra", ptrString("texture_service_apply_skin"))
		}, 400, "Invalid texture_type"},
		{"set missing profile", func() error {
			return profileSvc.SetProfileTexture(ctx, textureActor("texture-service-admin", "profile.update.any"), "missing-profile", "skin", ptrString("texture_service_apply_skin"))
		}, 404, "profile not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpErrorIs(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil || unchanged.SkinHash != nil || unchanged.CapeHash != nil || unchanged.TextureModel != profile.TextureModel {
		t.Fatalf("failed apply attempts must not mutate profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestUploaderDeleteRemovesWardrobeCopiesButKeepsAppliedProfileHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-delete-owner@test.com", "Password123", "DeleteOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-delete-other@test.com", "Password123", "DeleteOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_delete_profile", "SiteDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_delete_skin", "skin", "Texture Delete Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(other.ID), profile.ID, "texture_service_delete_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(owner.ID), "texture_service_delete_skin", "skin"); err != nil {
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
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-count-owner@test.com", "Password123", "CountOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-count-other@test.com", "Password123", "CountOther", false)
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_count_skin", "skin", "Texture Count Skin", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteTexture(ctx, textureUserActor(other.ID), "texture_service_count_skin", "skin"); err != nil {
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

func TestDeleteMissingWardrobeTextureReturnsNotFoundAndKeepsAppliedHash(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-textures-missing-owner@test.com", "Password123", "MissingOwner", false)
	other := testutil.CreateUser(t, db, "site-textures-missing-other@test.com", "Password123", "MissingOther", false)
	profile := testutil.CreateProfile(t, db, other.ID, "site_missing_delete_profile", "SiteMissingDeleteProfile")
	if err := db.Textures.AddToLibrary(ctx, owner.ID, "texture_service_missing_delete", "skin", "Missing Delete Texture", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, ptrString("texture_service_missing_delete")); err != nil {
		t.Fatal(err)
	}

	err := svc.DeleteTexture(ctx, textureUserActor(other.ID), "texture_service_missing_delete", "skin")
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 404 || httpErr.Detail != "Texture not found" {
		t.Fatalf("missing wardrobe delete should return exact 404 error, got %#v", err)
	}
	if info, err := db.Textures.GetInfo(ctx, owner.ID, "texture_service_missing_delete", "skin"); err != nil || info == nil {
		t.Fatalf("missing wardrobe delete must keep uploader library row: info=%#v err=%v", info, err)
	}
	afterDelete, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || afterDelete == nil || afterDelete.SkinHash == nil || *afterDelete.SkinHash != "texture_service_missing_delete" {
		t.Fatalf("missing wardrobe delete must not clear applied profile hash: profile=%#v err=%v", afterDelete, err)
	}
}

func TestApplySkinRollsBackHashWhenModelUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-apply-atomic@test.com", "Password123", "ApplyAtomic", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_apply_atomic", "SiteApplyAtomic")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_apply_atomic_skin", "skin", "Atomic Skin", false, "slim"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE profiles ADD CONSTRAINT reject_slim_model CHECK (texture_model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "site_apply_atomic_skin", "skin")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("apply skin failure = %#v, want PostgreSQL 23514", err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || unchanged == nil ||
		unchanged.SkinHash != nil ||
		unchanged.TextureModel != "default" {
		t.Fatalf("failed skin apply must preserve hash and model: profile=%#v err=%v", unchanged, err)
	}
}

func TestUpdateTextureRejectsInvalidFieldsBeforeMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-validate@test.com", "Password123", "TextureValidate", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_validate", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name   string
		body   map[string]any
		detail string
	}{
		{"invalid model", map[string]any{"note": "Changed", "model": "wide"}, "invalid model"},
		{"invalid public", map[string]any{"note": "Changed", "is_public": "yes"}, "invalid is_public"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "site_texture_validate", "skin", test.body)
			if result != nil || !httpErrorIs(err, 400, test.detail) {
				t.Fatalf("invalid update result=%#v err=%#v, want exact 400 %q", result, err, test.detail)
			}
			info, err := db.Textures.GetInfo(ctx, user.ID, "site_texture_validate", "skin")
			if err != nil || info == nil ||
				info["note"] != "Original" ||
				info["model"] != "default" ||
				info["is_public"] != 1 {
				t.Fatalf("invalid update changed texture: info=%#v err=%v", info, err)
			}
		})
	}
}

func TestUpdateTextureRollsBackAllFieldsWhenLibraryModelUpdateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-patch-rollback@test.com", "Password123", "TexturePatchRollback", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "site_texture_patch_rollback", "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx,
		`ALTER TABLE skin_library ADD CONSTRAINT reject_slim_library_model CHECK (model <> 'slim')`,
	); err != nil {
		t.Fatal(err)
	}

	result, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "site_texture_patch_rollback", "skin", map[string]any{
		"note":      "Changed",
		"model":     "slim",
		"is_public": false,
	})
	var pgErr *pgconn.PgError
	if result != nil || !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("atomic user patch result=%#v err=%#v, want nil and PostgreSQL 23514", result, err)
	}
	info, err := db.Textures.GetInfo(ctx, user.ID, "site_texture_patch_rollback", "skin")
	if err != nil || info == nil ||
		info["note"] != "Original" ||
		info["model"] != "default" ||
		info["is_public"] != 1 {
		t.Fatalf("failed user patch changed texture: info=%#v err=%v", info, err)
	}
}

func TestTextureServiceMapsMissingUpdatesAndDetailToExactNotFound(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-missing-update@test.com", "Password123", "TextureMissingUpdate", false)

	for _, tc := range []struct {
		name string
		call func() error
	}{
		{name: "detail", call: func() error {
			_, err := svc.TextureDetail(ctx, textureUserActor(user.ID), "missing_texture", "skin")
			return err
		}},
		{name: "note update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"note": "No row"})
			return err
		}},
		{name: "model update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"model": "slim"})
			return err
		}},
		{name: "visibility update", call: func() error {
			_, err := svc.UpdateTexture(ctx, textureUserActor(user.ID), "missing_texture", "skin", map[string]any{"is_public": true})
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !httpErrorIs(err, 404, "Texture not found") {
				t.Fatalf("%s should map to exact not-found error, got %#v", tc.name, err)
			}
		})
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("missing texture operations must not create rows: count=%d err=%v", count, err)
	}
}

func TestTextureServiceAppliesCapeWithoutChangingSkinOrModel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-texture-cape@test.com", "Password123", "TextureCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_texture_cape_profile", "TextureCapeProfile")
	skin := "existing_skin_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, user.ID, "texture_service_cape", "cape", "Texture Service Cape", true, "slim"); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyTextureToProfile(ctx, textureUserActor(user.ID), profile.ID, "texture_service_cape", "cape"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || *updated.SkinHash != skin ||
		updated.CapeHash == nil || *updated.CapeHash != "texture_service_cape" ||
		updated.TextureModel != profile.TextureModel {
		t.Fatalf("cape apply must change only cape hash: profile=%#v err=%v", updated, err)
	}
}

func TestApplyTextureToProfileWithModel(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "apply-model@test.com", "Password123", "ApplyModel", false)
	profile := testutil.CreateProfile(t, db, user.ID, "apply_model_profile", "ApplyModelProfile")
	if err := db.Textures.AddToLibrary(ctx, user.ID, "apply_model_skin", "skin", "Model Skin", true, "slim"); err != nil {
		t.Fatal(err)
	}

	if err := svc.ApplyTextureToProfileWithModel(ctx, textureUserActor(user.ID), profile.ID, "apply_model_skin", "skin", "default"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil {
		t.Fatalf("profile not found after apply: err=%v", err)
	}
	if updated.SkinHash == nil || *updated.SkinHash != "apply_model_skin" {
		t.Fatalf("skin hash mismatch: %#v", updated.SkinHash)
	}
	if updated.TextureModel != "default" {
		t.Fatalf("model should be 'default' as explicitly passed: %s", updated.TextureModel)
	}
}

func TestTextureLibraryCursorsAndDisabledPublicLibraryExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	if err := db.Textures.AddToLibrary(ctx, user.ID, "profile_cursor_skin", "skin", "Profile Cursor Skin", true, "default"); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.ListMyTextures(ctx, textureUserActor(user.ID), "not-base64", 10, "skin"); !httpErrorIs(err, 400, "Invalid cursor") {
		t.Fatalf("invalid texture cursor should reject exactly, got %#v", err)
	}
	if _, err := svc.PublicLibrary(ctx, "not-base64", 10, "skin", "", "latest"); !httpErrorIs(err, 400, "Invalid cursor") {
		t.Fatalf("invalid public library cursor should reject exactly, got %#v", err)
	}

	if err := db.Settings.Set(ctx, "enable_skin_library", "false"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PublicLibrary(ctx, "", 10, "skin", "", "latest"); !httpErrorIs(err, 403, "Skin library is disabled by administrator") {
		t.Fatalf("disabled public library should reject exactly, got %#v", err)
	}
}

func TestPublicLibraryPropagatesSettingsCacheErrorExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("settings cache unavailable")
	svc := texturesvc.LibraryService{DB: db, Settings: settingssvc.Settings{DB: db, Redis: cache}}

	result, err := svc.PublicLibrary(ctx, "", 10, "skin", "", "latest")
	if result != nil || !errors.Is(err, cache.Err) {
		t.Fatalf("PublicLibrary settings dependency error result=%#v err=%v want nil %v", result, err, cache.Err)
	}
}

func TestTextureListsAndPublicLibraryCursorsAdvanceExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-profile-cursor-owner@test.com", "Password123", "ProfileCursorOwner", false)
	other := testutil.CreateUser(t, db, "site-profile-cursor-other@test.com", "Password123", "ProfileCursorOther", false)
	for _, item := range []struct {
		hash string
		name string
	}{
		{"profile_cursor_old", "Profile Cursor Old"},
		{"profile_cursor_new", "Profile Cursor New"},
	} {
		if err := db.Textures.AddToLibrary(ctx, owner.ID, item.hash, "skin", item.name, true, "default"); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "profile_cursor_old", "skin"); err != nil {
		t.Fatal(err)
	}

	firstPage, err := svc.ListMyTextures(ctx, textureUserActor(owner.ID), "", 1, "skin")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	cursor, _ := firstPage["next_cursor"].(string)
	if len(firstItems) != 1 || cursor == "" {
		t.Fatalf("ListMyTextures first page should include one item and next cursor: %#v", firstPage)
	}
	secondPage, err := svc.ListMyTextures(ctx, textureUserActor(owner.ID), cursor, 10, "skin")
	if err != nil {
		t.Fatal(err)
	}
	if secondItems := secondPage["items"].([]map[string]any); len(secondItems) != 1 || secondItems[0]["hash"] == firstItems[0]["hash"] {
		t.Fatalf("ListMyTextures cursor should advance to next item: first=%#v second=%#v", firstPage, secondPage)
	}

	public, err := svc.PublicLibrary(ctx, "", 1, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	publicItems := public["items"].([]map[string]any)
	publicCursor, _ := public["next_cursor"].(string)
	if len(publicItems) != 1 || publicCursor == "" || publicItems[0]["usage_count"] != int64(2) {
		t.Fatalf("most_used public library first page mismatch: %#v", public)
	}
	nextPublic, err := svc.PublicLibrary(ctx, publicCursor, 10, "skin", "Profile Cursor", "most_used")
	if err != nil {
		t.Fatal(err)
	}
	nextItems := nextPublic["items"].([]map[string]any)
	if len(nextItems) != 1 || nextItems[0]["hash"] == publicItems[0]["hash"] {
		t.Fatalf("most_used public library cursor should advance exactly: first=%#v next=%#v", public, nextPublic)
	}
}

func TestDeleteUserRecountsSharedLibraryButDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	owner := testutil.CreateUser(t, db, "site-profile-delete-owner@test.com", "Password123", "ProfileDeleteOwner", false)
	target := testutil.CreateUser(t, db, "site-profile-delete-target@test.com", "Password123", "ProfileDeleteTarget", false)
	other := testutil.CreateUser(t, db, "site-profile-delete-other@test.com", "Password123", "ProfileDeleteOther", false)

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(target.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "delete_user_shared_skin", "skin"); err != nil {
		t.Fatal(err)
	}
	if err := svc.AddTextureToWardrobe(ctx, textureUserActor(other.ID), "delete_user_uploaded_skin", "skin"); err != nil {
		t.Fatal(err)
	}

	accountSvc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	if err := accountSvc.DeleteUser(ctx, textureActor("admin-delete-user", "account.delete.any"), target.ID); err != nil {
		t.Fatalf("DeleteUser returned err=%v", err)
	}
	assertServicePublicUsage(t, svc, "delete_user_shared_skin", int64(2))
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}

func TestPublicLibraryRejectsIncompleteAndCrossSortCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)

	for _, tc := range []struct {
		name   string
		cursor string
		sort   string
	}{
		{
			name: "latest cursor missing hash",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": int64(1234),
			}),
			sort: "latest",
		},
		{
			name: "latest cursor reused for most used",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": int64(1234),
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "most_used",
		},
		{
			name: "fractional timestamp",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": 1.5,
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "latest",
		},
		{
			name: "negative timestamp",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at": -1,
				"last_skin_hash":  "cursor_hash",
			}),
			sort: "latest",
		},
		{
			name: "fractional usage",
			cursor: util.EncodeCursor(map[string]any{
				"last_created_at":  1,
				"last_skin_hash":   "cursor_hash",
				"last_usage_count": 2.5,
			}),
			sort: "most_used",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := svc.PublicLibrary(ctx, tc.cursor, 10, "skin", "", tc.sort)
			if result != nil || !httpErrorIs(err, 400, "Invalid cursor") {
				t.Fatalf("PublicLibrary result=%#v err=%#v; want nil and exact invalid cursor", result, err)
			}
		})
	}
}

func TestPrivateTextureListsRejectIncompleteCursors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	user := testutil.CreateUser(t, db, "private-cursor@test.com", "Password123", "PrivateCursor", false)

	textureResult, err := svc.ListMyTextures(ctx, textureUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": int64(1234),
	}), 10, "skin")
	if textureResult != nil || !httpErrorIs(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyTextures result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}

	textureResult, err = svc.ListMyTextures(ctx, textureUserActor(user.ID), util.EncodeCursor(map[string]any{
		"last_created_at": 1.5,
		"last_hash":       "cursor_hash",
	}), 10, "skin")
	if textureResult != nil || !httpErrorIs(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyTextures fractional cursor result=%#v err=%#v; want nil and exact invalid cursor", textureResult, err)
	}
}

func TestTextureServiceClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newLibraryService(db)
	actor := textureUserActor("closed-texture-user")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "apply texture", call: func() error {
			return svc.ApplyTextureToProfile(ctx, actor, "closed-profile", "closed-hash", "skin")
		}},
		{name: "apply texture with model", call: func() error {
			return svc.ApplyTextureToProfileWithModel(ctx, actor, "closed-profile", "closed-hash", "skin", "slim")
		}},
		{name: "texture detail", call: func() error {
			result, err := svc.TextureDetail(ctx, actor, "closed-hash", "skin")
			if result != nil {
				t.Fatalf("TextureDetail closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update texture note", call: func() error {
			result, err := svc.UpdateTexture(ctx, actor, "closed-hash", "skin", map[string]any{"note": "Closed"})
			if result != nil {
				t.Fatalf("UpdateTexture closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update texture empty body", call: func() error {
			result, err := svc.UpdateTexture(ctx, actor, "closed-hash", "skin", map[string]any{})
			if result != nil {
				t.Fatalf("UpdateTexture empty closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "delete texture", call: func() error {
			return svc.DeleteTexture(ctx, actor, "closed-hash", "skin")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}

func ptrString(s string) *string {
	return &s
}

func assertServicePublicUsage(t *testing.T, svc texturesvc.LibraryService, hash string, want int64) {
	t.Helper()
	public, err := svc.PublicLibrary(context.Background(), "", 10, "skin", hash, "most_used")
	if err != nil {
		t.Fatalf("PublicLibrary(%s): %v", hash, err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != hash || items[0]["usage_count"] != want {
		t.Fatalf("PublicLibrary usage for %s = %#v; want usage_count=%d", hash, public, want)
	}
}

func newLibraryService(db *database.DB) texturesvc.LibraryService {
	redis := testutil.NewMemoryRedis()
	return texturesvc.LibraryService{DB: db, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}

func textureUserActor(userID string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, role := range permission.Roles {
		if role.ID != permission.RoleUser {
			continue
		}
		for _, def := range role.Permissions {
			bits.Set(def.BitIndex)
		}
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}
