package config

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoadParsesNestedYAMLConfig(t *testing.T) {
	clearConfigEnv(t)
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
mojang:
  skin_domains:
    - "textures.minecraft.net"
    - "cdn.example"
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
	if !reflect.DeepEqual(cfg.FallbackDomains, []string{"textures.minecraft.net", "cdn.example"}) {
		t.Fatalf("fallback skin domains not parsed: %#v", cfg.FallbackDomains)
	}
}

func TestLoadEnvOverridesFileAndPersistsExactYAMLValues(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(`
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
  expire_days: 3
  access_expire_minutes: 10
keys:
  private_key: "file-private.pem"
  public_key: "file-public.pem"
database:
  dsn: "postgresql://file"
  max_connections: 7
server:
  site_url: "https://file.example"
  api_url: "https://file.example/api"
  host: "127.0.0.1"
  port: 9000
textures:
  directory: "/file/textures"
carousel:
  directory: "/file/carousel"
redis:
  public_cache_ttl_seconds: 12
  auth_cache_ttl_seconds: 13
mojang:
  skin_domains:
    - "file.example"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("JWT_EXPIRE_DAYS", "8")
	t.Setenv("JWT_ACCESS_EXPIRE_MINUTES", "35")
	t.Setenv("KEYS_PRIVATE_KEY", "env-private.pem")
	t.Setenv("KEYS_PUBLIC_KEY", "/abs/env-public.pem")
	t.Setenv("DATABASE_DSN", "postgresql://env")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "31")
	t.Setenv("SERVER_SITE_URL", "https://env.example")
	t.Setenv("SERVER_API_URL", "https://env.example/api")
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "8100")
	t.Setenv("TEXTURES_DIRECTORY", "/env/textures")
	t.Setenv("CAROUSEL_DIRECTORY", "/env/carousel")
	t.Setenv("REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("REDIS_PASSWORD", "env-redis-password")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_KEY_PREFIX", "envprefix:")
	t.Setenv("REDIS_PUBLIC_CACHE_TTL_SECONDS", "220")
	t.Setenv("REDIS_AUTH_CACHE_TTL_SECONDS", "25")
	t.Setenv("MOJANG_SKIN_DOMAINS", "textures.minecraft.net, cdn.env.example")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.JWTSecret != "env-secret-abcdefghijklmnopqrstuvwxyz" {
		t.Fatalf("JWT_SECRET env should override file, got %q", cfg.JWTSecret)
	}
	if cfg.JWTExpireDays != 8 || cfg.AccessMinutes != 35 {
		t.Fatalf("jwt env should override file, got %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "env-private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "abs", "env-public.pem") {
		t.Fatalf("key path env should override and resolve exactly, got %#v", cfg)
	}
	if cfg.DatabaseDSN != "postgresql://env" {
		t.Fatalf("DATABASE_DSN env should override file, got %q", cfg.DatabaseDSN)
	}
	if cfg.MaxConnections != 31 || cfg.SiteURL != "https://env.example" || cfg.APIURL != "https://env.example/api" ||
		cfg.ServerHost != "0.0.0.0" || cfg.ServerPort != "8100" || cfg.TexturesDir != "/env/textures" || cfg.CarouselDir != "/env/carousel" {
		t.Fatalf("env should override file/defaults: %#v", cfg)
	}
	if cfg.RedisAddr != "127.0.0.1:6380" || cfg.RedisPassword != "env-redis-password" || cfg.RedisDB != 3 || cfg.RedisKeyPrefix != "envprefix:" ||
		cfg.PublicCacheTTL != 220 || cfg.AuthCacheTTL != 25 {
		t.Fatalf("redis env should override file/defaults: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.FallbackDomains, []string{"textures.minecraft.net", "cdn.env.example"}) {
		t.Fatalf("mojang skin domains env mismatch: %#v", cfg.FallbackDomains)
	}

	var persisted rawConfig
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &persisted); err != nil {
		t.Fatal(err)
	}
	assertRawValue(t, persisted, "jwt.secret", "env-secret-abcdefghijklmnopqrstuvwxyz")
	assertRawValue(t, persisted, "jwt.expire_days", 8)
	assertRawValue(t, persisted, "keys.private_key", "env-private.pem")
	assertRawValue(t, persisted, "database.max_connections", 31)
	assertRawValue(t, persisted, "server.port", 8100)
	assertRawValue(t, persisted, "redis.public_cache_ttl_seconds", 220)
	if got, _ := lookup(persisted, "mojang.skin_domains"); !reflect.DeepEqual(got, []any{"textures.minecraft.net", "cdn.env.example"}) {
		t.Fatalf("persisted mojang.skin_domains=%#v", got)
	}
}

func TestLoadMissingFileCreatesExactDefaultConfig(t *testing.T) {
	clearConfigEnv(t)
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	got, err := Load(missingPath)
	if err != nil {
		t.Fatalf("missing config should use defaults: %v", err)
	}
	want := Defaults()
	want.PrivateKeyPath = filepath.Join(filepath.Dir(missingPath), want.PrivateKeyPath)
	want.PublicKeyPath = filepath.Join(filepath.Dir(missingPath), want.PublicKeyPath)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missing config mismatch:\n got: %#v\nwant: %#v", got, want)
	}
	if _, err := os.Stat(missingPath); err != nil {
		t.Fatalf("missing config should be created: %v", err)
	}
	var persisted rawConfig
	b, err := os.ReadFile(missingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &persisted); err != nil {
		t.Fatal(err)
	}
	assertRawValue(t, persisted, "database.dsn", want.DatabaseDSN)
	assertRawValue(t, persisted, "server.port", 8000)
}

func TestLoadMalformedFilePreservesExactDefaultsAndDoesNotRewrite(t *testing.T) {
	clearConfigEnv(t)
	want := Defaults()
	malformedPath := filepath.Join(t.TempDir(), "malformed.yaml")
	malformed := []byte("jwt:\n  secret: [unterminated")
	if err := os.WriteFile(malformedPath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(malformedPath)
	if err == nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("malformed config result: cfg=%#v err=%v; want exact defaults plus YAML error", got, err)
	}
	after, readErr := os.ReadFile(malformedPath)
	if readErr != nil || string(after) != string(malformed) {
		t.Fatalf("malformed config should not be rewritten: readErr=%v after=%q", readErr, string(after))
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
	t.Setenv("DATABASE_MAX_CONNECTIONS", "-1")
	t.Setenv("SERVER_PORT", "not-a-port")
	t.Setenv("REDIS_PUBLIC_CACHE_TTL_SECONDS", "0")

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
	clearConfigEnv(t)
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
	setRaw(raw, "created.child.value", "ok")
	if value, ok := lookup(raw, "created.child.value"); !ok || value != "ok" {
		t.Fatalf("setRaw created value=%#v,%v; want ok,true", value, ok)
	}
}

func TestLoadWithoutEnvironmentDoesNotRewriteExistingFile(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := "jwt:\n  secret: \"file-secret-abcdefghijklmnopqrstuvwxyz\"\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != original {
		t.Fatalf("config without env overrides should not be rewritten:\n got %q\nwant %q", string(after), original)
	}
}

func assertRawValue(t *testing.T, raw rawConfig, dotted string, want any) {
	t.Helper()
	got, ok := lookup(raw, dotted)
	if !ok || !reflect.DeepEqual(got, want) {
		t.Fatalf("raw %s=%#v,%v; want %#v,true", dotted, got, ok, want)
	}
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_DSN",
		"DATABASE_MAX_CONNECTIONS",
		"JWT_SECRET",
		"JWT_EXPIRE_DAYS",
		"JWT_ACCESS_EXPIRE_MINUTES",
		"KEYS_PRIVATE_KEY",
		"KEYS_PUBLIC_KEY",
		"SERVER_SITE_URL",
		"SERVER_API_URL",
		"SERVER_HOST",
		"SERVER_PORT",
		"TEXTURES_DIRECTORY",
		"CAROUSEL_DIRECTORY",
		"REDIS_ADDR",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_KEY_PREFIX",
		"REDIS_PUBLIC_CACHE_TTL_SECONDS",
		"REDIS_AUTH_CACHE_TTL_SECONDS",
		"MOJANG_SKIN_DOMAINS",
	} {
		t.Setenv(key, "")
	}
}
