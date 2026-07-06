package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds the union-svc runtime configuration.
type Config struct {
	Server struct {
		Addr string `yaml:"addr" env:"SERVER_ADDR"`
		Port int    `yaml:"port" env:"SERVER_PORT"`
	} `yaml:"server"`
	Elementskin struct {
		BaseURL string `yaml:"base_url" env:"ELEMENTSKIN_BASE_URL"`
		OAuth   struct {
			ClientID     string `yaml:"client_id" env:"ELEMENTSKIN_OAUTH_CLIENT_ID"`
			ClientSecret string `yaml:"client_secret" env:"ELEMENTSKIN_OAUTH_CLIENT_SECRET"`
			RedirectURI  string `yaml:"redirect_uri" env:"ELEMENTSKIN_OAUTH_REDIRECT_URI"`
		} `yaml:"oauth"`
	} `yaml:"elementskin"`
	Storage struct {
		Path string `yaml:"path" env:"STORAGE_PATH"`
	} `yaml:"storage"`
	Union struct {
		HubURL         string `yaml:"hub_url" env:"HUB_URL"`
		MemberKey      string `yaml:"member_key" env:"MEMBER_KEY"`
		TimeoutSeconds int    `yaml:"timeout_seconds" env:"TIMEOUT_SECONDS"`
	} `yaml:"union" env:"UNION"`
	Log struct {
		Level string `yaml:"level" env:"LOG_LEVEL"`
	} `yaml:"log"`
}

// defaults returns a Config populated with default values.
func defaults() Config {
	var cfg Config
	cfg.Server.Addr = ""
	cfg.Server.Port = 8001
	cfg.Elementskin.BaseURL = "http://127.0.0.1:8000"
	cfg.Elementskin.OAuth.RedirectURI = "http://127.0.0.1:8001/oauth/callback"
	cfg.Storage.Path = "./union-svc.db"
	cfg.Union.TimeoutSeconds = 30
	cfg.Log.Level = "info"
	return cfg
}

// Load reads the YAML file at path, applies default values, and then overrides
// with environment variables that carry the UNION_ prefix.
func Load(path string) (Config, error) {
	cfg := defaults()

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return cfg, fmt.Errorf("read config file: %w", err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return cfg, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	if err := applyEnv("UNION", reflect.ValueOf(&cfg).Elem()); err != nil {
		return cfg, fmt.Errorf("apply env vars: %w", err)
	}

	return cfg, nil
}

// applyEnv walks v recursively and, for each struct field that carries an
// `env` tag, checks for a matching environment variable under prefix. Nested
// structs are walked with a combined prefix.
func applyEnv(prefix string, v reflect.Value) error {
	t := v.Type()
	for i := range t.NumField() {
		field := t.Field(i)
		fv := v.Field(i)

		if fv.Kind() == reflect.Struct {
			nextPrefix := prefix
			if tag := field.Tag.Get("env"); tag != "" {
				nextPrefix = joinEnv(nextPrefix, tag)
			}
			if err := applyEnv(nextPrefix, fv); err != nil {
				return err
			}
			continue
		}

		tag := field.Tag.Get("env")
		if tag == "" {
			continue
		}
		key := joinEnv(prefix, tag)
		value, ok := os.LookupEnv(key)
		if !ok {
			continue
		}
		if err := setField(fv, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	return nil
}

func joinEnv(prefix, suffix string) string {
	return prefix + "_" + suffix
}

func setField(v reflect.Value, value string) error {
	switch v.Kind() {
	case reflect.String:
		v.SetString(value)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return err
		}
		v.SetUint(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return err
		}
		v.SetBool(b)
	default:
		return fmt.Errorf("unsupported kind %s", v.Kind())
	}
	return nil
}

// ListenAddr returns the host:port string derived from Server config.
func (c Config) ListenAddr() string {
	if c.Server.Addr == "" {
		return fmt.Sprintf("127.0.0.1:%d", c.Server.Port)
	}
	return fmt.Sprintf("%s:%d", c.Server.Addr, c.Server.Port)
}
