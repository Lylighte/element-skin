package oauth_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/httpapi"
	sitesvc "element-skin/backend/internal/service/site"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestOAuthAuthorizationCodeFlowIssuesDelegatedBearerForV1API(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-flow@test.com", "Password123", "OAuthFlow", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	session := webCookie(t, cfg.JWTSecret, user.ID)

	createRes := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Flow app",
		"description":  "Flow app description",
		"redirect_uri": "https://client.example/callback",
		"website_url":  "https://client.example",
		"client_type":  "confidential",
		"permissions":  []string{"account.read.self"},
	}, session, "")
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create app status=%d body=%s", createRes.Code, createRes.Body.String())
	}
	app := decodeMap(t, createRes.Body.Bytes())
	clientID := app["client_id"].(string)
	clientSecret := app["client_secret"].(string)
	if clientID == "" || clientSecret == "" || app["secret_hash"] != nil {
		t.Fatalf("client response should expose id and one-time secret only: %#v", app)
	}
	if got := app["permissions"].([]any); len(got) != 1 || got[0] != "account.read.self" {
		t.Fatalf("client permissions mismatch: %#v", got)
	}

	verifier := "test-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	authRes := doJSON(t, router, http.MethodPost, "/oauth/authorize", map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          "https://client.example/callback",
		"scope":                 "account.read.self",
		"state":                 "state-1",
		"code_challenge":        challenge,
		"code_challenge_method": "S256",
	}, session, "")
	if authRes.Code != http.StatusOK {
		t.Fatalf("authorize status=%d body=%s", authRes.Code, authRes.Body.String())
	}
	auth := decodeMap(t, authRes.Body.Bytes())
	code := auth["code"].(string)
	redirectURL := auth["redirect_url"].(string)
	if code == "" || !strings.Contains(redirectURL, "state=state-1") || !strings.Contains(redirectURL, "code=") {
		t.Fatalf("authorization response mismatch: %#v", auth)
	}

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", "https://client.example/callback")
	form.Set("code_verifier", verifier)
	tokenRes := doForm(t, router, "/oauth/token", form, "", "")
	if tokenRes.Code != http.StatusOK {
		t.Fatalf("token status=%d body=%s", tokenRes.Code, tokenRes.Body.String())
	}
	token := decodeMap(t, tokenRes.Body.Bytes())
	access := token["access_token"].(string)
	refresh := token["refresh_token"].(string)
	if access == "" || refresh == "" || token["token_type"] != "Bearer" || token["scope"] != "account.read.self" || token["expires_in"].(float64) != 3600 {
		t.Fatalf("token response mismatch: %#v", token)
	}

	meRes := doJSON(t, router, http.MethodGet, "/v1/users/me", nil, nil, access)
	if meRes.Code != http.StatusOK {
		t.Fatalf("bearer me status=%d body=%s", meRes.Code, meRes.Body.String())
	}
	me := decodeMap(t, meRes.Body.Bytes())
	if me["id"] != user.ID {
		t.Fatalf("bearer me user mismatch: %#v", me)
	}
	permissions := stringSet(me["permissions"].([]any))
	if !permissions["account.read.self"] || permissions["account.update.self"] {
		t.Fatalf("delegated permissions should be narrowed exactly: %#v", permissions)
	}

	updateRes := doJSON(t, router, http.MethodPatch, "/v1/users/me", map[string]any{"display_name": "ShouldFail"}, nil, access)
	if updateRes.Code != http.StatusForbidden || !strings.Contains(updateRes.Body.String(), "permission denied") {
		t.Fatalf("unauthorized bearer update mismatch: status=%d body=%s", updateRes.Code, updateRes.Body.String())
	}

	refreshForm := url.Values{}
	refreshForm.Set("grant_type", "refresh_token")
	refreshForm.Set("client_id", clientID)
	refreshForm.Set("client_secret", clientSecret)
	refreshForm.Set("refresh_token", refresh)
	refreshRes := doForm(t, router, "/oauth/token", refreshForm, "", "")
	if refreshRes.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshRes.Code, refreshRes.Body.String())
	}
	refreshed := decodeMap(t, refreshRes.Body.Bytes())
	if refreshed["access_token"] == access || refreshed["refresh_token"] == refresh || refreshed["scope"] != "account.read.self" {
		t.Fatalf("refresh should rotate tokens and preserve scope: %#v", refreshed)
	}
	reuseRes := doForm(t, router, "/oauth/token", refreshForm, "", "")
	if reuseRes.Code != http.StatusBadRequest || !strings.Contains(reuseRes.Body.String(), "invalid refresh_token") {
		t.Fatalf("refresh reuse mismatch: status=%d body=%s", reuseRes.Code, reuseRes.Body.String())
	}
}

func TestOAuthCreateAppRejectsScopeMissingFromActor(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	cfg := testutil.TestConfig()
	user := testutil.CreateUser(t, db, "oauth-scope-deny@test.com", "Password123", "OAuthScopeDeny", false)
	router := httpapi.NewRouter(cfg, db, sitesvc.Site{DB: db, Cfg: cfg}, yggsvc.Yggdrasil{DB: db, Cfg: cfg})
	res := doJSON(t, router, http.MethodPost, "/v1/oauth/apps", map[string]any{
		"name":         "Denied app",
		"redirect_uri": "https://client.example/callback",
		"client_type":  "confidential",
		"permissions":  []string{"account.ban.any"},
	}, webCookie(t, cfg.JWTSecret, user.ID), "")
	if res.Code != http.StatusForbidden || !strings.Contains(res.Body.String(), "permission denied") {
		t.Fatalf("scope deny mismatch: status=%d body=%s", res.Code, res.Body.String())
	}
}

func webCookie(t *testing.T, secret, userID string) *http.Cookie {
	t.Helper()
	token, err := util.CreateAccessToken(secret, userID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return &http.Cookie{Name: "access_token", Value: token}
}

func doJSON(t *testing.T, router http.Handler, method, path string, body any, cookie *http.Cookie, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func doForm(t *testing.T, router http.Handler, path string, form url.Values, cookieValue, bearer string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if cookieValue != "" {
		req.AddCookie(&http.Cookie{Name: "access_token", Value: cookieValue})
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func decodeMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode json %q: %v", string(data), err)
	}
	return out
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func stringSet(values []any) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		if s, ok := value.(string); ok {
			out[s] = true
		}
	}
	return out
}
