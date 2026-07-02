package texture_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestUploadServiceUploadToLibraryPersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-service@test.com", "Password123", "TextureUploadService", false)
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}

	res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.update_visibility.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "",
		Note:        "Service Upload",
		IsPublic:    true,
		Model:       "slim",
	})
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := res["hash"].(string)
	if hash == "" || res["texture_type"] != "skin" {
		t.Fatalf("upload response mismatch: %#v", res)
	}
	if _, err := os.Stat(filepath.Join(dir, hash+".png")); err != nil {
		t.Fatalf("uploaded file missing: %v", err)
	}
	info, err := db.Textures.GetInfo(ctx, user.ID, hash, "skin")
	if err != nil || info == nil || info["note"] != "Service Upload" || info["model"] != "slim" || info["is_public"] != 1 {
		t.Fatalf("uploaded library row mismatch: info=%#v err=%v", info, err)
	}
}

func TestUploadServiceRejectsPermissionsBeforeSideEffects(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-perm@test.com", "Password123", "TextureUploadPerm", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_perm", "TextureUploadPerm")
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}
	data := pngBytes(t, 64, 64, testColor())

	if res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID),
		Data:        data,
		TextureType: "skin",
	}); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("upload without create permission result=%#v err=%#v; want exact 403", res, err)
	}
	if res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned"),
		Data:        data,
		TextureType: "skin",
	}, profile.ID); res != nil || !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("upload apply without apply permission result=%#v err=%#v; want exact 403", res, err)
	}
	if count, err := db.Textures.CountForUser(ctx, user.ID); err != nil || count != 0 {
		t.Fatalf("permission failures must not insert texture rows: count=%d err=%v", count, err)
	}
	if entries, err := os.ReadDir(dir); err != nil || len(entries) != 0 {
		t.Fatalf("permission failures must not create texture files: entries=%#v err=%v", entries, err)
	}
}

func TestUploadServiceCleansNewFileWhenDatabaseInsertFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-db-fail@test.com", "Password123", "TextureUploadDBFail", false)
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE user_textures ADD CONSTRAINT reject_service_upload CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	svc := texturesvc.UploadService{DB: db, TexturesDir: dir}

	res, err := svc.UploadToLibrary(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "skin",
	})
	if res != nil || err == nil {
		t.Fatalf("database failure result=%#v err=%#v; want nil and error", res, err)
	}
	if count, countErr := db.Textures.CountForUser(ctx, user.ID); countErr != nil || count != 0 {
		t.Fatalf("failed insert must leave no texture rows: count=%d err=%v", count, countErr)
	}
	if entries, readErr := os.ReadDir(dir); readErr != nil || len(entries) != 0 {
		t.Fatalf("failed insert must delete newly created file: entries=%#v err=%v", entries, readErr)
	}
}

func TestUploadServiceUploadAndApplyUpdatesProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "texture-upload-apply@test.com", "Password123", "TextureUploadApply", false)
	profile := testutil.CreateProfile(t, db, user.ID, "texture_upload_apply", "TextureUploadApply")
	svc := texturesvc.UploadService{DB: db, TexturesDir: t.TempDir()}

	res, err := svc.UploadAndApply(ctx, texturesvc.UploadInput{
		Actor:       textureActor(user.ID, "texture.create.owned", "texture.apply.owned"),
		Data:        pngBytes(t, 64, 64, testColor()),
		TextureType: "skin",
		Model:       "slim",
	}, profile.ID)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := res["hash"].(string)
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if hash == "" || res["ok"] != true || res["type"] != "skin" ||
		err != nil || updated == nil || updated.SkinHash == nil ||
		*updated.SkinHash != hash || updated.TextureModel != "slim" {
		t.Fatalf("upload apply mismatch: res=%#v profile=%#v err=%v", res, updated, err)
	}
}

func textureActor(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   permissiondb.SubjectIDForUser(userID),
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func testColor() color.RGBA {
	return color.RGBA{R: 80, G: 120, B: 200, A: 255}
}

func pngBytes(t *testing.T, width, height int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := range width {
		for y := range height {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}
