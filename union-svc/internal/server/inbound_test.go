package server

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"element-skin/union-svc/internal/union"
)

const (
	signatureHeader = "X-Message-Signature"
	timestampHeader = "X-Message-Timestamp"
	nonceHeader     = "X-Message-Nonce"
)

// setupInboundTestServer creates a Server configured to talk to a mock Union
// Hub that advertises the given public key.
func setupInboundTestServer(t *testing.T, pubPEM string) (*Server, *httptest.Server) {
	t.Helper()

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{
			"union_host_signature_public_key": pubPEM,
		})
	}))
	t.Cleanup(hub.Close)

	cfg := testConfig("http://127.0.0.1:1")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")
	cfg.Union.HubURL = hub.URL
	cfg.Union.MemberKey = "test-member-key"

	srv, err := New(cfg, testLogger())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	return srv, hub
}

// signInboundRequestWithPEM signs a request body using the PEM-encoded private
// key and returns the signature headers.
func signInboundRequestWithPEM(t *testing.T, body, privPEM string) (sig, ts, nonce string) {
	t.Helper()

	block, _ := pem.Decode([]byte(privPEM))
	if block == nil {
		t.Fatal("failed to decode private key PEM")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse private key: %v", err)
	}

	sig, ts, nonce, err = union.SignInboundRequest(body, key)
	if err != nil {
		t.Fatalf("sign request: %v", err)
	}
	return sig, ts, nonce
}

func TestInboundRoutesHelloIsPublic(t *testing.T) {
	_, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/api/union/member/")
	if err != nil {
		t.Fatalf("get hello: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("hello status = %d, want 200: %s", resp.StatusCode, string(body))
	}
}

func TestInboundRoutesUnsignedRequestReturns401(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp, err := http.Post(testServer.URL+"/api/union/member/sync", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("post sync: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("sync status = %d, want 401: %s", resp.StatusCode, string(body))
	}

	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["detail"] == "" {
		t.Errorf("expected non-empty detail in 401 body")
	}

	_ = privPEM
}

func TestInboundRoutesSignedDiagnoseReturns200(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	body := `{"nonce":"test-nonce"}`
	sig, ts, nonce := signInboundRequestWithPEM(t, body, privPEM)

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/api/union/member/diagnose", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signatureHeader, sig)
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(nonceHeader, nonce)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post diagnose: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("diagnose status = %d, want 200: %s", resp.StatusCode, string(respBody))
	}
}

func TestInboundRoutesInvalidSignatureReturns401(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	body := `{"nonce":"test-nonce"}`
	_, ts, nonce := signInboundRequestWithPEM(t, body, privPEM)

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/api/union/member/diagnose", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signatureHeader, "dGVzdA==")
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(nonceHeader, nonce)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post diagnose: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("diagnose status = %d, want 401: %s", resp.StatusCode, string(respBody))
	}
}

func TestInboundRoutesExpiredTimestampReturns401(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	body := `{"nonce":"test-nonce"}`
	sig, _, nonce := signInboundRequestWithPEM(t, body, privPEM)

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/api/union/member/diagnose", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signatureHeader, sig)
	req.Header.Set(timestampHeader, "1")
	req.Header.Set(nonceHeader, nonce)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post diagnose: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("diagnose status = %d, want 401: %s", resp.StatusCode, string(respBody))
	}
}

func TestInboundRoutesExistingRoutesNotVerified(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/health")
	if err != nil {
		t.Fatalf("get health: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health status = %d, want 200", resp.StatusCode)
	}

	resp, err = http.Get(testServer.URL + "/")
	if err != nil {
		t.Fatalf("get root: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("root status = %d, want 200", resp.StatusCode)
	}

	_ = privPEM
}

func newTestServerWithHub(t *testing.T, hub *httptest.Server, logger *slog.Logger) *Server {
	t.Helper()

	cfg := testConfig("http://127.0.0.1:1")
	cfg.Storage.Path = filepath.Join(t.TempDir(), "store.db")
	cfg.Union.HubURL = hub.URL
	cfg.Union.MemberKey = "test-member-key"

	srv, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	return srv
}

func signedPost(t *testing.T, url, body, privPEM string) *http.Response {
	t.Helper()

	sig, ts, nonce := signInboundRequestWithPEM(t, body, privPEM)
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(signatureHeader, sig)
	req.Header.Set(timestampHeader, ts)
	req.Header.Set(nonceHeader, nonce)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func TestHelloReturnsVersionAndFeatures(t *testing.T) {
	_, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/api/union/member/")
	if err != nil {
		t.Fatalf("get hello: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("hello status = %d, want 200: %s", resp.StatusCode, string(body))
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want *", got)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["yggdrasilApiVersion"] != "union-svc/1.0" {
		t.Errorf("yggdrasilApiVersion = %q, want union-svc/1.0", body["yggdrasilApiVersion"])
	}
	if body["serverListVersion"] != float64(0) {
		t.Errorf("serverListVersion = %v, want 0", body["serverListVersion"])
	}
	if body["privateKeyVersion"] != float64(0) {
		t.Errorf("privateKeyVersion = %v, want 0", body["privateKeyVersion"])
	}
	features, ok := body["enabledFeatures"].([]any)
	if !ok || len(features) != 1 || features[0] != "unionBlacklist" {
		t.Errorf("enabledFeatures = %v, want [unionBlacklist]", body["enabledFeatures"])
	}
}

func TestHelloReturnsSeededVersions(t *testing.T) {
	_, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	ctx := context.Background()
	if err := srv.settingsStore().Set(ctx, "server_list_version", "3"); err != nil {
		t.Fatalf("seed server_list_version: %v", err)
	}
	if err := srv.settingsStore().Set(ctx, "private_key_version", "5"); err != nil {
		t.Fatalf("seed private_key_version: %v", err)
	}

	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp, err := http.Get(testServer.URL + "/api/union/member/")
	if err != nil {
		t.Fatalf("get hello: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("hello status = %d, want 200: %s", resp.StatusCode, string(body))
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["serverListVersion"] != float64(3) {
		t.Errorf("serverListVersion = %v, want 3", body["serverListVersion"])
	}
	if body["privateKeyVersion"] != float64(5) {
		t.Errorf("privateKeyVersion = %v, want 5", body["privateKeyVersion"])
	}
}

func TestDiagnoseEchoesNonceAndTimestamp(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	body := `{"nonce":"test-nonce"}`
	resp := signedPost(t, testServer.URL+"/api/union/member/diagnose", body, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("diagnose status = %d, want 200: %s", resp.StatusCode, string(respBody))
	}

	var got map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got["nonce"] != "test-nonce" {
		t.Errorf("nonce = %q, want test-nonce", got["nonce"])
	}
	ts, ok := got["timestamp"].(float64)
	if !ok {
		t.Fatalf("timestamp = %v, want float64", got["timestamp"])
	}
	if ts <= 0 {
		t.Errorf("timestamp = %v, want positive value", ts)
	}
}

func TestUpdateBackendKeyStoresMemberKey(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	srv, _ := setupInboundTestServer(t, pubPEM)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	body := `{"key":"new-member-key"}`
	resp := signedPost(t, testServer.URL+"/api/union/member/updatebackendkey", body, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("updatebackendkey status = %d, want 200: %s", resp.StatusCode, string(respBody))
	}

	got, err := srv.settingsStore().Get(context.Background(), "member_key")
	if err != nil {
		t.Fatalf("get member_key: %v", err)
	}
	if got != "new-member-key" {
		t.Errorf("member_key = %q, want new-member-key", got)
	}
}

func TestUpdateListStoresServerListAndVersion(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"union_host_signature_public_key": pubPEM,
			})
		case "/serverlist":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"servers":[{"name":"hub"}],"version":3}`))
		default:
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
	}))
	defer hub.Close()

	srv := newTestServerWithHub(t, hub, testLogger())
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp := signedPost(t, testServer.URL+"/api/union/member/updatelist", `{}`, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("updatelist status = %d, want 200: %s", resp.StatusCode, string(respBody))
	}

	ctx := context.Background()
	list, err := srv.settingsStore().Get(ctx, "server_list")
	if err != nil {
		t.Fatalf("get server_list: %v", err)
	}
	if list != `[{"name":"hub"}]` {
		t.Errorf("server_list = %q, want [{\"name\":\"hub\"}]", list)
	}
	version, err := srv.settingsStore().Get(ctx, "server_list_version")
	if err != nil {
		t.Fatalf("get server_list_version: %v", err)
	}
	if version != "3" {
		t.Errorf("server_list_version = %q, want 3", version)
	}
}

func TestUpdateListPassesThroughHubError(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"union_host_signature_public_key": pubPEM,
			})
		case "/serverlist":
			w.WriteHeader(http.StatusForbidden)
		default:
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
	}))
	defer hub.Close()

	srv := newTestServerWithHub(t, hub, testLogger())
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp := signedPost(t, testServer.URL+"/api/union/member/updatelist", `{}`, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("updatelist status = %d, want 403: %s", resp.StatusCode, string(respBody))
	}
}

func TestUpdatePrivateKeyStoresVersionAndNotPEM(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"union_host_signature_public_key": pubPEM,
			})
		case "/privatekey":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"privateKey":"-----BEGIN RSA PRIVATE KEY-----","privateKeyVersion":5}`))
		default:
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
	}))
	defer hub.Close()

	srv := newTestServerWithHub(t, hub, logger)
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp := signedPost(t, testServer.URL+"/api/union/member/updateprivatekey", `{}`, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("updateprivatekey status = %d, want 200: %s", resp.StatusCode, string(respBody))
	}

	ctx := context.Background()
	version, err := srv.settingsStore().Get(ctx, "private_key_version")
	if err != nil {
		t.Fatalf("get private_key_version: %v", err)
	}
	if version != "5" {
		t.Errorf("private_key_version = %q, want 5", version)
	}

	pem, err := srv.settingsStore().Get(ctx, "private_key")
	if err != nil {
		t.Fatalf("get private_key: %v", err)
	}
	if pem != "" {
		t.Errorf("private_key must not be stored, got %q", pem)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "Union private key updated to version 5") {
		t.Errorf("log does not contain version update message: %q", logs)
	}
	if !strings.Contains(logs, "Admin must manually replace PEM in skin-backend") {
		t.Errorf("log does not contain manual replacement reminder: %q", logs)
	}
}

func TestUpdatePrivateKeyPassesThroughHubError(t *testing.T) {
	privPEM, pubPEM, err := union.GenerateRSAKeyPair()
	if err != nil {
		t.Fatalf("generate key pair: %v", err)
	}

	hub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			_ = json.NewEncoder(w).Encode(map[string]string{
				"union_host_signature_public_key": pubPEM,
			})
		case "/privatekey":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected hub path %s", r.URL.Path)
		}
	}))
	defer hub.Close()

	srv := newTestServerWithHub(t, hub, testLogger())
	testServer := httptest.NewServer(srv.Handler())
	defer testServer.Close()

	resp := signedPost(t, testServer.URL+"/api/union/member/updateprivatekey", `{}`, privPEM)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("updateprivatekey status = %d, want 500: %s", resp.StatusCode, string(respBody))
	}

	ctx := context.Background()
	version, err := srv.settingsStore().Get(ctx, "private_key_version")
	if err != nil {
		t.Fatalf("get private_key_version: %v", err)
	}
	if version != "" {
		t.Errorf("private_key_version = %q, want empty on Hub error", version)
	}
}

