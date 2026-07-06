package bridge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"element-skin/union-svc/internal/config"
	"element-skin/union-svc/internal/oauth"
	"element-skin/union-svc/internal/union"

	_ "modernc.org/sqlite"
)

func testOAuthManager(t *testing.T, accessToken string) *oauth.Manager {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "oauth.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	store, err := oauth.NewStore(db)
	if err != nil {
		t.Fatalf("create token store: %v", err)
	}
	if accessToken != "" {
		row := oauth.TokenRow{
			AccessToken:  accessToken,
			RefreshToken: "refresh-token",
			ExpiresAtMS:  time.Now().Add(time.Hour).UnixMilli(),
		}
		if err := store.Save(context.Background(), row); err != nil {
			t.Fatalf("save token: %v", err)
		}
	}
	return oauth.NewManagerWithDeps(config.Config{}, store, nil)
}

func TestBridgeListProfilesMapsUnionResponse(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/unmapped/byname/Steve" {
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Union-Member-Key"); got != "member-key" {
			t.Errorf("member key = %q, want member-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]union.Profile{
			{UUID: "uuid-1", Name: "Steve"},
			{UUID: "uuid-2", Name: "Steve2"},
		})
	}))
	defer hub.Close()

	uc := union.NewClientWithDeps(hub.URL, "member-key", 30, hub.Client(), nil, nil)
	b := New("http://127.0.0.1:1", uc, testOAuthManager(t, ""), hub.Client())

	items, err := b.ListProfiles(context.Background(), "Steve")
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0].UUID != "uuid-1" || items[0].Name != "Steve" {
		t.Errorf("items[0] = %+v, want uuid-1/Steve", items[0])
	}
	if items[1].UUID != "uuid-2" || items[1].Name != "Steve2" {
		t.Errorf("items[1] = %+v, want uuid-2/Steve2", items[1])
	}
}

func TestBridgeListProfilesReturnsNotConfiguredWhenUnionHubMissing(t *testing.T) {
	uc := union.NewClientWithDeps("", "", 30, nil, nil, nil)
	b := New("http://127.0.0.1:1", uc, testOAuthManager(t, ""), nil)

	_, err := b.ListProfiles(context.Background(), "Steve")
	if !errors.Is(err, union.ErrUnionNotConfigured) {
		t.Fatalf("expected ErrUnionNotConfigured, got %v", err)
	}
}

func TestBridgeImportProfileCreatesProfileWithBearerToken(t *testing.T) {
	var gotAuth string
	var gotBody map[string]string
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id":    "profile-id",
			"name":  "Steve",
			"model": "default",
		})
	}))
	defer elementskin.Close()

	uc := union.NewClientWithDeps("http://127.0.0.1:1", "key", 30, nil, nil, nil)
	b := New(elementskin.URL, uc, testOAuthManager(t, "access-token-123"), elementskin.Client())

	profile, err := b.ImportProfile(context.Background(), ImportProfileRequest{Name: "Steve", Model: "default"})
	if err != nil {
		t.Fatalf("import profile: %v", err)
	}
	if profile.ID != "profile-id" || profile.Name != "Steve" {
		t.Errorf("profile = %+v, want profile-id/Steve", profile)
	}
	if gotAuth != "Bearer access-token-123" {
		t.Errorf("authorization = %q, want Bearer access-token-123", gotAuth)
	}
	if gotBody["name"] != "Steve" || gotBody["model"] != "default" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestBridgeImportProfilePassesThroughConflictError(t *testing.T) {
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "profile name already exists"})
	}))
	defer elementskin.Close()

	uc := union.NewClientWithDeps("http://127.0.0.1:1", "key", 30, nil, nil, nil)
	b := New(elementskin.URL, uc, testOAuthManager(t, "token"), elementskin.Client())

	_, err := b.ImportProfile(context.Background(), ImportProfileRequest{Name: "Steve", Model: "slim"})
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusConflict {
		t.Errorf("status = %d, want %d", apiErr.Status, http.StatusConflict)
	}
	if apiErr.Detail != "profile name already exists" {
		t.Errorf("detail = %q, want profile name already exists", apiErr.Detail)
	}
}

func TestBridgeImportProfileFailsWithoutToken(t *testing.T) {
	b := New("http://127.0.0.1:1", nil, testOAuthManager(t, ""), nil)

	_, err := b.ImportProfile(context.Background(), ImportProfileRequest{Name: "Steve"})
	if !errors.Is(err, oauth.ErrNoToken) {
		t.Fatalf("expected ErrNoToken, got %v", err)
	}
}

func TestElementSkinClientCreateProfilePassesThroughPlainTextErrorBody(t *testing.T) {
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, "bad request")
	}))
	defer elementskin.Close()

	client := NewElementSkinClient(elementskin.URL, elementskin.Client())
	_, err := client.CreateProfile(context.Background(), "token", "Steve", "default")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", apiErr.Status, http.StatusBadRequest)
	}
	if apiErr.Detail != "bad request" {
		t.Errorf("detail = %q, want bad request", apiErr.Detail)
	}
}

func TestElementSkinClientCreateProfileReturns400JSONError(t *testing.T) {
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "invalid profile name"})
	}))
	defer elementskin.Close()

	client := NewElementSkinClient(elementskin.URL, elementskin.Client())
	_, err := client.CreateProfile(context.Background(), "token", "", "default")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", apiErr.Status, http.StatusBadRequest)
	}
	if apiErr.Detail != "invalid profile name" {
		t.Errorf("detail = %q, want 'invalid profile name'", apiErr.Detail)
	}
}

func TestBridgeListProfilesReturnsEmptyListWhenHubReturnsEmpty(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer hub.Close()

	uc := union.NewClientWithDeps(hub.URL, "member-key", 30, hub.Client(), nil, nil)
	b := New("http://127.0.0.1:1", uc, testOAuthManager(t, ""), hub.Client())

	items, err := b.ListProfiles(context.Background(), "Steve")
	if err != nil {
		t.Fatalf("list profiles with empty hub response: %v", err)
	}
	if items == nil {
		t.Fatal("expected non-nil empty slice, got nil")
	}
	if len(items) != 0 {
		t.Fatalf("len(items) = %d, want 0", len(items))
	}
}

func TestBridgeListProfilesReturnsErrorWhenHubFails(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer hub.Close()

	uc := union.NewClientWithDeps(hub.URL, "member-key", 30, hub.Client(), nil, nil)
	b := New("http://127.0.0.1:1", uc, testOAuthManager(t, ""), hub.Client())

	_, err := b.ListProfiles(context.Background(), "Steve")
	if err == nil {
		t.Fatal("expected error from hub failure")
	}
	var hubErr *union.HubError
	if !errors.As(err, &hubErr) {
		t.Fatalf("expected *union.HubError, got %T", err)
	}
	if hubErr.Status != http.StatusInternalServerError {
		t.Errorf("hub error status = %d, want 500", hubErr.Status)
	}
}

func TestElementSkinClientCreateProfileReturns409JSONError(t *testing.T) {
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "name already exists"})
	}))
	defer elementskin.Close()

	client := NewElementSkinClient(elementskin.URL, elementskin.Client())
	_, err := client.CreateProfile(context.Background(), "token", "Steve", "default")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusConflict {
		t.Errorf("status = %d, want %d", apiErr.Status, http.StatusConflict)
	}
	if apiErr.Detail != "name already exists" {
		t.Errorf("detail = %q, want 'name already exists'", apiErr.Detail)
	}
}
