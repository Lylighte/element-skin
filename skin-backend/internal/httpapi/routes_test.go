package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestRoutesRegistersPublicAndYggdrasilEntrypointsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	cases := []struct {
		method string
		path   string
		body   string
		status int
		want   string
		exact  bool
	}{
		{method: http.MethodGet, path: "/", status: http.StatusOK, want: "implementationName"},
		{method: http.MethodGet, path: "/v2/public/settings", status: http.StatusOK, want: "site_name"},
		{method: http.MethodPost, path: "/authserver/validate", body: `{"accessToken":"missing"}`, status: http.StatusForbidden, want: "{\"error\":\"ForbiddenOperationException\",\"errorMessage\":\"Invalid token.\"}\n", exact: true},
	}
	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		bodyMatches := rec.Body.String() == tc.want
		if !tc.exact {
			bodyMatches = strings.Contains(rec.Body.String(), tc.want)
		}
		if rec.Code != tc.status || !bodyMatches {
			t.Fatalf("%s %s mismatch: status=%d body=%q", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestRoutesRegisterMinecraftServicesLookupsAndPublicKeys(t *testing.T) {
	db, router := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "services-routes@test.com", "Password123", "ServicesRoutes", false)
	profile := testutil.CreateProfile(t, db, user.ID, "services_routes_profile", "ServicesRoutesPlayer")

	namePath := "/minecraftservices/minecraft/profile/lookup/name/" + profile.Name
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, namePath, nil))
	wantProfile := "{\"id\":\"" + profile.ID + "\",\"name\":\"" + profile.Name + "\"}\n"
	if rec.Code != http.StatusOK || rec.Body.String() != wantProfile {
		t.Fatalf("services name lookup status=%d body=%q; want 200 %q", rec.Code, rec.Body.String(), wantProfile)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/minecraftservices/minecraft/profile/lookup/name/MissingServicesPlayer", nil))
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("missing services name lookup status=%d body=%q; want 204 empty", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/minecraftservices/minecraft/profile/lookup/bulk/byname", strings.NewReader(`["ServicesRoutesPlayer","MissingServicesPlayer"]`)))
	var profiles []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode services bulk lookup body=%q: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || len(profiles) != 1 || profiles[0]["id"] != profile.ID || profiles[0]["name"] != profile.Name {
		t.Fatalf("services bulk lookup status=%d profiles=%#v", rec.Code, profiles)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/minecraft/profile/lookup/bulk/byname", strings.NewReader(`["ServicesRoutesPlayer","MissingServicesPlayer"]`)))
	profiles = nil
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("api minecraft bulk lookup body=%q: %v", rec.Body.String(), err)
	}
	if rec.Code != http.StatusOK || len(profiles) != 1 || profiles[0]["id"] != profile.ID || profiles[0]["name"] != profile.Name {
		t.Fatalf("api minecraft bulk lookup status=%d profiles=%#v", rec.Code, profiles)
	}

	legacy := httptest.NewRecorder()
	router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, "/api/publickeys", nil))
	services := httptest.NewRecorder()
	router.ServeHTTP(services, httptest.NewRequest(http.MethodGet, "/minecraftservices/publickeys", nil))
	if services.Code != http.StatusOK || services.Body.String() != legacy.Body.String() || services.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("services public keys status=%d body=%q; api public keys status=%d body=%q", services.Code, services.Body.String(), legacy.Code, legacy.Body.String())
	}
}

func TestRoutesDoNotRetainV1CompatibilityPaths(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	router := httpapi.NewRouter(cfg, db, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	for _, path := range []string{
		"/v1/public/settings",
		"/v1/auth/login",
		"/v1/users/me",
		"/v1/admin/users",
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound || rec.Body.String() != "404 page not found\n" {
			t.Fatalf("legacy path %s mismatch: status=%d body=%q", path, rec.Code, rec.Body.String())
		}
	}
}
