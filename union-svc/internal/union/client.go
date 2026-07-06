package union

import (
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

	mu       sync.RWMutex
	pubKey   string
	pubKeyAt time.Time
}

// NewClient constructs a Union Hub client from configuration, opening the
// nonce store at the configured storage path.
func NewClient(cfg config.Config, httpClient *http.Client) (*Client, error) {
	nonces, err := OpenNonceStore(cfg.Storage.Path)
	if err != nil {
		return nil, fmt.Errorf("open nonce store: %w", err)
	}

	return newClient(cfg.Union.HubURL, cfg.Union.MemberKey, cfg.Union.TimeoutSeconds, httpClient, nonces), nil
}

// NewClientWithDeps constructs a client with explicit dependencies. It is
// intended for tests.
func NewClientWithDeps(hubURL, memberKey string, timeoutSeconds int, httpClient *http.Client, nonces *NonceStore) *Client {
	return newClient(hubURL, memberKey, timeoutSeconds, httpClient, nonces)
}

func newClient(hubURL, memberKey string, timeoutSeconds int, httpClient *http.Client, nonces *NonceStore) *Client {
	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	c := &Client{
		hubURL:    strings.TrimRight(hubURL, "/"),
		memberKey: memberKey,
		timeout:   timeout,
		nonces:    nonces,
	}

	if httpClient != nil {
		c.http = httpClient
	}

	return c
}

// Close closes the nonce store.
func (c *Client) Close() error {
	if c.nonces == nil {
		return nil
	}
	return c.nonces.Close()
}

func (c *Client) configured() error {
	if c.hubURL == "" || c.memberKey == "" {
		return ErrUnionNotConfigured
	}
	return nil
}
