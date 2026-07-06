package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaults verifies that Load without any file or env uses the expected
// default values.
func TestDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load empty path: %v", err)
	}

	if cfg.Server.Addr != "" {
		t.Errorf("Server.Addr = %q, want empty", cfg.Server.Addr)
	}
	if cfg.Server.Port != 8001 {
		t.Errorf("Server.Port = %d, want 8001", cfg.Server.Port)
	}
	if cfg.Elementskin.BaseURL != "http://127.0.0.1:8000" {
		t.Errorf("Elementskin.BaseURL = %q, want http://127.0.0.1:8000", cfg.Elementskin.BaseURL)
	}
	if cfg.Elementskin.OAuth.RedirectURI != "http://127.0.0.1:8001/oauth/callback" {
		t.Errorf("Elementskin.OAuth.RedirectURI = %q, want http://127.0.0.1:8001/oauth/callback", cfg.Elementskin.OAuth.RedirectURI)
	}
	if cfg.Elementskin.ServiceAccount.Scope != "profile.read.any" {
		t.Errorf("Elementskin.ServiceAccount.Scope = %q, want profile.read.any", cfg.Elementskin.ServiceAccount.Scope)
	}
	if cfg.Storage.Path != "./union-svc.db" {
		t.Errorf("Storage.Path = %q, want ./union-svc.db", cfg.Storage.Path)
	}
	if cfg.Union.TimeoutSeconds != 30 {
		t.Errorf("Union.TimeoutSeconds = %d, want 30", cfg.Union.TimeoutSeconds)
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want info", cfg.Log.Level)
	}
}

// TestLoadFromYAML verifies that a YAML file overrides default values.
func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	yamlContent := `
server:
  addr: "0.0.0.0"
  port: 9090
elementskin:
  base_url: "https://elementskin.example.com"
  oauth:
    client_id: "yaml-client"
    client_secret: "yaml-secret"
storage:
  path: "/data/custom.db"
union:
  hub_url: "https://hub.example.com"
  member_key: "yaml-key"
  timeout_seconds: 60
log:
  level: "debug"
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load yaml: %v", err)
	}

	if cfg.Server.Addr != "0.0.0.0" {
		t.Errorf("Server.Addr = %q, want 0.0.0.0", cfg.Server.Addr)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090", cfg.Server.Port)
	}
	if cfg.Elementskin.BaseURL != "https://elementskin.example.com" {
		t.Errorf("Elementskin.BaseURL = %q, want https://elementskin.example.com", cfg.Elementskin.BaseURL)
	}
	if cfg.Elementskin.OAuth.ClientID != "yaml-client" {
		t.Errorf("Elementskin.OAuth.ClientID = %q, want yaml-client", cfg.Elementskin.OAuth.ClientID)
	}
	if cfg.Elementskin.OAuth.ClientSecret != "yaml-secret" {
		t.Errorf("Elementskin.OAuth.ClientSecret = %q, want yaml-secret", cfg.Elementskin.OAuth.ClientSecret)
	}
	if cfg.Storage.Path != "/data/custom.db" {
		t.Errorf("Storage.Path = %q, want /data/custom.db", cfg.Storage.Path)
	}
	if cfg.Union.HubURL != "https://hub.example.com" {
		t.Errorf("Union.HubURL = %q, want https://hub.example.com", cfg.Union.HubURL)
	}
	if cfg.Union.MemberKey != "yaml-key" {
		t.Errorf("Union.MemberKey = %q, want yaml-key", cfg.Union.MemberKey)
	}
	if cfg.Union.TimeoutSeconds != 60 {
		t.Errorf("Union.TimeoutSeconds = %d, want 60", cfg.Union.TimeoutSeconds)
	}
	if cfg.Log.Level != "debug" {
		t.Errorf("Log.Level = %q, want debug", cfg.Log.Level)
	}
}

// TestEnvOverride verifies that UNION_* environment variables override both
// defaults and YAML values.
func TestEnvOverride(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	// Write a YAML that sets port, elementskin base_url, and timeout.
	yamlContent := `
server:
  port: 9999
elementskin:
  base_url: "http://from-yaml:8000"
  service_account:
    client_id: "yaml-svc-id"
    scope: "profile.read.owned"
union:
  timeout_seconds: 10
`
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	// Set env vars that should override YAML values.
	t.Setenv("UNION_SERVER_PORT", "1234")
	t.Setenv("UNION_ELEMENTSKIN_BASE_URL", "http://from-env:9000")
	t.Setenv("UNION_ELEMENTSKIN_SERVICE_ACCOUNT_CLIENT_ID", "env-svc-id")
	t.Setenv("UNION_ELEMENTSKIN_SERVICE_ACCOUNT_CLIENT_SECRET", "env-svc-secret")
	t.Setenv("UNION_ELEMENTSKIN_SERVICE_ACCOUNT_SCOPE", "profile.write.any")
	t.Setenv("UNION_UNION_HUB_URL", "https://hub-from-env.example.com")
	t.Setenv("UNION_LOG_LEVEL", "error")

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load with env: %v", err)
	}

	// Env overrides YAML.
	if cfg.Server.Port != 1234 {
		t.Errorf("Server.Port = %d, want 1234 (from env)", cfg.Server.Port)
	}
	if cfg.Elementskin.BaseURL != "http://from-env:9000" {
		t.Errorf("Elementskin.BaseURL = %q, want http://from-env:9000", cfg.Elementskin.BaseURL)
	}
	if cfg.Elementskin.ServiceAccount.ClientID != "env-svc-id" {
		t.Errorf("Elementskin.ServiceAccount.ClientID = %q, want env-svc-id", cfg.Elementskin.ServiceAccount.ClientID)
	}
	if cfg.Elementskin.ServiceAccount.ClientSecret != "env-svc-secret" {
		t.Errorf("Elementskin.ServiceAccount.ClientSecret = %q, want env-svc-secret", cfg.Elementskin.ServiceAccount.ClientSecret)
	}
	if cfg.Elementskin.ServiceAccount.Scope != "profile.write.any" {
		t.Errorf("Elementskin.ServiceAccount.Scope = %q, want profile.write.any", cfg.Elementskin.ServiceAccount.Scope)
	}
	if cfg.Union.HubURL != "https://hub-from-env.example.com" {
		t.Errorf("Union.HubURL = %q, want https://hub-from-env.example.com", cfg.Union.HubURL)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("Log.Level = %q, want error", cfg.Log.Level)
	}

	// Values from YAML that were NOT overridden by env should survive.
	if cfg.Union.TimeoutSeconds != 10 {
		t.Errorf("Union.TimeoutSeconds = %d, want 10 (from yaml, unchanged)", cfg.Union.TimeoutSeconds)
	}

	// Defaults that were not touched by YAML or env should remain.
	if cfg.Server.Addr != "" {
		t.Errorf("Server.Addr = %q, want empty (default)", cfg.Server.Addr)
	}
	if cfg.Elementskin.OAuth.RedirectURI != "http://127.0.0.1:8001/oauth/callback" {
		t.Errorf("Elementskin.OAuth.RedirectURI = %q, want default", cfg.Elementskin.OAuth.RedirectURI)
	}
	if cfg.Storage.Path != "./union-svc.db" {
		t.Errorf("Storage.Path = %q, want default", cfg.Storage.Path)
	}
}

// TestListenAddr verifies ListenAddr produces the expected host:port string.
func TestListenAddr(t *testing.T) {
	tests := []struct {
		name string
		addr string
		port int
		want string
	}{
		{"empty addr uses loopback", "", 8001, "127.0.0.1:8001"},
		{"specific addr", "0.0.0.0", 3000, "0.0.0.0:3000"},
		{"non-standard port", "192.168.1.1", 9000, "192.168.1.1:9000"},
		{"ipv6 localhost", "::1", 8001, "::1:8001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg Config
			cfg.Server.Addr = tt.addr
			cfg.Server.Port = tt.port
			if got := cfg.ListenAddr(); got != tt.want {
				t.Errorf("ListenAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNonExistentFileDoesNotError verifies that Load does not fail when the
// config file path does not exist — it falls through to defaults + env.
func TestNonExistentFileDoesNotError(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("Load with nonexistent file should not error, got: %v", err)
	}
}
