package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"element-skin/union-svc/internal/oauth"
	"element-skin/union-svc/internal/union"
)

func seedOAuthToken(t *testing.T, storagePath, accessToken string) {
	t.Helper()
	store, err := oauth.OpenStore(storagePath)
	if err != nil {
		t.Fatalf("open token store: %v", err)
	}
	defer store.Close()
	row := oauth.TokenRow{
		AccessToken:  accessToken,
		RefreshToken: "refresh",
		ExpiresAtMS:  time.Now().Add(time.Hour).UnixMilli(),
	}
	if err := store.Save(context.Background(), row); err != nil {
		t.Fatalf("save token: %v", err)
	}
}

func TestListProfilesEndpointReturnsUnionProfiles(t *testing.T) {
	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/unmapped/byname/Steve" {
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]union.Profile{
			{UUID: "u1", Name: "Steve"},
		})
	}))
	defer hub.Close()

	cfg := testConfig("http://127.0.0.1:1")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")
	cfg.Union.HubURL = hub.URL
	cfg.Union.MemberKey = "member-key"

	srv, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/profiles?username=Steve")
	if err != nil {
		t.Fatalf("get profiles: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, string(body))
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	items := got["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	item := items[0].(map[string]any)
	if item["uuid"] != "u1" || item["name"] != "Steve" {
		t.Errorf("item = %+v, want u1/Steve", item)
	}
}

func TestListProfilesEndpointRejectsMissingUsername(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")

	srv, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/profiles")
	if err != nil {
		t.Fatalf("get profiles: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestImportProfileEndpointCreatesProfileAndPassesThroughConflict(t *testing.T) {
	elementskin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/profiles" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer token-123" {
			t.Errorf("authorization = %q, want Bearer token-123", auth)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["name"] != "Steve" || body["model"] != "default" {
			t.Errorf("body = %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_ = json.NewEncoder(w).Encode(map[string]any{"detail": "name already taken"})
	}))
	defer elementskin.Close()

	cfg := testConfig(elementskin.URL)
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")
	seedOAuthToken(t, cfg.Storage.Path, "token-123")

	srv, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	reqBody := `{"name":"Steve"}`
	resp, err := http.Post(ts.URL+"/api/profiles/import", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("post import: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 409: %s", resp.StatusCode, string(body))
	}
	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got["detail"] != "name already taken" {
		t.Errorf("detail = %q, want name already taken", got["detail"])
	}
}

func TestImportProfileEndpointRejectsInvalidModel(t *testing.T) {
	cfg := testConfig("http://127.0.0.1:1")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")

	srv, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	defer srv.Close()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/api/profiles/import", "application/json", strings.NewReader(`{"name":"Steve","model":"wide"}`))
	if err != nil {
		t.Fatalf("post import: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}
