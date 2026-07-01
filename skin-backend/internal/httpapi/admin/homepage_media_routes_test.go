package admin_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/admin"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestListHomepageMedia(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	t.Run("empty list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/homepage-media", nil)
		req = withAdminActor(req, "admin-test-user")
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("empty list status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "[]\n" {
			t.Fatalf("empty list must be [], got %q", rec.Body.String())
		}
	})

	t.Run("permission denied", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/admin/homepage-media", nil)
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("permission denied status=%d body=%q", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
			t.Fatalf("permission denied body mismatch: %q", rec.Body.String())
		}
	})
}

func TestHomepageMediaUploadFailsWhenRedisInvalidateFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, &homepageInvalidateFailRedis{Store: testutil.NewMemoryRedis()}, nil)

	req := multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64))
	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis fail upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("redis fail upload body mismatch: %q", rec.Body.String())
	}
	// Verify DB record was cleaned up after Redis failure.
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no items after Redis invalidate failure, got %d", len(items))
	}
	// Verify file was cleaned up.
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no files after Redis invalidate failure, got %d", len(entries))
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("redis fail panorama upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err = db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("expected no panorama items after Redis invalidate failure, got %#v", items)
	}
	entries, err = os.ReadDir(cfg.CarouselDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected no panorama files after Redis invalidate failure, got %d", len(entries))
	}
}

func TestHomepageMediaUploadDatabaseFailureCleansFilesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	if _, err := db.Pool.Exec(context.Background(), `ALTER TABLE homepage_media ADD CONSTRAINT reject_homepage_media_insert CHECK (FALSE)`); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("image database failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil || len(items) != 0 {
		t.Fatalf("failed image upload must leave no rows: items=%#v err=%v", items, err)
	}
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed image upload must remove file: entries=%#v err=%v", entries, err)
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("panorama database failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err = db.HomepageMedia.List(context.Background(), false)
	if err != nil || len(items) != 0 {
		t.Fatalf("failed panorama upload must leave no rows: items=%#v err=%v", items, err)
	}
	entries, err = os.ReadDir(cfg.CarouselDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed panorama upload must remove directory: entries=%#v err=%v", entries, err)
	}
}

func TestHomepageMediaImageUploadPatchReorderDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	if err := cache.SetPublicHomepageMedia(context.Background(), []model.HomepageMedia{{ID: "stale"}}, 0); err != nil {
		t.Fatal(err)
	}
	h := admin.NewWithRedis(cfg, db, cache, nil)

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 64, 64)))
	if rec.Code != http.StatusOK {
		t.Fatalf("image upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "image" || item.DurationMS != 6000 || item.SortOrder != 0 || !item.Enabled || item.StoragePath != item.ID+".png" {
		t.Fatalf("image upload item mismatch: %#v", item)
	}
	if item.OverlayOpacityLight != 0.45 || item.OverlayOpacityDark != 0.45 {
		t.Fatalf("image default overlay opacity mismatch: %#v", item)
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.StoragePath)); err != nil {
		t.Fatalf("uploaded image should exist: %v", err)
	}
	if _, err := cache.GetPublicHomepageMedia(context.Background()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("image upload must invalidate public homepage media cache, got %v", err)
	}

	body := strings.NewReader(`{"title":"Hero","enabled":false,"duration_ms":7000,"overlay_opacity_light":0.38,"overlay_opacity_dark":0.62}`)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/"+item.ID, body)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch image status=%d body=%q", rec.Code, rec.Body.String())
	}
	patched := decodeMedia(t, rec.Body.Bytes())
	if patched.Title != "Hero" || patched.Enabled || patched.DurationMS != 7000 {
		t.Fatalf("patched image mismatch: %#v", patched)
	}
	if patched.OverlayOpacityLight != 0.38 || patched.OverlayOpacityDark != 0.62 {
		t.Fatalf("patched image overlay opacity mismatch: %#v", patched)
	}

	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "second.png", pngBytes(t, 32, 32)))
	if rec.Code != http.StatusOK {
		t.Fatalf("second upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	second := decodeMedia(t, rec.Body.Bytes())
	reorderBody := strings.NewReader(`{"ids":["` + second.ID + `","` + item.ID + `"]}`)
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", reorderBody)
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	h.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reorder status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[0].SortOrder != 0 || items[1].ID != item.ID || items[1].SortOrder != 1 {
		t.Fatalf("reorder did not persist exact order: %#v", items)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/"+item.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.StoragePath)); !os.IsNotExist(err) {
		t.Fatalf("deleted image file should be gone, stat err=%v", err)
	}
}

func TestHomepageMediaUploadAcceptsWebPBytesAndRejectsMalformedMultipartExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/image", strings.NewReader("not multipart"))
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("image malformed multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", strings.NewReader("not multipart"))
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("panorama malformed multipart mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = multipartUploadRequest(t, "/v1/admin/homepage-media/image", "not_file", "slide.png", pngBytes(t, 8, 8))
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"file is required\"}\n" {
		t.Fatalf("image missing file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "not_file", "panorama.zip", standardPanoramaZip(t))
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"file is required\"}\n" {
		t.Fatalf("panorama missing file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.webp", []byte("webp bytes are stored without image decoding"))
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("webp upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "image" || !strings.HasSuffix(item.StoragePath, ".webp") || item.Title != "slide.webp" {
		t.Fatalf("webp homepage media mismatch: %#v", item)
	}
	got, err := os.ReadFile(filepath.Join(cfg.CarouselDir, item.StoragePath))
	if err != nil || string(got) != "webp bytes are stored without image decoding" {
		t.Fatalf("webp upload should persist exact bytes: got=%q err=%v", got, err)
	}
}

func TestHomepageMediaMutationsRejectInvalidRequestsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	for _, tc := range []struct {
		name string
		run  func(*httptest.ResponseRecorder)
	}{
		{name: "upload image", run: func(rec *httptest.ResponseRecorder) {
			req := multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8))
			req = httptest.NewRequest(req.Method, req.URL.String(), req.Body)
			h.UploadHomepageImage(rec, req)
		}},
		{name: "upload panorama", run: func(rec *httptest.ResponseRecorder) {
			req := multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t))
			req = httptest.NewRequest(req.Method, req.URL.String(), req.Body)
			h.UploadHomepagePanorama(rec, req)
		}},
		{name: "patch", run: func(rec *httptest.ResponseRecorder) {
			h.PatchHomepageMedia(rec, httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{}`)))
		}},
		{name: "reorder", run: func(rec *httptest.ResponseRecorder) {
			h.ReorderHomepageMedia(rec, httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", strings.NewReader(`{"ids":[]}`)))
		}},
		{name: "delete", run: func(rec *httptest.ResponseRecorder) {
			h.DeleteHomepageMedia(rec, httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/missing", nil))
		}},
	} {
		t.Run("permission denied "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.run(rec)
			if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
				t.Fatalf("%s permission mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.gif", []byte("gif")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Unsupported file format\"}\n" {
		t.Fatalf("unsupported image mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", []byte("not png")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid image\"}\n" {
		t.Fatalf("invalid image mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := multipartUploadRequestWithFields(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8), map[string]string{"overlay_opacity_light": "bad"})
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"overlay_opacity_light must be a number\"}\n" {
		t.Fatalf("image form opacity parse mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = multipartUploadRequestWithFields(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8), map[string]string{"duration_ms": "999"})
	h.UploadHomepageImage(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"duration_ms out of range\"}\n" {
		t.Fatalf("image duration range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{bad`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json body\"}\n" {
		t.Fatalf("patch invalid json mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{"duration_ms":999}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"duration_ms out of range\"}\n" {
		t.Fatalf("patch duration mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{"overlay_opacity_dark":0.91}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"overlay_opacity_dark out of range\"}\n" {
		t.Fatalf("patch opacity mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{"title":"x"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
		t.Fatalf("patch missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "invalid json", body: `{bad`},
		{name: "empty ids", body: `{"ids":[]}`},
		{name: "duplicate ids", body: `{"ids":["a","a"]}`},
		{name: "blank id", body: `{"ids":[""]}`},
	} {
		t.Run("reorder "+tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", strings.NewReader(tc.body))
			req = withAdminActor(req, "admin-test-user")
			h.ReorderHomepageMedia(rec, req)
			wantBody := "{\"detail\":\"ids must be unique non-empty strings\"}\n"
			if tc.name == "invalid json" {
				wantBody = "{\"detail\":\"invalid json body\"}\n"
			}
			if tc.name == "empty ids" {
				wantBody = "{\"detail\":\"ids is required\"}\n"
			}
			if rec.Code != http.StatusBadRequest || rec.Body.String() != wantBody {
				t.Fatalf("reorder %s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", strings.NewReader(`{"ids":["missing"]}`))
	req = withAdminActor(req, "admin-test-user")
	h.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
		t.Fatalf("reorder missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/missing", nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", "missing")
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
		t.Fatalf("delete missing mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestHomepageMediaRoutesReturnExactErrorsForClosedDatabaseAndFilesystemFailures(t *testing.T) {
	t.Run("closed database", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cfg.CarouselDir = t.TempDir()
		h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
		db.Close()

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/homepage-media", nil)
		req = withAdminActor(req, "admin-test-user")
		rec := httptest.NewRecorder()
		h.ListHomepageMedia(rec, req)
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("list closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		rec = httptest.NewRecorder()
		h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("image upload closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if entries, err := os.ReadDir(cfg.CarouselDir); err != nil || len(entries) != 0 {
			t.Fatalf("image upload closed database should leave no files: entries=%d err=%v", len(entries), err)
		}

		rec = httptest.NewRecorder()
		h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("panorama upload closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		if entries, err := os.ReadDir(cfg.CarouselDir); err != nil || len(entries) != 0 {
			t.Fatalf("panorama upload closed database should leave no files: entries=%d err=%v", len(entries), err)
		}

		req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/missing", strings.NewReader(`{"title":"x"}`))
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("id", "missing")
		rec = httptest.NewRecorder()
		h.PatchHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
			t.Fatalf("patch closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", strings.NewReader(`{"ids":["missing"]}`))
		req = withAdminActor(req, "admin-test-user")
		rec = httptest.NewRecorder()
		h.ReorderHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
			t.Fatalf("reorder closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/missing", nil)
		req = withAdminActor(req, "admin-test-user")
		req.SetPathValue("id", "missing")
		rec = httptest.NewRecorder()
		h.DeleteHomepageMedia(rec, req)
		if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"homepage media not found\"}\n" {
			t.Fatalf("delete closed database mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
	})

	t.Run("carousel dir is existing file", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		cfg := testutil.TestConfig()
		cfg.CarouselDir = filepath.Join(t.TempDir(), "not-a-dir")
		if err := os.WriteFile(cfg.CarouselDir, []byte("blocks mkdir"), 0o644); err != nil {
			t.Fatal(err)
		}
		h := admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)

		rec := httptest.NewRecorder()
		h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "slide.png", pngBytes(t, 8, 8)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("image mkdir failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		items, err := db.HomepageMedia.List(context.Background(), false)
		if err != nil || len(items) != 0 {
			t.Fatalf("image mkdir failure should leave no rows: items=%#v err=%v", items, err)
		}

		rec = httptest.NewRecorder()
		h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
		if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
			t.Fatalf("panorama mkdir failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
		}
		items, err = db.HomepageMedia.List(context.Background(), false)
		if err != nil || len(items) != 0 {
			t.Fatalf("panorama mkdir failure should leave no rows: items=%#v err=%v", items, err)
		}
	})
}

func TestHomepageMediaMutationRedisFailuresKeepExactPersistedSideEffects(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	healthy := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, healthy, nil)

	rec := httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "first.png", pngBytes(t, 8, 8)))
	if rec.Code != http.StatusOK {
		t.Fatalf("first upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	first := decodeMedia(t, rec.Body.Bytes())
	rec = httptest.NewRecorder()
	h.UploadHomepageImage(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/image", "file", "second.png", pngBytes(t, 8, 8)))
	if rec.Code != http.StatusOK {
		t.Fatalf("second upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	second := decodeMedia(t, rec.Body.Bytes())

	failing := admin.NewWithRedis(cfg, db, &homepageInvalidateFailRedis{Store: testutil.NewMemoryRedis()}, nil)
	req := httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/"+first.ID, strings.NewReader(`{"title":"Patched before cache failure"}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", first.ID)
	rec = httptest.NewRecorder()
	failing.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("patch invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	row, err := db.HomepageMedia.Get(context.Background(), first.ID)
	if err != nil || row.Title != "Patched before cache failure" {
		t.Fatalf("patch should persist before invalidate failure: row=%#v err=%v", row, err)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/reorder", strings.NewReader(`{"ids":["`+second.ID+`","`+first.ID+`"]}`))
	req = withAdminActor(req, "admin-test-user")
	rec = httptest.NewRecorder()
	failing.ReorderHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("reorder invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	items, err := db.HomepageMedia.List(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].ID != second.ID || items[0].SortOrder != 0 || items[1].ID != first.ID || items[1].SortOrder != 1 {
		t.Fatalf("reorder should persist before invalidate failure: %#v", items)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/"+first.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", first.ID)
	rec = httptest.NewRecorder()
	failing.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("delete invalidate failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := db.HomepageMedia.Get(context.Background(), first.ID); err == nil {
		t.Fatal("delete should remove DB row before invalidate failure")
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, first.StoragePath)); !os.IsNotExist(err) {
		t.Fatalf("delete should remove file before invalidate failure, stat err=%v", err)
	}
}

func TestHomepageMediaPanoramaUploadUsesGeneratedStandardZipAndYawPitchFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	h := admin.NewWithRedis(cfg, db, cache, nil)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"overlay_opacity_light": "0.25", "overlay_opacity_dark": "0.55", "start_yaw": "-45", "start_pitch": "5", "yaw_speed_dps": "6", "pitch_speed_dps": "-1.5", "duration_ms": "11000",
	} {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("panorama upload status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	if item.Type != "panorama" || item.DurationMS != 11000 || item.StoragePath != item.ID {
		t.Fatalf("panorama item mismatch: %#v", item)
	}
	if item.OverlayOpacityLight != 0.25 || item.OverlayOpacityDark != 0.55 || item.StartYaw != -45 || item.StartPitch != 5 || item.YawSpeedDPS != 6 || item.PitchSpeedDPS != -1.5 {
		t.Fatalf("panorama fields mismatch: %#v", item)
	}
	for _, name := range []string{
		"panorama_0.png",
		"panorama_1.png",
		"panorama_2.png",
		"panorama_3.png",
		"panorama_4.png",
		"panorama_5.png",
	} {
		if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.ID, name)); err != nil {
			t.Fatalf("panorama face %s should exist: %v", name, err)
		}
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/"+item.ID, strings.NewReader(`{"start_yaw":30,"start_pitch":-10,"yaw_speed_dps":-3.5,"pitch_speed_dps":2.25}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch panorama status=%d body=%q", rec.Code, rec.Body.String())
	}
	patched := decodeMedia(t, rec.Body.Bytes())
	if patched.StartYaw != 30 || patched.StartPitch != -10 || patched.YawSpeedDPS != -3.5 || patched.PitchSpeedDPS != 2.25 {
		t.Fatalf("patched panorama fields mismatch: %#v", patched)
	}

	req = httptest.NewRequest(http.MethodDelete, "/v1/admin/homepage-media/"+item.ID, nil)
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.DeleteHomepageMedia(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete panorama status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, item.ID)); !os.IsNotExist(err) {
		t.Fatalf("deleted panorama directory should be gone, stat err=%v", err)
	}
}

func TestHomepageMediaRejectsInvalidPanoramaInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	h := admin.NewWithRedis(testutil.TestConfig(), db, testutil.NewMemoryRedis(), nil)

	rec := httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "bad.txt", []byte("x")))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"Unsupported file format\"}\n" {
		t.Fatalf("bad extension mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "bad.zip", invalidPanoramaZip(t)))
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"missing panorama_5.png\"}\n" {
		t.Fatalf("missing face mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("start_pitch", "91"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"start_pitch out of range\"}\n" {
		t.Fatalf("pitch range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("yaw_speed_dps", "91"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"yaw_speed_dps out of range\"}\n" {
		t.Fatalf("yaw speed range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	body.Reset()
	writer = multipart.NewWriter(&body)
	part, err = writer.CreateFormFile("file", "panorama.zip")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(standardPanoramaZip(t)); err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteField("overlay_opacity_dark", "1"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/admin/homepage-media/panorama", &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"overlay_opacity_dark out of range\"}\n" {
		t.Fatalf("overlay range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		name   string
		fields map[string]string
		body   string
	}{
		{name: "overlay opacity dark parse", fields: map[string]string{"overlay_opacity_dark": "bad"}, body: "{\"detail\":\"overlay_opacity_dark must be a number\"}\n"},
		{name: "start yaw parse", fields: map[string]string{"start_yaw": "bad"}, body: "{\"detail\":\"start_yaw must be a number\"}\n"},
		{name: "start pitch parse", fields: map[string]string{"start_pitch": "bad"}, body: "{\"detail\":\"start_pitch must be a number\"}\n"},
		{name: "yaw speed parse", fields: map[string]string{"yaw_speed_dps": "bad"}, body: "{\"detail\":\"yaw_speed_dps must be a number\"}\n"},
		{name: "pitch speed parse", fields: map[string]string{"pitch_speed_dps": "bad"}, body: "{\"detail\":\"pitch_speed_dps must be a number\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := multipartUploadRequestWithFields(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t), tc.fields)
			h.UploadHomepagePanorama(rec, req)
			if rec.Code != http.StatusBadRequest || rec.Body.String() != tc.body {
				t.Fatalf("%s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	for _, tc := range []struct {
		name string
		data []byte
		body string
	}{
		{name: "not zip", data: []byte("not a zip archive"), body: "{\"detail\":\"invalid panorama zip\"}\n"},
		{name: "nested file", data: panoramaZipWithEntries(t, map[string][]byte{"nested/panorama_0.png": pngBytes(t, 8, 8)}), body: "{\"detail\":\"panorama files must be at zip root\"}\n"},
		{name: "unexpected file", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": pngBytes(t, 8, 8), "extra.png": pngBytes(t, 8, 8)}), body: "{\"detail\":\"panorama zip must contain only panorama_0.png through panorama_5.png\"}\n"},
		{name: "oversized face", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": bytes.Repeat([]byte{'x'}, maxHomepageFaceBytesForTest()+1)}), body: "{\"detail\":\"panorama face too large\"}\n"},
		{name: "invalid face image", data: panoramaZipWithEntries(t, map[string][]byte{"panorama_0.png": []byte("not png")}), body: "{\"detail\":\"invalid panorama face image\"}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", tc.data))
			if rec.Code != http.StatusBadRequest || rec.Body.String() != tc.body {
				t.Fatalf("%s mismatch: status=%d body=%q", tc.name, rec.Code, rec.Body.String())
			}
		})
	}

	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h = admin.NewWithRedis(cfg, db, testutil.NewMemoryRedis(), nil)
	rec = httptest.NewRecorder()
	h.UploadHomepagePanorama(rec, multipartUploadRequest(t, "/v1/admin/homepage-media/panorama", "file", "panorama.zip", standardPanoramaZip(t)))
	if rec.Code != http.StatusOK {
		t.Fatalf("panorama setup status=%d body=%q", rec.Code, rec.Body.String())
	}
	item := decodeMedia(t, rec.Body.Bytes())
	req = httptest.NewRequest(http.MethodPatch, "/v1/admin/homepage-media/"+item.ID, strings.NewReader(`{"start_yaw":361}`))
	req = withAdminActor(req, "admin-test-user")
	req.SetPathValue("id", item.ID)
	rec = httptest.NewRecorder()
	h.PatchHomepageMedia(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"start_yaw out of range\"}\n" {
		t.Fatalf("patch panorama yaw range mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

type homepageInvalidateFailRedis struct {
	redisstore.Store
}

func (r *homepageInvalidateFailRedis) InvalidatePublicHomepageMedia(context.Context) error {
	return errors.New("redis invalidate failed")
}

func multipartUploadRequest(t *testing.T, path, field, filename string, data []byte) *http.Request {
	return multipartUploadRequestWithFields(t, path, field, filename, data, nil)
}

func multipartUploadRequestWithFields(t *testing.T, path, field, filename string, data []byte, fields map[string]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req = withAdminActor(req, "admin-test-user")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func pngBytes(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.SetRGBA(x, y, color.RGBA{R: 240, G: 240, B: 240, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func standardPanoramaZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 6; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(pngBytes(t, 16, 16)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func invalidPanoramaZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 5; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(pngBytes(t, 16, 16)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func panoramaZipWithEntries(t *testing.T, entries map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func maxHomepageFaceBytesForTest() int {
	return 5 * 1024 * 1024
}

func decodeMedia(t *testing.T, raw []byte) model.HomepageMedia {
	t.Helper()
	var item model.HomepageMedia
	if err := json.Unmarshal(raw, &item); err != nil {
		t.Fatalf("decode media %q: %v", raw, err)
	}
	return item
}
