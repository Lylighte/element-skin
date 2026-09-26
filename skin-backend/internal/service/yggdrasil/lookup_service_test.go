package yggdrasil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	settingssvc "element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestLookupServiceRejectsMalformedFallbackProfile(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	redis := testutil.NewMemoryRedis()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet || req.URL.Path != "/minecraft/profile/lookup/name/RemotePlayer" {
			t.Errorf("fallback request=%s %s; want GET services lookup", req.Method, req.URL.Path)
			return
		}
		_, _ = w.Write([]byte(`{"id":"remote-id"}`))
	}))
	defer server.Close()
	if err := db.Fallbacks.SaveEndpoints(t.Context(), []dbfallback.Endpoint{{
		Priority:      1,
		SessionURL:    server.URL,
		AccountURL:    server.URL,
		ServicesURL:   server.URL,
		EnableProfile: true,
	}}); err != nil {
		t.Fatal(err)
	}
	settings := settingssvc.Settings{DB: db, Redis: redis}
	lookup := yggsvc.LookupService{
		Ygg:      yggsvc.Yggdrasil{DB: db, Cfg: testutil.TestConfig()},
		Fallback: fallbacksvc.Fallback{DB: db, Redis: redis, Settings: settings, Client: server.Client()},
	}
	profile, found, err := lookup.Name(t.Context(), "RemotePlayer", yggsvc.LookupServices)
	if profile != nil || found || err == nil || err.Error() != "invalid fallback profile lookup response" {
		t.Fatalf("malformed fallback profile=%#v found=%t err=%v", profile, found, err)
	}
}
