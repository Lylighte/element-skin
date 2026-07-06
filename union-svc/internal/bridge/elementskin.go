package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CreatedProfile mirrors the Element-Skin POST /v1/profiles success body.
type CreatedProfile struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Model string `json:"model,omitempty"`
}

// APIError describes a non-success HTTP response from the Element-Skin API.
type APIError struct {
	Status int
	Detail string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("element-skin API returned HTTP %d: %s", e.Status, e.Detail)
}

// ElementSkinClient calls the Element-Skin public API using a Bearer token.
type ElementSkinClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewElementSkinClient creates a client for the upstream Element-Skin API.
func NewElementSkinClient(baseURL string, httpClient *http.Client) *ElementSkinClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &ElementSkinClient{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: httpClient,
	}
}

// CreateProfile creates a profile on Element-Skin for the authenticated user.
func (c *ElementSkinClient) CreateProfile(ctx context.Context, token, name, model string) (*CreatedProfile, error) {
	body, err := json.Marshal(map[string]string{
		"name":  name,
		"model": model,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal create profile body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/profiles", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build create profile request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute create profile request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read create profile response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		detail := extractDetail(respBody)
		if detail == "" {
			detail = string(respBody)
		}
		return nil, &APIError{Status: resp.StatusCode, Detail: detail}
	}

	var cp CreatedProfile
	if err := json.Unmarshal(respBody, &cp); err != nil {
		return nil, fmt.Errorf("decode create profile response: %w", err)
	}
	return &cp, nil
}

func extractDetail(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return string(body)
	}
	if d, ok := m["detail"].(string); ok {
		return d
	}
	return string(body)
}
