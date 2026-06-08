package yggdrasil_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi/yggdrasil"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestTextureRoutesRequireBearerAndDeleteClearsProfileSkinExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-texture@test.com", "Password123", "YggTexture", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_texture_profile", "YggTextureProfile")

	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", strings.NewReader(""))
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "Bearer token required") {
		t.Fatalf("upload without bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	profileID := profile.ID
	if err := redis.SetYggToken(context.Background(), model.Token{AccessToken: "delete_texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: time.Now().UnixMilli()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if token, err := db.Tokens.Get(context.Background(), "delete_texture_token"); err != nil || token != nil {
		t.Fatalf("texture route seed token must be redis-only: %#v err=%v", token, err)
	}
	skin := "skin_before_delete"
	if err := db.Profiles.UpdateSkin(req.Context(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/api/user/profile/"+profile.ID+"/skin", nil)
	req.Header.Set("Authorization", "Bearer delete_texture_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec = httptest.NewRecorder()
	h.DeleteTexture(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete texture should return 204 exactly: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(req.Context(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash != nil {
		t.Fatalf("skin hash should be cleared exactly: profile=%#v err=%v", updated, err)
	}
}

func TestTextureRouteUploadUsesRedisYggToken(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.TexturesDir = t.TempDir()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-upload@test.com", "Password123", "YggUpload", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_upload_profile", "YggUploadProfile")
	token := model.Token{AccessToken: "upload_texture_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: time.Now().UnixMilli()}
	if err := redis.SetYggToken(context.Background(), token, time.Minute); err != nil {
		t.Fatal(err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("model", "slim"); err != nil {
		t.Fatal(err)
	}
	part, err := writer.CreateFormFile("file", "skin.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(testPNG(t, 64, 64)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/user/profile/"+profile.ID+"/skin", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer upload_texture_token")
	req.SetPathValue("uuid", profile.ID)
	req.SetPathValue("texture_type", "skin")
	rec := httptest.NewRecorder()
	h.UploadTexture(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hash"`) {
		t.Fatalf("upload texture mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Profiles.GetByID(context.Background(), profile.ID)
	if err != nil || updated == nil || updated.SkinHash == nil || updated.TextureModel != "slim" {
		t.Fatalf("upload should apply skin/model: profile=%#v err=%v", updated, err)
	}
	if dbToken, err := db.Tokens.Get(context.Background(), token.AccessToken); err != nil || dbToken != nil {
		t.Fatalf("upload token must remain redis-only: %#v err=%v", dbToken, err)
	}
}

func testPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.Set(x, y, color.RGBA{R: 255, G: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
