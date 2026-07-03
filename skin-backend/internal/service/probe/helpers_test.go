package probe

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestProbeHelpersUseExactURLClockRetentionAndStatusRules(t *testing.T) {
	if got := joinURL("", "/path"); got != "" {
		t.Fatalf("empty base joinURL = %q, want empty", got)
	}
	if got := joinURL("https://example.test/api/", "path"); got != "https://example.test/api/path" {
		t.Fatalf("trimmed joinURL mismatch: got %q", got)
	}
	if got := joinURL("https://example.test/api", "/path"); got != "https://example.test/api/path" {
		t.Fatalf("slash-preserving joinURL mismatch: got %q", got)
	}

	svc := &Service{}
	if got := svc.checkURL(context.Background(), ""); got != StatusDown {
		t.Fatalf("empty URL status = %q, want %q", got, StatusDown)
	}
	if got := svc.checkURL(context.Background(), "://bad"); got != StatusDown {
		t.Fatalf("invalid URL status = %q, want %q", got, StatusDown)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch req.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/missing":
			w.WriteHeader(http.StatusNotFound)
		case "/empty":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	defer server.Close()
	if got := svc.checkURL(context.Background(), server.URL+"/ok"); got != StatusUp {
		t.Fatalf("200 status = %q, want %q", got, StatusUp)
	}
	if got := svc.checkURL(context.Background(), server.URL+"/missing"); got != StatusUp {
		t.Fatalf("404 status = %q, want %q", got, StatusUp)
	}
	if got := svc.checkURL(context.Background(), server.URL+"/empty"); got != StatusUp {
		t.Fatalf("204 status = %q, want %q", got, StatusUp)
	}
	if got := svc.checkURL(context.Background(), server.URL+"/teapot"); got != StatusDown {
		t.Fatalf("418 status = %q, want %q", got, StatusDown)
	}

	fixed := time.Unix(123, int64(456*time.Millisecond))
	svc.Now = func() time.Time { return fixed }
	if got := svc.now(); !got.Equal(fixed) {
		t.Fatalf("injected now mismatch: got %s want %s", got, fixed)
	}
	svc.Now = nil
	if got := svc.now(); got.IsZero() {
		t.Fatalf("default now should not be zero")
	}
	if got := svc.retention(); got != redisstore.ProbeHistoryRetention {
		t.Fatalf("default retention = %s, want %s", got, redisstore.ProbeHistoryRetention)
	}
	svc.Retention = 3 * time.Hour
	if got := svc.retention(); got != 3*time.Hour {
		t.Fatalf("custom retention = %s, want 3h", got)
	}
}

func TestReadIntervalUsesDefaultWhenReaderFailsExactly(t *testing.T) {
	got := ReadInterval(context.Background(), errorIntervalReader{})
	if got != defaultInterval {
		t.Fatalf("reader error interval = %s, want %s", got, defaultInterval)
	}
}

type errorIntervalReader struct{}

func (errorIntervalReader) Get(context.Context, string, string) (string, error) {
	return "", errors.New("settings unavailable")
}
