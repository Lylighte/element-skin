package admin_test

import (
	"bytes"
	"context"
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
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"
)

func TestCarouselRoutesRejectUnsupportedUploadFormat(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "slide.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte("not an image")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	h := admin.New(testutil.TestConfig(), nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/carousel", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Unsupported file format") {
		t.Fatalf("unsupported carousel upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCarouselRoutesUploadAndDeleteExactFileState(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := testutil.NewMemoryRedis()
	if err := cache.SetPublicCarousel(context.Background(), []string{"stale.png"}, 0); err != nil {
		t.Fatal(err)
	}
	h := admin.NewWithRedis(cfg, nil, cache, nil)
	req := carouselUploadRequest(t, "slide.png", carouselPNG(t))
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"filename":"`) || !strings.Contains(rec.Body.String(), `.png"`) {
		t.Fatalf("carousel upload response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	filename := responseStringField(t, rec.Body.String(), "filename")
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, filename)); err != nil {
		t.Fatalf("uploaded carousel file should exist: %v", err)
	}
	if _, err := cache.GetPublicCarousel(context.Background()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("upload should invalidate public carousel cache, got %v", err)
	}
	if err := cache.SetPublicCarousel(context.Background(), []string{filename}, 0); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/carousel/"+filename, nil)
	req.SetPathValue("filename", filename)
	rec = httptest.NewRecorder()
	h.DeleteCarousel(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("carousel delete response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(cfg.CarouselDir, filename)); !os.IsNotExist(err) {
		t.Fatalf("carousel file should be deleted, stat err=%v", err)
	}
	if _, err := cache.GetPublicCarousel(context.Background()); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should invalidate public carousel cache, got %v", err)
	}
}

func TestCarouselRoutesRejectInvalidUploadInputsExactly(t *testing.T) {
	h := admin.New(testutil.TestConfig(), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/admin/carousel", strings.NewReader("not multipart"))
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid multipart form\"}\n" {
		t.Fatalf("bad multipart carousel upload mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	var missingBody bytes.Buffer
	missingWriter := multipart.NewWriter(&missingBody)
	if err := missingWriter.WriteField("note", "no file"); err != nil {
		t.Fatal(err)
	}
	if err := missingWriter.Close(); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/admin/carousel", &missingBody)
	req.Header.Set("Content-Type", missingWriter.FormDataContentType())
	rec = httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"file is required\"}\n" {
		t.Fatalf("missing carousel file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = carouselUploadRequest(t, "huge.png", bytes.Repeat([]byte("x"), 5*1024*1024+1))
	rec = httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"File too large\"}\n" {
		t.Fatalf("oversized carousel file mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCarouselRoutesDeleteInvalidOrMissingFileExactly(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	h := admin.New(cfg, nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/admin/carousel/", nil)
	req.SetPathValue("filename", "")
	rec := httptest.NewRecorder()
	h.DeleteCarousel(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid filename\"}\n" {
		t.Fatalf("delete empty filename mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/admin/carousel/missing.png", nil)
	req.SetPathValue("filename", "missing.png")
	rec = httptest.NewRecorder()
	h.DeleteCarousel(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("delete missing carousel file should be idempotent: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestCarouselRoutesCacheFailureRollsBackFileMutation(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.CarouselDir = t.TempDir()
	cache := &carouselInvalidateFailRedis{Store: testutil.NewMemoryRedis()}
	h := admin.NewWithRedis(cfg, nil, cache, nil)

	req := carouselUploadRequest(t, "slide.png", carouselPNG(t))
	rec := httptest.NewRecorder()
	h.UploadCarousel(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("upload cache failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	entries, err := os.ReadDir(cfg.CarouselDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed upload cache invalidation must remove the new file: entries=%#v err=%v", entries, err)
	}

	const existingName = "existing.png"
	original := []byte("existing carousel bytes")
	path := filepath.Join(cfg.CarouselDir, existingName)
	if err := os.WriteFile(path, original, 0o640); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodDelete, "/admin/carousel/"+existingName, nil)
	req.SetPathValue("filename", existingName)
	rec = httptest.NewRecorder()
	h.DeleteCarousel(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("delete cache failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	restored, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(restored, original) {
		t.Fatalf("failed delete cache invalidation must restore exact file bytes: restored=%q err=%v", restored, err)
	}
}

type carouselInvalidateFailRedis struct {
	redisstore.Store
}

func (r *carouselInvalidateFailRedis) InvalidatePublicCarousel(context.Context) error {
	return errors.New("redis invalidate failed")
}

func carouselUploadRequest(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/carousel", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func carouselPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for x := 0; x < 64; x++ {
		for y := 0; y < 64; y++ {
			img.SetRGBA(x, y, color.RGBA{R: 20, G: 40, B: 80, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func responseStringField(t *testing.T, body, field string) string {
	t.Helper()
	marker := `"` + field + `":"`
	start := strings.Index(body, marker)
	if start < 0 {
		t.Fatalf("missing field %s in %q", field, body)
	}
	start += len(marker)
	end := strings.Index(body[start:], `"`)
	if end < 0 {
		t.Fatalf("unterminated field %s in %q", field, body)
	}
	return body[start : start+end]
}
