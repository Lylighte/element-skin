package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/httpapi"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
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
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/users/me", nil))
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("missing cookie auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: "not-a-jwt"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), "not authenticated") {
		t.Fatalf("invalid jwt auth mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "permission denied") {
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
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("redis auth error should fail without DB fallback, got %d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthUsesRedisCachedSubjectIDButRecomputesPermissions(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-hit@test.com", "Password123", "AuthCacheHit", true)
	cache := testutil.NewMemoryRedis()
	if err := cache.SetAuthUser(t.Context(), redisstore.AuthUser{ID: user.ID}, time.Minute); err != nil {
		t.Fatal(err)
	}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("cached subject ID should still use DB permissions: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthFailsClosedWhenColdCacheCannotBePopulated(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-cache-write@test.com", "Password123", "AuthCacheWrite", false)
	cache := &authCacheWriteFailStore{Store: redisstore.NewMemoryStore()}
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError || rec.Body.String() != "{\"detail\":\"Internal server error\"}\n" {
		t.Fatalf("cold-cache write failure mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if cache.setCalls != 1 {
		t.Fatalf("auth middleware should attempt one cache population, calls=%d", cache.setCalls)
	}
	if _, err := cache.Store.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed cache population must not leave a partial entry, got %v", err)
	}
}

func TestAuthCachesBanStateWithoutBlockingWebDashboard(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-banned-cold@test.com", "Password123", "AuthBannedCold", false)
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if err := db.Users.Ban(t.Context(), user.ID, bannedUntil); err != nil {
		t.Fatal(err)
	}
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.AddCookie(&http.Cookie{Name: "access_token", Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`+user.ID+`"`) {
		t.Fatalf("banned web user should keep dashboard access: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cached, err := cache.GetAuthUser(t.Context(), user.ID)
	if err != nil || cached.BannedUntil == nil || *cached.BannedUntil != bannedUntil || !cached.Banned(time.Now()) {
		t.Fatalf("ban state should be cached exactly: cached=%#v err=%v", cached, err)
	}
}

func TestAuthAcceptsDelegatedBearerAndNarrowsPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "auth-bearer-user@test.com", "Password123", "AuthBearerUser", false)
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	clientID := createActiveOAuthClientForAuthTest(t, db, user.ID, "auth-bearer-client", []int64{
		int64(permission.MustDefinitionByCode("account.read.self").ID),
	})
	grantID := "grant-auth-bearer"
	if err := db.OAuth.CreateGrant(t.Context(), model.OAuthGrant{
		ID:        grantID,
		UserID:    user.ID,
		SubjectID: permissiondb.SubjectIDForUser(user.ID),
		ClientID:  clientID,
		Status:    "active",
		CreatedAt: database.NowMS(),
	}, []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)}); err != nil {
		t.Fatal(err)
	}
	rawToken := "delegated-bearer-token"
	now := database.NowMS()
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawToken),
		ClientID:      clientID,
		UserID:        user.ID,
		GrantID:       grantID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("delegated bearer me status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body struct {
		ID          string   `json:"id"`
		Permissions []string `json:"permissions"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode bearer me: %v body=%q", err, rec.Body.String())
	}
	if body.ID != user.ID {
		t.Fatalf("delegated bearer user id mismatch: got %q want %q", body.ID, user.ID)
	}
	if !containsAuthPermission(body.Permissions, "account.read.self") || containsAuthPermission(body.Permissions, "account.update.self") {
		t.Fatalf("delegated bearer permissions should be narrowed to token scope: %#v", body.Permissions)
	}

	req = httptest.NewRequest(http.MethodPatch, "/v1/users/me", strings.NewReader(`{"display_name":"ShouldFail"}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || rec.Body.String() != "{\"detail\":\"permission denied\"}\n" {
		t.Fatalf("delegated bearer update denial mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthAcceptsClientBearerAndRejectsInactiveOrMissingBearerExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	owner := testutil.CreateUser(t, db, "auth-client-owner@test.com", "Password123", "AuthClientOwner", false)
	profileUser := testutil.CreateUser(t, db, "auth-client-profile@test.com", "Password123", "AuthClientProfile", false)
	profile := testutil.CreateProfile(t, db, profileUser.ID, "AuthBearerProfile", "default")
	cache := redisstore.NewMemoryStore()
	router := httpapi.NewRouterWithRedis(cfg, db, cache, sitesvc.Site{DB: db, Cfg: cfg, Redis: cache}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	clientID := createActiveOAuthClientForAuthTest(t, db, owner.ID, "auth-client-bearer", nil)
	def := permission.MustDefinitionByCode("minecraft_session.hasjoined.server")
	if err := db.Permissions.SetPermissionOverrideForSubject(t.Context(), permissiondb.SubjectIDForClient(clientID), def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	rawToken := "client-bearer-token"
	now := database.NowMS()
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawToken),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(def.ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/minecraft/session/has-joined", strings.NewReader(`{"username":"`+profile.Name+`","server_id":"route-server"}`))
	req.Header.Set("Authorization", "Bearer "+rawToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"joined\":false,\"profile\":null}\n" {
		t.Fatalf("client bearer has-joined mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer missing-token")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"detail\":\"not authenticated\"}\n" {
		t.Fatalf("missing bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	expired := "expired-client-bearer"
	if err := cache.SetOAuthAccessToken(t.Context(), redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(expired),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(def.ID)},
		ExpiresAt:     now - 1,
		CreatedAt:     now - int64(time.Hour/time.Millisecond),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/v1/minecraft/session/has-joined", strings.NewReader(`{"username":"`+profile.Name+`","server_id":"route-server"}`))
	req.Header.Set("Authorization", "Bearer "+expired)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized || rec.Body.String() != "{\"detail\":\"not authenticated\"}\n" {
		t.Fatalf("expired bearer mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func createActiveOAuthClientForAuthTest(t *testing.T, db *database.DB, ownerUserID, id string, permissionIDs []int64) string {
	t.Helper()
	now := database.NowMS()
	client := model.OAuthClient{
		ID:          id,
		OwnerUserID: ownerUserID,
		Name:        id,
		RedirectURI: "https://client.example/callback",
		ClientType:  "confidential",
		SecretHash:  util.HashRefreshToken("secret-" + id),
		Status:      "active",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := db.OAuth.CreateClient(t.Context(), client, permissionIDs); err != nil {
		t.Fatal(err)
	}
	return id
}

func containsAuthPermission(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type authCacheWriteFailStore struct {
	redisstore.Store
	setCalls int
}

func (s *authCacheWriteFailStore) SetAuthUser(context.Context, redisstore.AuthUser, time.Duration) error {
	s.setCalls++
	return errors.New("cache write failed")
}
