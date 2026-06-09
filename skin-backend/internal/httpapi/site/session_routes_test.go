package site_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"element-skin/backend/internal/httpapi/site"
	"element-skin/backend/internal/redisstore"
	sitesvc "element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSessionRoutesLoginSetsExactCookies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "site-login@test.com", "Password123", "SiteLogin", false)
	if err := db.Settings.Set(t.Context(), "jwt_expire_days", 2); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/site-login", strings.NewReader(`{"email":"site-login@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"user_id":"`+user.ID+`"`) || !strings.Contains(rec.Body.String(), `"is_admin":false`) {
		t.Fatalf("login body mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookies[0].Name != "access_token" || cookies[1].Name != "refresh_token" || !cookies[0].HttpOnly || !cookies[1].HttpOnly ||
		cookies[0].Path != "/" || cookies[1].Path != "/" || cookies[0].MaxAge != cfg.AccessMinutes*60 || cookies[1].MaxAge != 2*24*3600 {
		t.Fatalf("login should set exact http-only session cookies: %#v", cookies)
	}
}

func TestSessionRoutesRefreshRotatesAndLogoutRevokesExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	h := site.New(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	testutil.CreateUser(t, db, "site-refresh@test.com", "Password123", "SiteRefresh", false)

	req := httptest.NewRequest(http.MethodPost, "/site-login", strings.NewReader(`{"email":"site-refresh@test.com","password":"Password123"}`))
	rec := httptest.NewRecorder()
	h.Login(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login before refresh status=%d body=%q", rec.Code, rec.Body.String())
	}
	initialRefresh := cookieValue(t, rec.Result().Cookies(), "refresh_token")
	if initialRefresh == "" {
		t.Fatalf("login should issue refresh token cookies: %#v", rec.Result().Cookies())
	}

	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: initialRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_admin":false`) {
		t.Fatalf("refresh token response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	rotatedRefresh := cookieValue(t, rec.Result().Cookies(), "refresh_token")
	if rotatedRefresh == "" || rotatedRefresh == initialRefresh {
		t.Fatalf("refresh should rotate refresh cookie: old=%q cookies=%#v", initialRefresh, rec.Result().Cookies())
	}
	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: initialRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old refresh token should be single-use after rotation: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rotatedRefresh})
	rec = httptest.NewRecorder()
	h.Logout(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("logout response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 2 || cookieMaxAge(t, cookies, "access_token") != -1 || cookieMaxAge(t, cookies, "refresh_token") != -1 {
		t.Fatalf("logout should clear both session cookies: %#v", cookies)
	}
	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: rotatedRefresh})
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logout should revoke the current refresh token: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/session/refresh", nil)
	rec = httptest.NewRecorder()
	h.RefreshToken(rec, req)
	if rec.Code != http.StatusUnauthorized || !strings.Contains(rec.Body.String(), `"detail":"not authenticated"`) {
		t.Fatalf("refresh without cookie mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestSessionRoutesRegisterCreatesFirstAdminAndProfileExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	if err := db.Settings.Set(t.Context(), "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(`{"email":"new-user@test.com","password":"Password123","username":"New User"}`))
	rec := httptest.NewRecorder()
	h.Register(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"`) {
		t.Fatalf("register response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	id := jsonStringField(t, rec.Body.String(), "id")
	user, err := db.Users.GetByID(req.Context(), id)
	if err != nil || user == nil || !user.IsAdmin || !user.IsSuperAdmin || user.Email != "new-user@test.com" || user.DisplayName != "New User" {
		t.Fatalf("first registered user should be super admin exactly: user=%#v err=%v", user, err)
	}
	profiles, err := db.Profiles.GetByUser(req.Context(), id, 10)
	if err != nil || len(profiles) != 1 || profiles[0].Name != "new_user" || profiles[0].ID != util.OfflineUUIDNoDash("new_user") {
		t.Fatalf("register should create exact offline profile: profiles=%#v err=%v", profiles, err)
	}
}

func TestSessionRoutesVerificationAndResetPasswordExactFlow(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	h := site.NewWithRedis(cfg, db, redis, sitesvc.Site{DB: db, Cfg: cfg}, nil)
	user := testutil.CreateUser(t, db, "reset-flow@test.com", "Password123", "ResetFlow", false)

	req := httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"reset"}`))
	rec := httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"Email verification is disabled"`) {
		t.Fatalf("verification disabled response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	if err := db.Settings.Set(t.Context(), "email_verify_enabled", "true"); err != nil {
		t.Fatal(err)
	}
	if err := db.Settings.Set(t.Context(), "email_verify_ttl", "123"); err != nil {
		t.Fatal(err)
	}
	if err := redis.InvalidateSettings(t.Context()); err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"reset"}`))
	rec = httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true,\"ttl\":123}\n" {
		t.Fatalf("send reset verification response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	code, err := redis.GetVerificationCode(t.Context(), "reset-flow@test.com", "reset")
	if err != nil || code == "" {
		t.Fatalf("reset verification code should be stored in redis: code=%q err=%v", code, err)
	}

	req = httptest.NewRequest(http.MethodPost, "/reset-password", strings.NewReader(`{"email":"reset-flow@test.com","password":"NewPassword123","code":"`+code+`"}`))
	rec = httptest.NewRecorder()
	h.ResetPassword(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"ok\":true}\n" {
		t.Fatalf("reset password response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
	updated, err := db.Users.GetByID(req.Context(), user.ID)
	if err != nil || updated == nil || !util.VerifyPassword("NewPassword123", updated.Password) {
		t.Fatalf("reset password should update user password: user=%#v err=%v", updated, err)
	}
	if _, err := redis.GetVerificationCode(t.Context(), "reset-flow@test.com", "reset"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset password should delete verification code, got %v", err)
	}

	req = httptest.NewRequest(http.MethodPost, "/verification-code", strings.NewReader(`{"email":"reset-flow@test.com","type":"bad"}`))
	rec = httptest.NewRecorder()
	h.SendVerificationCode(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"detail":"invalid verification type"`) {
		t.Fatalf("invalid verification type response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func cookieValue(t *testing.T, cookies []*http.Cookie, name string) string {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c.Value
		}
	}
	return ""
}

func cookieMaxAge(t *testing.T, cookies []*http.Cookie, name string) int {
	t.Helper()
	for _, c := range cookies {
		if c.Name == name {
			return c.MaxAge
		}
	}
	t.Fatalf("missing cookie %q in %#v", name, cookies)
	return 0
}
