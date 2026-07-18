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
	slowStarted := make(chan struct{})
	releaseSlow := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(slowStarted)
		<-releaseSlow
		w.WriteHeader(404)
	}))
	defer slow.Close()
	defer close(releaseSlow)
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
	type result struct {
		body string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := (newFallback(db, fast.Client())).GetProfile(ctx, "some-uuid", true)
		if resp == nil {
			done <- result{err: err}
			return
		}
		done <- result{body: string(resp.Body), err: err}
	}()

	select {
	case <-slowStarted:
	case <-time.After(time.Second):
		t.Fatal("slow fallback endpoint was not called")
	}
	select {
	case got := <-done:
		if got.err != nil || got.body != `{"fast":true}` {
			t.Fatalf("unexpected fallback profile response: body=%q err=%v", got.body, got.err)
		}
	case <-time.After(time.Second):
		t.Fatal("parallel fallback waited for a slower endpoint after receiving a successful response")
	}
}
