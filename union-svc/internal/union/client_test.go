package union

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"element-skin/union-svc/internal/config"
)

type methodRecord struct {
	method  string
	path    string
	query   string
	body    string
	headers http.Header
}

func record(r *http.Request) methodRecord {
	body, _ := io.ReadAll(r.Body)
	return methodRecord{
		method:  r.Method,
		path:    r.URL.Path,
		query:   r.URL.RawQuery,
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

func openTestSettingsStore(t *testing.T) *SettingsStore {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open memory sqlite: %v", err)
	}
	s, err := NewSettingsStore(db)
	if err != nil {
		t.Fatalf("create settings store: %v", err)
	}
	return s
}

func newTestClient(t *testing.T, hubURL string) *Client {
	t.Helper()
	return NewClientWithDeps(hubURL, "test-member-key", 5, nil, openTestNonceStore(t), openTestSettingsStore(t))
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

func assertQuery(t *testing.T, got methodRecord, key, want string) {
	t.Helper()
	values, err := url.ParseQuery(got.query)
	if err != nil {
		t.Fatalf("parse query %q: %v", got.query, err)
	}
	if values.Get(key) != want {
		t.Fatalf("expected query %s=%q, got %q", key, want, values.Get(key))
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

func TestSearchBlacklist(t *testing.T) {
	entries := []BlacklistEntry{
		{ID: "1", Email: "bad@example.com", Source: "manual", Reason: "spam", CreatedAt: "2024-01-01", ValidUntil: "2025-01-01"},
		{ID: "2", Email: "worse@example.com", Source: "sync", Reason: "abuse", CreatedAt: "2024-02-01", ValidUntil: "2025-02-01"},
	}

	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		_ = json.NewEncoder(w).Encode(entries)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	gotEntries, err := client.SearchBlacklist(context.Background(), "bad")
	if err != nil {
		t.Fatalf("SearchBlacklist failed: %v", err)
	}

	assertMethod(t, got, http.MethodGet, "/blacklist/query")
	assertQuery(t, got, "q", "bad")
	assertHeader(t, got, memberKeyHeader, "test-member-key")

	if len(gotEntries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(gotEntries))
	}
	if gotEntries[0].ID != "1" || gotEntries[0].Email != "bad@example.com" {
		t.Fatalf("unexpected first entry: %+v", gotEntries[0])
	}
	if gotEntries[1].ID != "2" || gotEntries[1].Email != "worse@example.com" {
		t.Fatalf("unexpected second entry: %+v", gotEntries[1])
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
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.DeleteBlacklist(context.Background(), "42"); err != nil {
		t.Fatalf("DeleteBlacklist failed: %v", err)
	}

	assertMethod(t, got, http.MethodDelete, "/blacklist/restful/42")
	assertHeader(t, got, memberKeyHeader, "test-member-key")
}

func TestInvalidateBlacklist(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	if err := client.InvalidateBlacklist(context.Background(), "42"); err != nil {
		t.Fatalf("InvalidateBlacklist failed: %v", err)
	}

	assertMethod(t, got, http.MethodPut, "/blacklist/invalidate/42")
	assertHeader(t, got, memberKeyHeader, "test-member-key")
}

func TestFetchServerList(t *testing.T) {
	var got methodRecord
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"servers":[{"name":"hub1"}],"version":3}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	servers, version, err := client.FetchServerList(context.Background())
	if err != nil {
		t.Fatalf("FetchServerList failed: %v", err)
	}

	assertMethod(t, got, http.MethodGet, "/serverlist")
	assertHeader(t, got, memberKeyHeader, "test-member-key")
	if version != 3 {
		t.Fatalf("expected version 3, got %d", version)
	}
	wantServers := `[{"name":"hub1"}]`
	if string(servers) != wantServers {
		t.Fatalf("expected servers %q, got %q", wantServers, string(servers))
	}
}

func TestFetchServerListReturnsHubError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"forbidden"}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	_, _, err := client.FetchServerList(context.Background())
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	var hubErr *HubError
	if !errors.As(err, &hubErr) {
		t.Fatalf("expected *HubError, got %T", err)
	}
	if hubErr.Status != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", hubErr.Status)
	}
}

func TestFetchPrivateKey(t *testing.T) {
	var got methodRecord
	pemData := "-----BEGIN RSA PRIVATE KEY-----\nMIIBOg==\n-----END RSA PRIVATE KEY-----"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = record(r)
		w.Header().Set("Content-Type", "application/json")
		resp, _ := json.Marshal(map[string]any{
			"privateKey":        pemData,
			"privateKeyVersion": 5,
		})
		_, _ = w.Write(resp)
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	gotPEM, version, err := client.FetchPrivateKey(context.Background())
	if err != nil {
		t.Fatalf("FetchPrivateKey failed: %v", err)
	}

	assertMethod(t, got, http.MethodGet, "/privatekey")
	assertHeader(t, got, memberKeyHeader, "test-member-key")
	if version != 5 {
		t.Fatalf("expected version 5, got %d", version)
	}
	if gotPEM != pemData {
		t.Fatalf("expected PEM %q, got %q", pemData, gotPEM)
	}
}

func TestDynamicMemberKeyFromSettings(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get(memberKeyHeader)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"servers":[],"version":1}`))
	}))
	defer server.Close()

	client := newTestClient(t, server.URL)
	defer client.Close()

	ctx := context.Background()
	if err := client.settings.Set(ctx, "member_key", "runtime-key"); err != nil {
		t.Fatalf("set runtime member_key: %v", err)
	}

	if _, _, err := client.FetchServerList(ctx); err != nil {
		t.Fatalf("FetchServerList failed: %v", err)
	}
	if gotKey != "runtime-key" {
		t.Fatalf("expected runtime member key %q, got %q", "runtime-key", gotKey)
	}
}
