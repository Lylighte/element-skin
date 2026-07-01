package admin

import (
	"archive/zip"
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/util"
)

func TestHomepageMediaFormHelpersParseDefaultsAndExactErrors(t *testing.T) {
	req := httptest.NewRequest("POST", "/upload", strings.NewReader(""))
	values, err := homepageMediaValuesFromForm(req, "image")
	if err != nil {
		t.Fatal(err)
	}
	if values.OverlayOpacityLight != 0.45 || values.OverlayOpacityDark != 0.45 ||
		values.StartYaw != 0 || values.StartPitch != 0 || values.YawSpeedDPS != 4 || values.PitchSpeedDPS != 0 {
		t.Fatalf("default image values mismatch: %#v", values)
	}
	if got := intForm(req, "duration_ms", 6000); got != 6000 {
		t.Fatalf("default intForm=%d, want 6000", got)
	}

	body := strings.NewReader("overlay_opacity_light=bad")
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = homepageMediaValuesFromForm(req, "image")
	assertHTTPError(t, err, 400, "overlay_opacity_light must be a number")

	body = strings.NewReader("overlay_opacity_light=0.91")
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = homepageMediaValuesFromForm(req, "image")
	assertHTTPError(t, err, 400, "overlay_opacity_light out of range")

	body = strings.NewReader("start_yaw=abc")
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = homepageMediaValuesFromForm(req, "panorama")
	assertHTTPError(t, err, 400, "start_yaw must be a number")

	body = strings.NewReader("start_yaw=361")
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_, err = homepageMediaValuesFromForm(req, "panorama")
	assertHTTPError(t, err, 400, "start_yaw out of range")

	body = strings.NewReader("duration_ms=not-number")
	req = httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if got := intForm(req, "duration_ms", 6000); got != 6000 {
		t.Fatalf("invalid intForm=%d, want fallback 6000", got)
	}
}

func TestReadPanoramaZipRejectsExactInvalidShapes(t *testing.T) {
	cases := []struct {
		name   string
		data   []byte
		detail string
	}{
		{name: "not zip", data: []byte("not a zip"), detail: "invalid panorama zip"},
		{name: "nested", data: panoramaZipWithFiles(t, map[string][]byte{"nested/panorama_0.png": pngBytesForAdminHelper(t, 4, 4)}), detail: "panorama files must be at zip root"},
		{name: "extra", data: panoramaZipWithFiles(t, map[string][]byte{"panorama_0.png": pngBytesForAdminHelper(t, 4, 4), "other.png": pngBytesForAdminHelper(t, 4, 4)}), detail: "panorama zip must contain only panorama_0.png through panorama_5.png"},
		{name: "invalid face", data: panoramaZipWithFiles(t, panoramaFaces(t, map[string][]byte{"panorama_3.png": []byte("bad png")})), detail: "invalid panorama face image"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			faces, err := readPanoramaZip(tc.data)
			if faces != nil {
				t.Fatalf("invalid panorama should not return faces: %#v", faces)
			}
			assertHTTPError(t, err, 400, tc.detail)
		})
	}

	faces, err := readPanoramaZip(panoramaZipWithFiles(t, panoramaFaces(t, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if len(faces) != 6 || len(faces["panorama_0.png"]) == 0 || len(faces["panorama_5.png"]) == 0 {
		t.Fatalf("valid panorama faces mismatch: keys=%v", faces)
	}
}

func TestValidateHomepageMediaValuesExactBoundaries(t *testing.T) {
	zero := 0.0
	if err := validateOpacity("overlay_opacity_light", nil); err != nil {
		t.Fatal(err)
	}
	if err := validateOpacity("overlay_opacity_light", &zero); err != nil {
		t.Fatal(err)
	}
	negative := -0.01
	assertHTTPError(t, validateOpacity("overlay_opacity_light", &negative), 400, "overlay_opacity_light out of range")

	startPitch := -90.0
	assertHTTPError(t, validatePanoramaValues(nil, &startPitch, nil, nil), 400, "start_pitch out of range")
	yawSpeed := -91.0
	assertHTTPError(t, validatePanoramaValues(nil, nil, &yawSpeed, nil), 400, "yaw_speed_dps out of range")
	pitchSpeed := 91.0
	assertHTTPError(t, validatePanoramaValues(nil, nil, nil, &pitchSpeed), 400, "pitch_speed_dps out of range")
}

func panoramaFaces(t *testing.T, overrides map[string][]byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for i := 0; i < 6; i++ {
		name := "panorama_" + string(rune('0'+i)) + ".png"
		out[name] = pngBytesForAdminHelper(t, 4, 4)
	}
	for name, data := range overrides {
		out[name] = data
	}
	return out
}

func panoramaZipWithFiles(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, data := range files {
		part, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func pngBytesForAdminHelper(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{R: 200, G: 210, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != status || httpErr.Detail != detail {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}
