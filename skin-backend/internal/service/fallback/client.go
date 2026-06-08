package fallback

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
)

func (f Fallback) get(ctx context.Context, rawURL string) (*FallbackResponse, error) {
	return f.do(ctx, http.MethodGet, rawURL, nil)
}

func (f Fallback) postJSON(ctx context.Context, rawURL string, body any) (*FallbackResponse, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(body); err != nil {
		return nil, err
	}
	return f.do(ctx, http.MethodPost, rawURL, &b)
}

func (f Fallback) do(ctx context.Context, method, rawURL string, reqBody io.Reader) (*FallbackResponse, error) {
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reqBody)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &FallbackResponse{Status: resp.StatusCode, Body: respBody}, nil
}
