package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DatabaseDSN     string
	MaxConnections  int32
	JWTSecret       string
	JWTExpireDays   int
	AccessMinutes   int
	SiteURL         string
	APIURL          string
	ServerHost      string
	ServerPort      string
	TexturesDir     string
	CarouselDir     string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RedisKeyPrefix  string
	PublicCacheTTL  int
	AuthCacheTTL    int
	PrivateKeyPath  string
	PublicKeyPath   string
	CORSOrigins     []string
	CORSCredentials bool
}

type rawConfig = map[string]any

func Load(path string) (Config, error) {
	cfg := Config{}
	raw := rawConfig{}
	writeConfig := false
	if b, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return cfg, err
		}
		cfg.apply(raw)
	} else if errors.Is(err, os.ErrNotExist) {
		writeConfig = true
	} else {
		return cfg, err
	}
	envChanged, err := applyEnvOverrides(&cfg, raw)
	if err != nil {
		return Config{}, err
	}
	if envChanged {
		writeConfig = true
	}
	if err := validateRequiredConfig(cfg, raw); err != nil {
		return Config{}, err
	}
	if writeConfig {
		if err := writeConfigFile(path, raw); err != nil {
			return cfg, err
		}
	}
	cfg.resolveKeyPaths(filepath.Dir(path))
	return cfg, nil
}

func Defaults() Config {
	return Config{
		DatabaseDSN:     "postgresql://elementskin:password@localhost:5432/elementskin",
		MaxConnections:  10,
		JWTSecret:       "dev-secret-please-change-to-a-very-long-string-in-production",
		JWTExpireDays:   7,
		AccessMinutes:   30,
		SiteURL:         "http://localhost",
		APIURL:          "",
		ServerHost:      "0.0.0.0",
		ServerPort:      "8000",
		TexturesDir:     "textures",
		CarouselDir:     "carousel",
		RedisAddr:       "127.0.0.1:6379",
		RedisPassword:   "",
		RedisDB:         0,
		RedisKeyPrefix:  "elementskin:",
		PublicCacheTTL:  60,
		AuthCacheTTL:    30,
		PrivateKeyPath:  "private.pem",
		PublicKeyPath:   "public.pem",
		CORSOrigins:     []string{"*"},
		CORSCredentials: true,
	}
}

func (c *Config) apply(raw rawConfig) {
	c.DatabaseDSN = getString(raw, "database.dsn", c.DatabaseDSN)
	if n := getInt(raw, "database.max_connections", int(c.MaxConnections)); n > 0 {
		c.MaxConnections = int32(n)
	}
	c.JWTSecret = getString(raw, "jwt.secret", c.JWTSecret)
	c.JWTExpireDays = getInt(raw, "jwt.expire_days", c.JWTExpireDays)
	c.AccessMinutes = getInt(raw, "jwt.access_expire_minutes", c.AccessMinutes)
	c.SiteURL = getString(raw, "server.site_url", c.SiteURL)
	c.APIURL = getString(raw, "server.api_url", c.APIURL)
	c.ServerHost = getString(raw, "server.host", c.ServerHost)
	if n := getInt(raw, "server.port", 0); n > 0 {
		c.ServerPort = strconv.Itoa(n)
	}
	c.TexturesDir = getString(raw, "textures.directory", c.TexturesDir)
	c.CarouselDir = getString(raw, "carousel.directory", c.CarouselDir)
	c.RedisAddr = getString(raw, "redis.addr", c.RedisAddr)
	c.RedisPassword = getString(raw, "redis.password", c.RedisPassword)
	if n := getInt(raw, "redis.db", c.RedisDB); n >= 0 {
		c.RedisDB = n
	}
	c.RedisKeyPrefix = getString(raw, "redis.key_prefix", c.RedisKeyPrefix)
	if n := getInt(raw, "redis.public_cache_ttl_seconds", c.PublicCacheTTL); n > 0 {
		c.PublicCacheTTL = n
	}
	if n := getInt(raw, "redis.auth_cache_ttl_seconds", c.AuthCacheTTL); n > 0 {
		c.AuthCacheTTL = n
	}
	c.PrivateKeyPath = getString(raw, "keys.private_key", c.PrivateKeyPath)
	c.PublicKeyPath = getString(raw, "keys.public_key", c.PublicKeyPath)
	c.CORSOrigins = getStringSlice(raw, "cors.allow_origins", c.CORSOrigins)
	c.CORSCredentials = getBool(raw, "cors.allow_credentials", c.CORSCredentials)
}

func configRaw(cfg Config) rawConfig {
	return rawConfig{
		"jwt": map[string]any{
			"secret":                cfg.JWTSecret,
			"expire_days":           cfg.JWTExpireDays,
			"access_expire_minutes": cfg.AccessMinutes,
		},
		"keys": map[string]any{
			"private_key": cfg.PrivateKeyPath,
			"public_key":  cfg.PublicKeyPath,
		},
		"database": map[string]any{
			"dsn":             cfg.DatabaseDSN,
			"max_connections": int(cfg.MaxConnections),
		},
		"redis": map[string]any{
			"addr":                     cfg.RedisAddr,
			"password":                 cfg.RedisPassword,
			"db":                       cfg.RedisDB,
			"key_prefix":               cfg.RedisKeyPrefix,
			"public_cache_ttl_seconds": cfg.PublicCacheTTL,
			"auth_cache_ttl_seconds":   cfg.AuthCacheTTL,
		},
		"textures": map[string]any{
			"directory": cfg.TexturesDir,
		},
		"carousel": map[string]any{
			"directory": cfg.CarouselDir,
		},
		"server": map[string]any{
			"host":     cfg.ServerHost,
			"port":     atoiDefault(cfg.ServerPort, 8000),
			"site_url": cfg.SiteURL,
			"api_url":  cfg.APIURL,
		},
		"cors": map[string]any{
			"allow_origins":     cfg.CORSOrigins,
			"allow_credentials": cfg.CORSCredentials,
		},
	}
}

func applyEnvOverrides(cfg *Config, raw rawConfig) (bool, error) {
	changed := false
	var applied bool
	var err error
	for _, apply := range []func() (bool, error){
		func() (bool, error) { return applyStringEnv(raw, "JWT_SECRET", "jwt.secret", &cfg.JWTSecret) },
		func() (bool, error) {
			return applyIntEnv(raw, "JWT_EXPIRE_DAYS", "jwt.expire_days", &cfg.JWTExpireDays, positiveInt)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "JWT_ACCESS_EXPIRE_MINUTES", "jwt.access_expire_minutes", &cfg.AccessMinutes, positiveInt)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "KEYS_PRIVATE_KEY", "keys.private_key", &cfg.PrivateKeyPath)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "KEYS_PUBLIC_KEY", "keys.public_key", &cfg.PublicKeyPath)
		},
		func() (bool, error) { return applyStringEnv(raw, "DATABASE_DSN", "database.dsn", &cfg.DatabaseDSN) },
		func() (bool, error) {
			return applyInt32Env(raw, "DATABASE_MAX_CONNECTIONS", "database.max_connections", &cfg.MaxConnections, positiveInt)
		},
		func() (bool, error) { return applyStringEnv(raw, "SERVER_SITE_URL", "server.site_url", &cfg.SiteURL) },
		func() (bool, error) { return applyStringEnv(raw, "SERVER_API_URL", "server.api_url", &cfg.APIURL) },
		func() (bool, error) { return applyStringEnv(raw, "SERVER_HOST", "server.host", &cfg.ServerHost) },
		func() (bool, error) { return applyServerPortEnv(raw, cfg) },
		func() (bool, error) {
			return applyStringEnv(raw, "TEXTURES_DIRECTORY", "textures.directory", &cfg.TexturesDir)
		},
		func() (bool, error) {
			return applyStringEnv(raw, "CAROUSEL_DIRECTORY", "carousel.directory", &cfg.CarouselDir)
		},
		func() (bool, error) { return applyStringEnv(raw, "REDIS_ADDR", "redis.addr", &cfg.RedisAddr) },
		func() (bool, error) {
			return applyStringEnv(raw, "REDIS_PASSWORD", "redis.password", &cfg.RedisPassword)
		},
		func() (bool, error) { return applyIntEnv(raw, "REDIS_DB", "redis.db", &cfg.RedisDB, nonNegativeInt) },
		func() (bool, error) {
			return applyStringEnv(raw, "REDIS_KEY_PREFIX", "redis.key_prefix", &cfg.RedisKeyPrefix)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "REDIS_PUBLIC_CACHE_TTL_SECONDS", "redis.public_cache_ttl_seconds", &cfg.PublicCacheTTL, positiveInt)
		},
		func() (bool, error) {
			return applyIntEnv(raw, "REDIS_AUTH_CACHE_TTL_SECONDS", "redis.auth_cache_ttl_seconds", &cfg.AuthCacheTTL, positiveInt)
		},
		func() (bool, error) {
			return applyStringSliceEnv(raw, "CORS_ALLOW_ORIGINS", "cors.allow_origins", &cfg.CORSOrigins)
		},
		func() (bool, error) {
			return applyBoolEnv(raw, "CORS_ALLOW_CREDENTIALS", "cors.allow_credentials", &cfg.CORSCredentials)
		},
	} {
		applied, err = apply()
		if err != nil {
			return false, err
		}
		changed = changed || applied
	}
	return changed, nil
}

func validateRequiredConfig(cfg Config, raw rawConfig) error {
	required := []string{
		"database.dsn",
		"database.max_connections",
		"jwt.secret",
		"jwt.expire_days",
		"jwt.access_expire_minutes",
		"server.site_url",
		"server.api_url",
		"server.host",
		"server.port",
		"textures.directory",
		"carousel.directory",
		"redis.addr",
		"redis.db",
		"redis.key_prefix",
		"redis.public_cache_ttl_seconds",
		"redis.auth_cache_ttl_seconds",
		"keys.private_key",
		"keys.public_key",
		"cors.allow_origins",
		"cors.allow_credentials",
	}
	var missing []string
	for _, field := range required {
		if _, ok := lookup(raw, field); !ok {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required config fields: %s", strings.Join(missing, ", "))
	}
	for _, check := range []struct {
		field string
		ok    bool
	}{
		{field: "database.dsn", ok: cfg.DatabaseDSN != ""},
		{field: "database.max_connections", ok: cfg.MaxConnections > 0},
		{field: "jwt.secret", ok: cfg.JWTSecret != ""},
		{field: "jwt.expire_days", ok: cfg.JWTExpireDays > 0},
		{field: "jwt.access_expire_minutes", ok: cfg.AccessMinutes > 0},
		{field: "server.site_url", ok: cfg.SiteURL != ""},
		{field: "server.api_url", ok: cfg.APIURL != ""},
		{field: "server.host", ok: cfg.ServerHost != ""},
		{field: "server.port", ok: atoiDefault(cfg.ServerPort, 0) > 0},
		{field: "textures.directory", ok: cfg.TexturesDir != ""},
		{field: "carousel.directory", ok: cfg.CarouselDir != ""},
		{field: "redis.addr", ok: cfg.RedisAddr != ""},
		{field: "redis.db", ok: cfg.RedisDB >= 0},
		{field: "redis.key_prefix", ok: cfg.RedisKeyPrefix != ""},
		{field: "redis.public_cache_ttl_seconds", ok: cfg.PublicCacheTTL > 0},
		{field: "redis.auth_cache_ttl_seconds", ok: cfg.AuthCacheTTL > 0},
		{field: "keys.private_key", ok: cfg.PrivateKeyPath != ""},
		{field: "keys.public_key", ok: cfg.PublicKeyPath != ""},
		{field: "cors.allow_origins", ok: len(cfg.CORSOrigins) > 0},
	} {
		if !check.ok {
			return fmt.Errorf("invalid config %s", check.field)
		}
	}
	return nil
}

func applyStringEnv(raw rawConfig, envName, dotted string, target *string) (bool, error) {
	value := os.Getenv(envName)
	if value == "" {
		return false, nil
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func applyIntEnv(raw rawConfig, envName, dotted string, target *int, valid func(int) bool) (bool, error) {
	value, ok, err := parseIntEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if valid != nil && !valid(value) {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func applyInt32Env(raw rawConfig, envName, dotted string, target *int32, valid func(int) bool) (bool, error) {
	value, ok, err := parseIntEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if valid != nil && !valid(value) {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = int32(value)
	setRaw(raw, dotted, value)
	return true, nil
}

func applyServerPortEnv(raw rawConfig, cfg *Config) (bool, error) {
	value, ok, err := parseIntEnv("SERVER_PORT")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	if !positiveInt(value) {
		return false, fmt.Errorf("invalid environment variable SERVER_PORT")
	}
	cfg.ServerPort = strconv.Itoa(value)
	setRaw(raw, "server.port", value)
	return true, nil
}

func applyStringSliceEnv(raw rawConfig, envName, dotted string, target *[]string) (bool, error) {
	value := os.Getenv(envName)
	if value == "" {
		return false, nil
	}
	items := splitEnvList(value)
	if len(items) == 0 {
		return false, fmt.Errorf("invalid environment variable %s", envName)
	}
	*target = items
	setRaw(raw, dotted, items)
	return true, nil
}

func applyBoolEnv(raw rawConfig, envName, dotted string, target *bool) (bool, error) {
	value, ok, err := parseBoolEnv(envName)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	*target = value
	setRaw(raw, dotted, value)
	return true, nil
}

func parseIntEnv(envName string) (int, bool, error) {
	raw := os.Getenv(envName)
	if raw == "" {
		return 0, false, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false, fmt.Errorf("invalid environment variable %s", envName)
	}
	return value, true, nil
}

func parseBoolEnv(envName string) (bool, bool, error) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return false, false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false, fmt.Errorf("invalid environment variable %s", envName)
	}
	return value, true, nil
}

func splitEnvList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func positiveInt(value int) bool {
	return value > 0
}

func nonNegativeInt(value int) bool {
	return value >= 0
}

func setRaw(raw rawConfig, dotted string, value any) {
	cur := raw
	start := 0
	for i := 0; i <= len(dotted); i++ {
		if i != len(dotted) && dotted[i] != '.' {
			continue
		}
		key := dotted[start:i]
		if i == len(dotted) {
			cur[key] = value
			return
		}
		next, ok := cur[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[key] = next
		}
		cur = next
		start = i + 1
	}
}

func writeConfigFile(path string, raw rawConfig) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	out, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o600)
}

func (c *Config) resolveKeyPaths(baseDir string) {
	c.PrivateKeyPath = resolveRelativePath(baseDir, c.PrivateKeyPath)
	c.PublicKeyPath = resolveRelativePath(baseDir, c.PublicKeyPath)
}

func resolveRelativePath(baseDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func getString(raw rawConfig, dotted string, fallback string) string {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fallback
}

func getInt(raw rawConfig, dotted string, fallback int) int {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		return atoiDefault(n, fallback)
	default:
		return fallback
	}
}

func getStringSlice(raw rawConfig, dotted string, fallback []string) []string {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch items := v.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		out := make([]string, 0, len(items))
		for _, item := range items {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		if len(out) == len(items) {
			return out
		}
		return fallback
	default:
		return fallback
	}
}

func getBool(raw rawConfig, dotted string, fallback bool) bool {
	v, ok := lookup(raw, dotted)
	if !ok || v == nil {
		return fallback
	}
	switch b := v.(type) {
	case bool:
		return b
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(b))
		if err == nil {
			return parsed
		}
		return fallback
	default:
		return fallback
	}
}

func lookup(raw rawConfig, dotted string) (any, bool) {
	cur := any(raw)
	start := 0
	for i := 0; i <= len(dotted); i++ {
		if i != len(dotted) && dotted[i] != '.' {
			continue
		}
		key := dotted[start:i]
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[key]
		if !ok {
			return nil, false
		}
		start = i + 1
	}
	return cur, true
}

func atoiDefault(s string, fallback int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fallback
	}
	return n
}
