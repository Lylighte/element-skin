package server

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"io"
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
