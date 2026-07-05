package site_test

import (
	"bytes"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func textureMultipartRequest(t *testing.T, target string, fields map[string]string, fileField, fileName string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatal(err)
		}
	}
	part, err := writer.CreateFormFile(fileField, fileName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func routePNG(t *testing.T, w, h int) []byte {
	return routePNGWithColor(t, w, h, color.RGBA{R: 80, G: 120, B: 200, A: 255})
}

func routePNGWithColor(t *testing.T, w, h int, c color.RGBA) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func jsonStringField(t *testing.T, body, field string) string {
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

func withUserActorWithoutPermission(req *http.Request, userID string, excludeCode string) *http.Request {
	var perms []permission.Definition
	for _, role := range permission.Roles {
		if role.ID == permission.RoleUser {
			for _, p := range role.Permissions {
				if p.Code != excludeCode {
					perms = append(perms, p)
				}
			}
			break
		}
	}
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, perms...))
}

func ptrString(s string) *string {
	return &s
}
