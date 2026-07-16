package fallback

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

type fallbackRoundTripFunc func(*http.Request) (*http.Response, error)

func (f fallbackRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestFallbackProductionClientRejectsPrivateURLBeforeCacheMutationExactly(t *testing.T) {
	store := redisstore.NewMemoryStore()
	store.Err = errors.New("cache should not be reached")
	response, err := (Fallback{Redis: store}).get(context.Background(), map[string]any{"id": 1}, "http://127.0.0.1/private")
	if response != nil || !errors.Is(err, util.ErrUnsafeURL) {
		t.Fatalf("private fallback response=(%#v, %v), want nil and ErrUnsafeURL", response, err)
	}
}

func TestFallbackResponseBodyHasExactHardLimit(t *testing.T) {
	client := &http.Client{Transport: fallbackRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(strings.Repeat("x", maxFallbackResponseBytes+1))),
		}, nil
	})}
	response, err := (Fallback{Client: client}).get(context.Background(), map[string]any{"id": 1}, "https://example.com/profile")
	if response != nil || err == nil || err.Error() != "fallback response too large" {
		t.Fatalf("oversized fallback response=(%#v, %v), want exact size error", response, err)
	}
}
