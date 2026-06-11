package util

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestDownloadTextureExactSuccessStatusAndSizeLimitsByFile(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/ok":
			return textureResponse(http.StatusOK, "abcde", 5), nil
		case "/too-large-header":
			return textureResponse(http.StatusOK, "abcdef", 6), nil
		case "/too-large-body":
			return textureResponse(http.StatusOK, "abcdef", -1), nil
		default:
			return textureResponse(http.StatusNotFound, "missing", -1), nil
		}
	})}

	data, err := DownloadTexture(client, "https://93.184.216.34/ok", 5)
	if err != nil || string(data) != "abcde" {
		t.Fatalf("DownloadTexture success mismatch: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/missing", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "status 404") {
		t.Fatalf("non-200 should reject: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/too-large-header", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "texture too large") {
		t.Fatalf("large content-length should reject: data=%q err=%v", data, err)
	}
	if data, err := DownloadTexture(client, "https://93.184.216.34/too-large-body", 5); err == nil || string(data) != "" || !strings.Contains(err.Error(), "texture too large") {
		t.Fatalf("large body should reject: data=%q err=%v", data, err)
	}
	if _, err := DownloadTexture(fileFakeClient(200, HardCapBytes+1, []byte("x")), "http://1.1.1.1/huge.png", 0); err == nil {
		t.Fatal("hard cap should apply when maxBytes <= 0")
	}
}

func TestDownloadTextureBlocksRedirectToPrivateAddressBeforeRequest(t *testing.T) {
	privateRequested := false
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Hostname() {
		case "93.184.216.34":
			response := textureResponse(http.StatusFound, "", 0)
			response.Header.Set("Location", "http://127.0.0.1/admin")
			return response, nil
		case "127.0.0.1":
			privateRequested = true
			return textureResponse(http.StatusOK, "secret", 6), nil
		default:
			t.Fatalf("unexpected redirect host: %q", req.URL.Hostname())
			return nil, nil
		}
	})}

	data, err := DownloadTexture(client, "https://93.184.216.34/redirect", 1024)
	if !errors.Is(err, ErrUnsafeURL) || data != nil {
		t.Fatalf("private redirect result: data=%q err=%v, want nil and ErrUnsafeURL", data, err)
	}
	if privateRequested {
		t.Fatal("private redirect target must be rejected before transport receives the request")
	}
}

func TestDownloadTexturePreservesRedirectPolicyAndDependencyErrorsExactly(t *testing.T) {
	redirectErr := errors.New("redirect rejected by caller")
	requests := 0
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if req.URL.String() != "https://1.1.1.1/final" || len(via) != 1 {
				t.Fatalf("redirect callback target=%q via=%d; want public final URL and one prior request", req.URL, len(via))
			}
			return redirectErr
		},
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			requests++
			response := textureResponse(http.StatusFound, "", 0)
			response.Header.Set("Location", "https://1.1.1.1/final")
			return response, nil
		}),
	}
	data, err := DownloadTexture(client, "https://1.1.1.1/start", 1024)
	if data != nil || !errors.Is(err, redirectErr) || requests != 1 {
		t.Fatalf("redirect result data=%q err=%v requests=%d; want nil, caller error, one request", data, err, requests)
	}

	transportErr := errors.New("transport unavailable")
	data, err = DownloadTexture(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, transportErr
	})}, "https://1.1.1.1/texture", 1024)
	if data != nil || !errors.Is(err, transportErr) {
		t.Fatalf("transport result data=%q err=%v; want nil and exact dependency error", data, err)
	}

	readErr := errors.New("response body interrupted")
	data, err = DownloadTexture(&http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    http.StatusOK,
			ContentLength: -1,
			Body:          io.NopCloser(errorReader{err: readErr}),
			Header:        make(http.Header),
		}, nil
	})}, "https://1.1.1.1/texture", 1024)
	if data != nil || !errors.Is(err, readErr) {
		t.Fatalf("read result data=%q err=%v; want nil and exact body error", data, err)
	}
}

func fileFakeClient(status int, contentLength int64, body []byte) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode:    status,
			ContentLength: contentLength,
			Body:          io.NopCloser(bytes.NewReader(body)),
			Header:        make(http.Header),
		}, nil
	})}
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}
