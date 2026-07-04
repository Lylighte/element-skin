package config

import (
	"errors"
	"log"
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
	cfg := Defaults()
	raw := configRaw(cfg)
	writeConfig := false
	if b, err := os.ReadFile(path); err == nil {
		raw = rawConfig{}
		if err := yaml.Unmarshal(b, &raw); err != nil {
			return cfg, err
		}
		cfg.apply(raw)
	} else if errors.Is(err, os.ErrNotExist) {
		log.Printf("警告：配置文件 %s 未找到，使用默认配置（JWT secret 为占位值，启动将失败）", path)
		writeConfig = true
	} else {
		return cfg, err
	}
	if applyEnvOverrides(&cfg, raw) {
		writeConfig = true
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
	c.ServerPort = strconv.Itoa(getInt(raw, "server.port", atoiDefault(c.ServerPort, 8000)))
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

func applyEnvOverrides(cfg *Config, raw rawConfig) bool {
	changed := false
	changed = applyStringEnv(raw, "JWT_SECRET", "jwt.secret", &cfg.JWTSecret) || changed
	changed = applyIntEnv(raw, "JWT_EXPIRE_DAYS", "jwt.expire_days", &cfg.JWTExpireDays, nil) || changed
	changed = applyIntEnv(raw, "JWT_ACCESS_EXPIRE_MINUTES", "jwt.access_expire_minutes", &cfg.AccessMinutes, nil) || changed
	changed = applyStringEnv(raw, "KEYS_PRIVATE_KEY", "keys.private_key", &cfg.PrivateKeyPath) || changed
	changed = applyStringEnv(raw, "KEYS_PUBLIC_KEY", "keys.public_key", &cfg.PublicKeyPath) || changed
	changed = applyStringEnv(raw, "DATABASE_DSN", "database.dsn", &cfg.DatabaseDSN) || changed
	changed = applyInt32Env(raw, "DATABASE_MAX_CONNECTIONS", "database.max_connections", &cfg.MaxConnections, positiveInt) || changed
	changed = applyStringEnv(raw, "SERVER_SITE_URL", "server.site_url", &cfg.SiteURL) || changed
	changed = applyStringEnv(raw, "SERVER_API_URL", "server.api_url", &cfg.APIURL) || changed
	changed = applyStringEnv(raw, "SERVER_HOST", "server.host", &cfg.ServerHost) || changed
	changed = applyServerPortEnv(raw, cfg) || changed
	changed = applyStringEnv(raw, "TEXTURES_DIRECTORY", "textures.directory", &cfg.TexturesDir) || changed
	changed = applyStringEnv(raw, "CAROUSEL_DIRECTORY", "carousel.directory", &cfg.CarouselDir) || changed
	changed = applyStringEnv(raw, "REDIS_ADDR", "redis.addr", &cfg.RedisAddr) || changed
	changed = applyStringEnv(raw, "REDIS_PASSWORD", "redis.password", &cfg.RedisPassword) || changed
	changed = applyIntEnv(raw, "REDIS_DB", "redis.db", &cfg.RedisDB, nonNegativeInt) || changed
	changed = applyStringEnv(raw, "REDIS_KEY_PREFIX", "redis.key_prefix", &cfg.RedisKeyPrefix) || changed
	changed = applyIntEnv(raw, "REDIS_PUBLIC_CACHE_TTL_SECONDS", "redis.public_cache_ttl_seconds", &cfg.PublicCacheTTL, positiveInt) || changed
	changed = applyIntEnv(raw, "REDIS_AUTH_CACHE_TTL_SECONDS", "redis.auth_cache_ttl_seconds", &cfg.AuthCacheTTL, positiveInt) || changed
	changed = applyStringSliceEnv(raw, "CORS_ALLOW_ORIGINS", "cors.allow_origins", &cfg.CORSOrigins) || changed
	changed = applyBoolEnv(raw, "CORS_ALLOW_CREDENTIALS", "cors.allow_credentials", &cfg.CORSCredentials) || changed
	return changed
}

func applyStringEnv(raw rawConfig, envName, dotted string, target *string) bool {
	value := os.Getenv(envName)
	if value == "" {
		return false
	}
	*target = value
	setRaw(raw, dotted, value)
	return true
}

func applyIntEnv(raw rawConfig, envName, dotted string, target *int, valid func(int) bool) bool {
	value, ok := parseIntEnv(envName)
	if !ok || valid != nil && !valid(value) {
		return false
	}
	*target = value
	setRaw(raw, dotted, value)
	return true
}

func applyInt32Env(raw rawConfig, envName, dotted string, target *int32, valid func(int) bool) bool {
	value, ok := parseIntEnv(envName)
	if !ok || valid != nil && !valid(value) {
		return false
	}
	*target = int32(value)
	setRaw(raw, dotted, value)
	return true
}

func applyServerPortEnv(raw rawConfig, cfg *Config) bool {
	value, ok := parseIntEnv("SERVER_PORT")
	if !ok || !positiveInt(value) {
		return false
	}
	cfg.ServerPort = strconv.Itoa(value)
	setRaw(raw, "server.port", value)
	return true
}

func applyStringSliceEnv(raw rawConfig, envName, dotted string, target *[]string) bool {
	value := os.Getenv(envName)
	if value == "" {
		return false
	}
	items := splitEnvList(value)
	*target = items
	setRaw(raw, dotted, items)
	return true
}

func applyBoolEnv(raw rawConfig, envName, dotted string, target *bool) bool {
	value, ok := parseBoolEnv(envName)
	if !ok {
		return false
	}
	*target = value
	setRaw(raw, dotted, value)
	return true
}

func parseIntEnv(envName string) (int, bool) {
	raw := os.Getenv(envName)
	if raw == "" {
		return 0, false
	}
	value, err := strconv.Atoi(raw)
	return value, err == nil
}

func parseBoolEnv(envName string) (bool, bool) {
	raw := strings.TrimSpace(os.Getenv(envName))
	if raw == "" {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	return value, err == nil
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
