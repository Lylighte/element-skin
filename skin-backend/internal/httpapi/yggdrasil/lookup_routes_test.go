package yggdrasil_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/yggdrasil"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/service/settings"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestLookupRoutesNamesReturnExactLocalProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-lookup@test.com", "Password123", "YggLookup", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_lookup_profile", "YggLookupProfile")

	req := httptest.NewRequest(http.MethodPost, "/api/profiles/minecraft", strings.NewReader(`["YggLookupProfile","MissingName"]`))
	rec := httptest.NewRecorder()
	h.LookupNames(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) || !strings.Contains(rec.Body.String(), `"name":"YggLookupProfile"`) {
		t.Fatalf("lookup names response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestProfileAndLookupRoutesReturnLocalProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := yggdrasil.New(cfg, db, redis, settings.Settings{DB: db, Redis: redis}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	user := testutil.CreateUser(t, db, "ygg-profile-route@test.com", "Password123", "YggProfileRoute", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_route", "YggProfileRoutePlayer")

	req := httptest.NewRequest(http.MethodGet, "/sessionserver/session/minecraft/profile/"+profile.ID+"?unsigned=true", nil)
	req.SetPathValue("uuid", profile.ID)
	rec := httptest.NewRecorder()
	h.Profile(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile route status=%d body=%q", rec.Code, rec.Body.String())
	}
	var profileBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profileBody); err != nil {
		t.Fatal(err)
	}
	if profileBody["id"] != profile.ID || profileBody["name"] != profile.Name {
		t.Fatalf("profile route body mismatch: %#v", profileBody)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/profiles/minecraft/"+profile.Name, nil)
	req.SetPathValue("playerName", profile.Name)
	rec = httptest.NewRecorder()
	h.LookupName(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+profile.ID+`"`) {
		t.Fatalf("lookup name route mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestWriteFallbackForTest(t *testing.T) {
	rec := httptest.NewRecorder()
	if !yggdrasil.WriteFallbackForTest(rec, &fallbacksvc.FallbackResponse{Status: http.StatusAccepted, Body: []byte(`{"ok":true}`)}) {
		t.Fatal("fallback response should be written")
	}
	if rec.Code != http.StatusAccepted || rec.Body.String() != `{"ok":true}` {
		t.Fatalf("fallback response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if yggdrasil.WriteFallbackForTest(httptest.NewRecorder(), nil) {
		t.Fatal("nil fallback response should not be written")
	}
}
