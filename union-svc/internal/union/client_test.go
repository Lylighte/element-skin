package union

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"element-skin/union-svc/internal/config"
)

type methodRecord struct {
	method  string
	path    string
	body    string
	headers http.Header
}

func record(r *http.Request) methodRecord {
	body, _ := io.ReadAll(r.Body)
	return methodRecord{
		method:  r.Method,
		path:    r.URL.Path,
		body:    string(body),
		headers: r.Header.Clone(),
	}
}

func openTestNonceStore(t *testing.T) *NonceStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory sqlite: %v", err)
	}
	s, err := NewNonceStore(db)
	if err != nil {
		t.Fatalf("create nonce store: %v", err)
	}
	return s
}

func newTestClient(t *testing.T, hubURL string) *Client {
	t.Helper()
	return NewClientWithDeps(hubURL, "test-member-key", 5, nil, openTestNonceStore(t))
}

func assertMethod(t *testing.T, got methodRecord, wantMethod, wantPath string) {
	t.Helper()
	if got.method != wantMethod {
		t.Fatalf("expected method %s, got %s", wantMethod, got.method)
	}
	if got.path != wantPath {
		t.Fatalf("expected path %s, got %s", wantPath, got.path)
	}
}

func assertHeader(t *testing.T, got methodRecord, key, want string) {
	t.Helper()
	if got.headers.Get(key) != want {
		t.Fatalf("expected header %s=%q, got %q", key, want, got.headers.Get(key))
	}
}

func assertJSONField(t *testing.T, body, key, want string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}
	if m[key] != want {
		t.Fatalf("expected body field %s=%q, got %v", key, want, m[key])
	}
}

func TestNewClientUsesDefaultTimeout(t *testing.T) {
	cfg := config.Config{}
	cfg.Storage.Path = filepath.Join(t.TempDir(), "test.db")

	client, err := NewClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	defer client.Close()

	if client.timeout != 30*time.Second {
		t.Fatalf("expected default timeout 30s, got %v", client.timeout)
	}
}

func TestSyncProfileAdd(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.SyncProfileAdd(context.Background(), "Steve", "uuid-1"); err != nil {
		t.Fatalf("SyncProfileAdd failed: %v", err)
	}

	assertMethod(t, got, http.MethodPost, "/profile")
	assertHeader(t, got, memberKeyHeader, "test-member-key")
	assertJSONField(t, got.body, "id", "uuid-1")
	assertJSONField(t, got.body, "name", "Steve")
}

func TestSyncProfileUpdate(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.SyncProfileUpdate(context.Background(), "uuid-2", "Alex"); err != nil {
		t.Fatalf("SyncProfileUpdate failed: %v", err)
	}

	assertMethod(t, got, http.MethodPut, "/profile/uuid-2")
	assertJSONField(t, got.body, "name", "Alex")
}

func TestSyncProfileDelete(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.SyncProfileDelete(context.Background(), "uuid-3"); err != nil {
		t.Fatalf("SyncProfileDelete failed: %v", err)
	}

	assertMethod(t, got, http.MethodDelete, "/profile/uuid-3")
}

func TestGetProfiles(t *testing.T) {
	profiles := []Profile{
		{UUID: "uuid-a", Name: "Steve", InternalID: "internal-a"},
		{UUID: "uuid-b", Name: "Steve", InternalID: "internal-b"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profile/unmapped/byname/Steve" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(profiles)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	got, err := client.GetProfiles(context.Background(), "Steve")
	if err != nil {
		t.Fatalf("GetProfiles failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}
	if got[0].InternalID != "internal-a" {
		t.Fatalf("unexpected profile: %+v", got[0])
	}
}

func TestGetSecurityLevel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/code":
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "secret-code"})
		case "/backend/secret-code/security/level":
			_, _ = w.Write([]byte("3"))
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	level, err := client.GetSecurityLevel(context.Background(), "ignored")
	if err != nil {
		t.Fatalf("GetSecurityLevel failed: %v", err)
	}
	if level != 3 {
		t.Fatalf("expected security level 3, got %d", level)
	}
}

func TestQueryBlacklist(t *testing.T) {
	entries := []BlacklistEntry{
		{ID: "1", Email: "bad@example.com", Reason: "spam"},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/blacklist/query" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"items": entries})
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	entry, err := client.QueryBlacklist(context.Background(), "bad@example.com")
	if err != nil {
		t.Fatalf("QueryBlacklist failed: %v", err)
	}
	if entry.ID != "1" || entry.Email != "bad@example.com" {
		t.Fatalf("unexpected entry: %+v", entry)
	}
}

func TestCreateBlacklist(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.CreateBlacklist(context.Background(), BlacklistEntry{
		Email:  "bad@example.com",
		Reason: "spam",
	}); err != nil {
		t.Fatalf("CreateBlacklist failed: %v", err)
	}

	assertMethod(t, got, http.MethodPost, "/blacklist/restful")
	assertJSONField(t, got.body, "email", "bad@example.com")
	assertJSONField(t, got.body, "reason", "spam")
}

func TestDeleteBlacklist(t *testing.T) {
	var deleted bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/blacklist/query":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"items": []BlacklistEntry{{ID: "42", Email: "bad@example.com"}},
			})
		case "/blacklist/restful/42":
			deleted = true
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.DeleteBlacklist(context.Background(), "bad@example.com"); err != nil {
		t.Fatalf("DeleteBlacklist failed: %v", err)
	}
	if !deleted {
		t.Fatal("expected delete endpoint to be called")
	}
}
