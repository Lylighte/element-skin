package union

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"element-skin/union-svc/internal/config"
)

// Client communicates with the Union Hub using the member_key protocol.
type Client struct {
	hubURL    string
	memberKey string
	timeout   time.Duration
	http      *http.Client
	nonces    *NonceStore
	settings  *SettingsStore

	mu       sync.RWMutex
	pubKey   string
	pubKeyAt time.Time
}

// NewClient constructs a Union Hub client from configuration, opening the
// nonce store and settings store at the configured storage path.
func NewClient(cfg config.Config, httpClient *http.Client) (*Client, error) {
	nonces, err := OpenNonceStore(cfg.Storage.Path)
	if err != nil {
		return nil, fmt.Errorf("open nonce store: %w", err)
	}

	settings, err := OpenSettingsStore(cfg.Storage.Path)
	if err != nil {
		_ = nonces.Close()
		return nil, fmt.Errorf("open settings store: %w", err)
	}

	// Seed the runtime member_key from configuration when the settings store
	// does not already have one. SQLite is the runtime authority after startup.
	if cfg.Union.MemberKey != "" {
		ctx := context.Background()
		existing, err := settings.Get(ctx, "member_key")
		if err != nil {
			_ = settings.Close()
			_ = nonces.Close()
			return nil, fmt.Errorf("read member_key setting: %w", err)
		}
		if existing == "" {
			if err := settings.Set(ctx, "member_key", cfg.Union.MemberKey); err != nil {
				_ = settings.Close()
				_ = nonces.Close()
				return nil, fmt.Errorf("seed member_key setting: %w", err)
			}
		}
	}

	return newClient(cfg.Union.HubURL, cfg.Union.MemberKey, cfg.Union.TimeoutSeconds, httpClient, nonces, settings), nil
}

// NewClientWithDeps constructs a client with explicit dependencies. It is
// intended for tests.
func NewClientWithDeps(hubURL, memberKey string, timeoutSeconds int, httpClient *http.Client, nonces *NonceStore, settings *SettingsStore) *Client {
	return newClient(hubURL, memberKey, timeoutSeconds, httpClient, nonces, settings)
}

func newClient(hubURL, memberKey string, timeoutSeconds int, httpClient *http.Client, nonces *NonceStore, settings *SettingsStore) *Client {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	c := &Client{
		hubURL:    strings.TrimRight(hubURL, "/"),
		memberKey: memberKey,
		timeout:   timeout,
		nonces:    nonces,
		settings:  settings,
	}

	if httpClient != nil {
		c.http = httpClient
	}

	return c
}

// Close closes the nonce store and settings store.
func (c *Client) Close() error {
	var firstErr error
	if c.nonces != nil {
		if err := c.nonces.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.settings != nil {
		if err := c.settings.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// SettingsStore returns the client's runtime settings store. Handlers use it
// to read and write Union configuration such as member_key and list versions.
func (c *Client) SettingsStore() *SettingsStore {
	return c.settings
}

func (c *Client) configured() error {
	if c.hubURL == "" {
		return ErrUnionNotConfigured
	}
	if c.memberKey == "" && (c.settings == nil || c.memberKeyFromSettings(context.Background()) == "") {
		return ErrUnionNotConfigured
	}
	return nil
}

// memberKeyFromSettings returns the current member_key from the settings store,
// or an empty string if the store is nil or the key is missing.
func (c *Client) memberKeyFromSettings(ctx context.Context) string {
	if c.settings == nil {
		return ""
	}
	key, err := c.settings.Get(ctx, "member_key")
	if err != nil {
		return ""
	}
	return key
}

// currentMemberKey returns the runtime member_key: the value from settings if
// present, otherwise the configured fallback.
func (c *Client) currentMemberKey(ctx context.Context) string {
	if key := c.memberKeyFromSettings(ctx); key != "" {
		return key
	}
	return c.memberKey
}

// FetchServerList pulls the current Union server list from the Hub.
// It returns the raw server list JSON and the version reported by the Hub.
func (c *Client) FetchServerList(ctx context.Context) (json.RawMessage, int, error) {
	body, err := c.request(ctx, http.MethodGet, "/serverlist", nil, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch server list: %w", err)
	}

	var resp struct {
		Servers json.RawMessage `json:"servers"`
		Version int             `json:"version"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("decode server list response: %w", err)
	}

	return resp.Servers, resp.Version, nil
}

// FetchPrivateKey pulls the current private key PEM and version from the Hub.
// The PEM itself is not persisted; only the version is stored by callers.
func (c *Client) FetchPrivateKey(ctx context.Context) (string, int, error) {
	body, err := c.request(ctx, http.MethodGet, "/privatekey", nil, nil)
	if err != nil {
		return "", 0, fmt.Errorf("fetch private key: %w", err)
	}

	var resp struct {
		PrivateKey        string `json:"privateKey"`
		PrivateKeyVersion int    `json:"privateKeyVersion"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", 0, fmt.Errorf("decode private key response: %w", err)
	}

	return resp.PrivateKey, resp.PrivateKeyVersion, nil
}

// SyncProfiles reports the local profile name-to-UUID mapping to the Union Hub.
func (c *Client) SyncProfiles(ctx context.Context, profileList map[string]string) error {
	_, err := c.request(ctx, http.MethodPost, "/sync", map[string]any{
		"profileList": profileList,
	}, nil)
	if err != nil {
		return fmt.Errorf("sync profiles: %w", err)
	}
	return nil
}
