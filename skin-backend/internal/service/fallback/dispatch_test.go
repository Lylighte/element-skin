package fallback_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/testutil"
)

func TestFallbackParallelReturnsFastSuccess(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(404)
	}))
	defer slow.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"fast":true}`))
	}))
	defer fast.Close()
	if err := db.Settings.Set(ctx, "fallback_strategy", "parallel"); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{
		{Priority: 1, SessionURL: slow.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60, EnableProfile: true, EnableHasJoined: true},
		{Priority: 2, SessionURL: fast.URL, AccountURL: "a", ServicesURL: "s", CacheTTL: 60, EnableProfile: true, EnableHasJoined: true},
	}); err != nil {
		t.Fatal(err)
	}
	resp, err := (newFallback(db, fast.Client())).GetProfile(ctx, "some-uuid", true)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil || string(resp.Body) != `{"fast":true}` {
		t.Fatalf("unexpected fallback profile response: %#v", resp)
	}
}
