package fallback_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/testutil"
)

func TestFallbackHTTPClientIgnoresNonOKResponses(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{{
		Priority: 1, SessionURL: server.URL, AccountURL: server.URL, ServicesURL: server.URL, CacheTTL: 60,
		EnableProfile: true, EnableHasJoined: true,
	}}); err != nil {
		t.Fatal(err)
	}
	resp, err := (newFallback(db, server.Client())).GetProfile(ctx, "missing", true)
	if err != nil || resp != nil {
		t.Fatalf("non-OK fallback response should be ignored: resp=%#v err=%v", resp, err)
	}
}
