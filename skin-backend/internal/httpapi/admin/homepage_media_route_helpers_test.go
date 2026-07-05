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
	"testing"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

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
