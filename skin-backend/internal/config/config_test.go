package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadParsesNestedYAMLConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "abcdefghijklmnopqrstuvwxyz1234567890"
  expire_days: 11
  access_expire_minutes: 45
keys:
  private_key: "keys/private.pem"
  public_key: "keys/public.pem"
database:
  dsn: "postgresql://user:pass@localhost:5432/db?sslmode=disable"
  max_connections: 23
server:
  site_url: "https://skin.example.com"
  api_url: "https://skin.example.com/api"
  host: "127.0.0.1"
  port: 9001
textures:
  directory: "/data/textures"
carousel:
  directory: "/data/carousel"
redis:
  addr: "redis:6379"
  password: "password123"
  db: 2
  key_prefix: "custom:"
  public_cache_ttl_seconds: 120
  auth_cache_ttl_seconds: 15
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "abcdefghijklmnopqrstuvwxyz1234567890" {
		t.Fatalf("JWTSecret was not loaded from nested YAML: %#v", cfg)
	}
	if cfg.JWTExpireDays != 11 || cfg.AccessMinutes != 45 {
		t.Fatalf("JWT expiry fields not parsed: %#v", cfg)
	}
	if cfg.DatabaseDSN != "postgresql://user:pass@localhost:5432/db?sslmode=disable" || cfg.MaxConnections != 23 {
		t.Fatalf("database fields not parsed: %#v", cfg)
	}
	if cfg.SiteURL != "https://skin.example.com" || cfg.APIURL != "https://skin.example.com/api" || cfg.ServerHost != "127.0.0.1" || cfg.ServerPort != "9001" {
		t.Fatalf("server fields not parsed: %#v", cfg)
	}
	if cfg.TexturesDir != "/data/textures" || cfg.CarouselDir != "/data/carousel" {
		t.Fatalf("storage directories not parsed: %#v", cfg)
	}
	if cfg.RedisAddr != "redis:6379" || cfg.RedisPassword != "password123" || cfg.RedisDB != 2 || cfg.RedisKeyPrefix != "custom:" {
		t.Fatalf("redis fields not parsed: %#v", cfg)
	}
	if cfg.PublicCacheTTL != 120 || cfg.AuthCacheTTL != 15 {
		t.Fatalf("redis ttl fields not parsed: %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "keys", "private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "keys", "public.pem") {
		t.Fatalf("key paths should resolve relative to config file: %#v", cfg)
	}
}

func TestLoadEnvOverridesFileSecrets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
database:
  dsn: "postgresql://file"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("DATABASE_DSN", "postgresql://env")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("REDIS_PASSWORD", "env-redis-password")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_KEY_PREFIX", "envprefix:")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "env-secret-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("JWT_SECRET env should override file, got %q", cfg.JWTSecret)
	}
	if cfg.DatabaseDSN != "postgresql://env" {
		t.Fatalf("DATABASE_DSN env should override file, got %q", cfg.DatabaseDSN)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" || cfg.RedisPassword != "env-redis-password" || cfg.RedisDB != 3 || cfg.RedisKeyPrefix != "envprefix:" {
		t.Fatalf("redis env should override file/defaults: %#v", cfg)
	}
}

func TestLoadMissingAndMalformedFilesPreserveExactDefaults(t *testing.T) {
	clearConfigEnv(t)
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	got, err := Load(missingPath)
	if err != nil {
		t.Fatalf("missing config should use defaults: %v", err)
	}
	want := Defaults()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing config mismatch:\n got: %#v\nwant: %#v", got, want)
	}

	malformedPath := filepath.Join(t.TempDir(), "malformed.yaml")
	if err := os.WriteFile(malformedPath, []byte("jwt:\n  secret: [unterminated"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = Load(malformedPath)
	if err == nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("malformed config result: cfg=%#v err=%v; want exact defaults plus YAML error", got, err)
	}
}

func TestLoadRejectsInvalidNumericOverridesWithoutChangingSafeValues(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	absolutePublic := filepath.Join(dir, "absolute-public.pem")
	raw := fmt.Sprintf(`
database:
  max_connections: 0
server:
  port: "invalid"
redis:
  db: -1
  public_cache_ttl_seconds: 0
  auth_cache_ttl_seconds: -5
keys:
  private_key: ""
  public_key: %q
`, filepath.ToSlash(absolutePublic))
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REDIS_DB", "not-an-int")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	defaults := Defaults()
	if cfg.MaxConnections != defaults.MaxConnections ||
		cfg.ServerPort != defaults.ServerPort ||
		cfg.RedisDB != defaults.RedisDB ||
		cfg.PublicCacheTTL != defaults.PublicCacheTTL ||
		cfg.AuthCacheTTL != defaults.AuthCacheTTL ||
		cfg.PrivateKeyPath != "" ||
		filepath.Clean(cfg.PublicKeyPath) != filepath.Clean(absolutePublic) {
		t.Fatalf("invalid numeric/path fallback mismatch: %#v", cfg)
	}
}

func TestConfigLookupAndIntegerCoercionExactValues(t *testing.T) {
	raw := rawConfig{
		"nested": map[string]any{
			"int":     3,
			"int64":   int64(4),
			"float64": float64(5.9),
			"string":  "6",
			"bad":     true,
			"text":    "value",
			"nil":     nil,
		},
	}
	for _, tc := range []struct {
		key  string
		want int
	}{
		{key: "nested.int", want: 3},
		{key: "nested.int64", want: 4},
		{key: "nested.float64", want: 5},
		{key: "nested.string", want: 6},
		{key: "nested.bad", want: 9},
		{key: "nested.missing", want: 9},
		{key: "nested.nil", want: 9},
		{key: "nested.text.child", want: 9},
	} {
		if got := getInt(raw, tc.key, 9); got != tc.want {
			t.Fatalf("getInt(%q)=%d; want %d", tc.key, got, tc.want)
		}
	}
	if got := getString(raw, "nested.text", "fallback"); got != "value" {
		t.Fatalf("getString text=%q; want value", got)
	}
	if got := getString(raw, "nested.int", "fallback"); got != "fallback" {
		t.Fatalf("getString non-string=%q; want fallback", got)
	}
	if value, ok := lookup(raw, "nested.text"); !ok || value != "value" {
		t.Fatalf("lookup existing=%#v,%v; want value,true", value, ok)
	}
	if value, ok := lookup(raw, "nested.missing"); ok || value != nil {
		t.Fatalf("lookup missing=%#v,%v; want nil,false", value, ok)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_DSN",
		"JWT_SECRET",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_KEY_PREFIX",
	} {
		t.Setenv(key, "")
	}
}
