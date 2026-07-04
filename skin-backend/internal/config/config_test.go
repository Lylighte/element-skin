package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
  host: "localhost"
  port: "5432"
  user: "user"
  password: "pass"
  name: "db"
  sslmode: "disable"
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
  host: "redis"
  port: "6379"
  password: "password123"
  db: 2
  key_prefix: "custom:"
  public_cache_ttl_seconds: 120
  auth_cache_ttl_seconds: 15
cors:
  allow_origins:
    - "https://skin.example.com"
    - "http://localhost:5173"
  allow_credentials: false
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
	if cfg.DatabaseHost != "localhost" || cfg.DatabasePort != "5432" || cfg.DatabaseUser != "user" ||
		cfg.DatabasePassword != "pass" || cfg.DatabaseName != "db" || cfg.DatabaseSSLMode != "disable" ||
		cfg.DatabaseDSN != "postgresql://user:pass@localhost:5432/db?sslmode=disable" || cfg.MaxConnections != 23 {
		t.Fatalf("database fields not parsed: %#v", cfg)
	}
	if cfg.SiteURL != "https://skin.example.com" || cfg.APIURL != "https://skin.example.com/api" || cfg.ServerHost != "127.0.0.1" || cfg.ServerPort != "9001" {
		t.Fatalf("server fields not parsed: %#v", cfg)
	}
	if cfg.TexturesDir != "/data/textures" || cfg.CarouselDir != "/data/carousel" {
		t.Fatalf("storage directories not parsed: %#v", cfg)
	}
	if cfg.RedisHost != "redis" || cfg.RedisPort != "6379" || cfg.RedisAddr != "redis:6379" ||
		cfg.RedisPassword != "password123" || cfg.RedisDB != 2 || cfg.RedisKeyPrefix != "custom:" {
		t.Fatalf("redis fields not parsed: %#v", cfg)
	}
	if cfg.PublicCacheTTL != 120 || cfg.AuthCacheTTL != 15 {
		t.Fatalf("redis ttl fields not parsed: %#v", cfg)
	}
	if cfg.PrivateKeyPath != filepath.Join(dir, "keys", "private.pem") || cfg.PublicKeyPath != filepath.Join(dir, "keys", "public.pem") {
		t.Fatalf("key paths should resolve relative to config file: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.CORSOrigins, []string{"https://skin.example.com", "http://localhost:5173"}) || cfg.CORSCredentials {
		t.Fatalf("cors fields not parsed: origins=%#v credentials=%v", cfg.CORSOrigins, cfg.CORSCredentials)
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
  host: "file-db"
  port: "5432"
  user: "file-user"
  password: "file-password"
  name: "file-db-name"
  sslmode: "disable"
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
  host: "file-redis"
  port: "6379"
  password: "file-redis-password"
  public_cache_ttl_seconds: 12
  auth_cache_ttl_seconds: 13
cors:
  allow_origins:
    - "https://file.example"
  allow_credentials: false
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("JWT_EXPIRE_DAYS", "8")
	t.Setenv("JWT_ACCESS_EXPIRE_MINUTES", "35")
	t.Setenv("KEYS_PRIVATE_KEY", "env-private.pem")
	t.Setenv("KEYS_PUBLIC_KEY", "/abs/env-public.pem")
	t.Setenv("DATABASE_HOST", "env-db")
	t.Setenv("DATABASE_PORT", "6543")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-password")
	t.Setenv("DATABASE_NAME", "env-db-name")
	t.Setenv("DATABASE_SSLMODE", "require")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "31")
	t.Setenv("SERVER_SITE_URL", "https://env.example")
	t.Setenv("SERVER_API_URL", "https://env.example/api")
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "8100")
	t.Setenv("TEXTURES_DIRECTORY", "/env/textures")
	t.Setenv("CAROUSEL_DIRECTORY", "/env/carousel")
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "env-redis-password")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_KEY_PREFIX", "envprefix:")
	t.Setenv("REDIS_PUBLIC_CACHE_TTL_SECONDS", "220")
	t.Setenv("REDIS_AUTH_CACHE_TTL_SECONDS", "25")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://env.example, http://localhost:5173")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "true")

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
	if cfg.DatabaseHost != "env-db" || cfg.DatabasePort != "6543" || cfg.DatabaseUser != "env-user" ||
		cfg.DatabasePassword != "env-password" || cfg.DatabaseName != "env-db-name" || cfg.DatabaseSSLMode != "require" ||
		cfg.DatabaseDSN != "postgresql://env-user:env-password@env-db:6543/env-db-name?sslmode=require" {
		t.Fatalf("database env should override file and derive DSN, got %#v", cfg)
	}
	if cfg.MaxConnections != 31 || cfg.SiteURL != "https://env.example" || cfg.APIURL != "https://env.example/api" ||
		cfg.ServerHost != "0.0.0.0" || cfg.ServerPort != "8100" || cfg.TexturesDir != "/env/textures" || cfg.CarouselDir != "/env/carousel" {
		t.Fatalf("env should override file/defaults: %#v", cfg)
	}
	if cfg.RedisHost != "127.0.0.1" || cfg.RedisPort != "6380" || cfg.RedisAddr != "127.0.0.1:6380" ||
		cfg.RedisPassword != "env-redis-password" || cfg.RedisDB != 3 || cfg.RedisKeyPrefix != "envprefix:" ||
		cfg.PublicCacheTTL != 220 || cfg.AuthCacheTTL != 25 {
		t.Fatalf("redis env should override file/defaults: %#v", cfg)
	}
	if !reflect.DeepEqual(cfg.CORSOrigins, []string{"https://env.example", "http://localhost:5173"}) || !cfg.CORSCredentials {
		t.Fatalf("cors env mismatch: origins=%#v credentials=%v", cfg.CORSOrigins, cfg.CORSCredentials)
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
	assertRawValue(t, persisted, "database.host", "env-db")
	assertRawValue(t, persisted, "database.port", "6543")
	assertRawValue(t, persisted, "database.user", "env-user")
	assertRawValue(t, persisted, "database.password", "env-password")
	assertRawValue(t, persisted, "database.name", "env-db-name")
	assertRawValue(t, persisted, "database.sslmode", "require")
	assertRawValue(t, persisted, "database.max_connections", 31)
	assertRawValue(t, persisted, "server.port", 8100)
	assertRawValue(t, persisted, "redis.host", "127.0.0.1")
	assertRawValue(t, persisted, "redis.port", "6380")
	assertRawValue(t, persisted, "redis.public_cache_ttl_seconds", 220)
	assertRawValue(t, persisted, "cors.allow_credentials", true)
	if got, _ := lookup(persisted, "cors.allow_origins"); !reflect.DeepEqual(got, []any{"https://env.example", "http://localhost:5173"}) {
		t.Fatalf("persisted cors.allow_origins=%#v", got)
	}
}

func TestLoadMissingFileWithoutCompleteEnvironmentFailsAndDoesNotWrite(t *testing.T) {
	clearConfigEnv(t)
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	cfg, err := Load(missingPath)
	if err == nil || !strings.HasPrefix(err.Error(), "missing required config fields: ") {
		t.Fatalf("missing config error=%v cfg=%#v; want missing required fields", err, cfg)
	}
	if !reflect.DeepEqual(cfg, Config{}) {
		t.Fatalf("missing incomplete config should return zero config, got %#v", cfg)
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("incomplete env should not create config file, stat err=%v", err)
	}
}

func TestLoadMissingFileWithCompleteEnvironmentCreatesExactConfig(t *testing.T) {
	clearConfigEnv(t)
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "generated.yaml")
	setCompleteConfigEnv(t)

	cfg, err := Load(missingPath)
	if err != nil {
		t.Fatalf("complete environment should create config: %v", err)
	}
	want := Config{
		DatabaseDSN:      "postgresql://env-user:env-password@env-db:6543/env-db-name?sslmode=require",
		DatabaseHost:     "env-db",
		DatabasePort:     "6543",
		DatabaseUser:     "env-user",
		DatabasePassword: "env-password",
		DatabaseName:     "env-db-name",
		DatabaseSSLMode:  "require",
		MaxConnections:   31,
		JWTSecret:        "env-secret-abcdefghijklmnopqrstuvwxyz",
		JWTExpireDays:    8,
		AccessMinutes:    35,
		SiteURL:          "https://env.example",
		APIURL:           "https://env.example/api",
		ServerHost:       "0.0.0.0",
		ServerPort:       "8100",
		TexturesDir:      "/env/textures",
		CarouselDir:      "/env/carousel",
		RedisAddr:        "127.0.0.1:6380",
		RedisHost:        "127.0.0.1",
		RedisPort:        "6380",
		RedisPassword:    "env-redis-password",
		RedisDB:          3,
		RedisKeyPrefix:   "envprefix:",
		PublicCacheTTL:   220,
		AuthCacheTTL:     25,
		PrivateKeyPath:   filepath.Join(dir, "env-private.pem"),
		PublicKeyPath:    filepath.Join(dir, "abs", "env-public.pem"),
		CORSOrigins:      []string{"https://env.example", "http://localhost:5173"},
		CORSCredentials:  false,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Fatalf("generated config mismatch:\n got: %#v\nwant: %#v", cfg, want)
	}
	var persisted rawConfig
	b, err := os.ReadFile(missingPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(b, &persisted); err != nil {
		t.Fatal(err)
	}
	assertRawValue(t, persisted, "jwt.secret", "env-secret-abcdefghijklmnopqrstuvwxyz")
	assertRawValue(t, persisted, "jwt.expire_days", 8)
	assertRawValue(t, persisted, "jwt.access_expire_minutes", 35)
	assertRawValue(t, persisted, "database.host", "env-db")
	assertRawValue(t, persisted, "database.port", "6543")
	assertRawValue(t, persisted, "database.user", "env-user")
	assertRawValue(t, persisted, "database.password", "env-password")
	assertRawValue(t, persisted, "database.name", "env-db-name")
	assertRawValue(t, persisted, "database.sslmode", "require")
	assertRawValue(t, persisted, "database.max_connections", 31)
	assertRawValue(t, persisted, "server.port", 8100)
	assertRawValue(t, persisted, "redis.host", "127.0.0.1")
	assertRawValue(t, persisted, "redis.port", "6380")
	assertRawValue(t, persisted, "redis.password", "env-redis-password")
	assertRawValue(t, persisted, "cors.allow_credentials", false)
	if got, _ := lookup(persisted, "cors.allow_origins"); !reflect.DeepEqual(got, []any{"https://env.example", "http://localhost:5173"}) {
		t.Fatalf("persisted cors.allow_origins=%#v", got)
	}
}

func TestLoadMalformedFileReturnsZeroConfigAndDoesNotRewrite(t *testing.T) {
	clearConfigEnv(t)
	malformedPath := filepath.Join(t.TempDir(), "malformed.yaml")
	malformed := []byte("jwt:\n  secret: [unterminated")
	if err := os.WriteFile(malformedPath, malformed, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(malformedPath)
	if err == nil || !reflect.DeepEqual(got, Config{}) {
		t.Fatalf("malformed config result: cfg=%#v err=%v; want zero config plus YAML error", got, err)
	}
	after, readErr := os.ReadFile(malformedPath)
	if readErr != nil || string(after) != string(malformed) {
		t.Fatalf("malformed config should not be rewritten: readErr=%v after=%q", readErr, string(after))
	}
}

func TestLoadRejectsInvalidConfigValuesWithoutRewrite(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	raw := `
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
  expire_days: 7
  access_expire_minutes: 30
keys:
  private_key: "private.pem"
  public_key: "public.pem"
database:
  host: "localhost"
  port: "5432"
  user: "file-user"
  password: "file-password"
  name: "file-db"
  sslmode: "disable"
  max_connections: 0
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
  host: "redis"
  port: "6379"
  password: "file-redis-password"
  db: 0
  key_prefix: "file:"
  public_cache_ttl_seconds: 60
  auth_cache_ttl_seconds: 30
cors:
  allow_origins:
    - "https://file.example"
  allow_credentials: true
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err == nil || err.Error() != "invalid config database.max_connections" {
		t.Fatalf("invalid config error=%v cfg=%#v; want exact database.max_connections error", err, cfg)
	}
	after, readErr := os.ReadFile(path)
	if readErr != nil || string(after) != raw {
		t.Fatalf("invalid config should not be rewritten: readErr=%v after=%q", readErr, string(after))
	}
}

func TestLoadRejectsInvalidEnvironmentOverride(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(minimalConfigYAML()), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_PORT", "not-a-port")

	cfg, err := Load(path)
	if err == nil || err.Error() != "invalid environment variable SERVER_PORT" {
		t.Fatalf("invalid env error=%v cfg=%#v; want exact SERVER_PORT error", err, cfg)
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

func TestPostgresDSNEscapesStructuredFieldsExactly(t *testing.T) {
	got := postgresDSN("db.internal", "5432", "skin user", "p@ss/word", "skin db", "require")
	want := "postgresql://skin%20user:p%40ss%2Fword@db.internal:5432/skin%20db?sslmode=require"
	if got != want {
		t.Fatalf("postgresDSN()=%q; want %q", got, want)
	}
}

func TestLoadWithoutEnvironmentDoesNotRewriteExistingFile(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := minimalConfigYAML()
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

func minimalConfigYAML() string {
	return `
jwt:
  secret: "file-secret-abcdefghijklmnopqrstuvwxyz"
  expire_days: 7
  access_expire_minutes: 30
keys:
  private_key: "private.pem"
  public_key: "public.pem"
database:
  host: "localhost"
  port: "5432"
  user: "file-user"
  password: "file-password"
  name: "file-db"
  sslmode: "disable"
  max_connections: 10
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
  host: "redis"
  port: "6379"
  password: "file-redis-password"
  db: 0
  key_prefix: "file:"
  public_cache_ttl_seconds: 60
  auth_cache_ttl_seconds: 30
cors:
  allow_origins:
    - "https://file.example"
  allow_credentials: true
`
}

func setCompleteConfigEnv(t *testing.T) {
	t.Helper()
	t.Setenv("JWT_SECRET", "env-secret-abcdefghijklmnopqrstuvwxyz")
	t.Setenv("JWT_EXPIRE_DAYS", "8")
	t.Setenv("JWT_ACCESS_EXPIRE_MINUTES", "35")
	t.Setenv("KEYS_PRIVATE_KEY", "env-private.pem")
	t.Setenv("KEYS_PUBLIC_KEY", "/abs/env-public.pem")
	t.Setenv("DATABASE_HOST", "env-db")
	t.Setenv("DATABASE_PORT", "6543")
	t.Setenv("DATABASE_USER", "env-user")
	t.Setenv("DATABASE_PASSWORD", "env-password")
	t.Setenv("DATABASE_NAME", "env-db-name")
	t.Setenv("DATABASE_SSLMODE", "require")
	t.Setenv("DATABASE_MAX_CONNECTIONS", "31")
	t.Setenv("SERVER_SITE_URL", "https://env.example")
	t.Setenv("SERVER_API_URL", "https://env.example/api")
	t.Setenv("SERVER_HOST", "0.0.0.0")
	t.Setenv("SERVER_PORT", "8100")
	t.Setenv("TEXTURES_DIRECTORY", "/env/textures")
	t.Setenv("CAROUSEL_DIRECTORY", "/env/carousel")
	t.Setenv("REDIS_HOST", "127.0.0.1")
	t.Setenv("REDIS_PORT", "6380")
	t.Setenv("REDIS_PASSWORD", "env-redis-password")
	t.Setenv("REDIS_DB", "3")
	t.Setenv("REDIS_KEY_PREFIX", "envprefix:")
	t.Setenv("REDIS_PUBLIC_CACHE_TTL_SECONDS", "220")
	t.Setenv("REDIS_AUTH_CACHE_TTL_SECONDS", "25")
	t.Setenv("CORS_ALLOW_ORIGINS", "https://env.example, http://localhost:5173")
	t.Setenv("CORS_ALLOW_CREDENTIALS", "false")
}

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DATABASE_HOST",
		"DATABASE_PORT",
		"DATABASE_USER",
		"DATABASE_PASSWORD",
		"DATABASE_NAME",
		"DATABASE_SSLMODE",
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
		"REDIS_HOST",
		"REDIS_PORT",
		"REDIS_PASSWORD",
		"REDIS_DB",
		"REDIS_KEY_PREFIX",
		"REDIS_PUBLIC_CACHE_TTL_SECONDS",
		"REDIS_AUTH_CACHE_TTL_SECONDS",
		"CORS_ALLOW_ORIGINS",
		"CORS_ALLOW_CREDENTIALS",
	} {
		t.Setenv(key, "")
	}
}
