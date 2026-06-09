package httpapi_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAuthRejectsMissingInvalidAndNonAdminExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-direct-user@test.com", "Password123", "AuthDirectUser", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me", nil))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("missing cookie auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "not-a-jwt"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("invalid jwt auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, false, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "admin required") {
		t.Fatalf("non-admin auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthRedisErrorDoesNotFallBackToDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-redis-error@test.com", "Password123", "AuthRedisError", true)
	cache := redisstore.NewMemoryStore()
	cache.Err = errors.New("redis down")
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis auth error should fail without DB fallback, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthUsesRedisCachedUserStateOnCacheHit(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-hit@test.com", "Password123", "AuthCacheHit", true)
	cache := testutil.NewMemoryRedis()
	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID, IsAdmin: false, IsSuperAdmin: false}, time.Minute); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, true, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "admin required") {
		t.Fatalf("cached non-admin state should override newer DB admin state until invalidated: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
