package main

import (
	"net/http"
	"testing"
	"time"

	"element-skin/backend/internal/config"
)

func TestNewHTTPServerUsesConfiguredAddressHandlerAndTimeout(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	cfg := config.Defaults()
	cfg.ServerHost = "127.0.0.1"
	cfg.ServerPort = "18080"

	server := newHTTPServer(cfg, handler)

	if server.Addr != "127.0.0.1:18080" {
		t.Fatalf("server address should come from config, got %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("server handler should be installed")
	}
	if server.ReadHeaderTimeout != 10*time.Second {
		t.Fatalf("unexpected read header timeout: %s", server.ReadHeaderTimeout)
	}
}
