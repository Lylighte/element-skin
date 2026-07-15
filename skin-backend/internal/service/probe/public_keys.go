package probe

import (
	"context"
	"io"
	"net/http"
	"sync"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	fallbacksvc "element-skin/backend/internal/service/fallback"
)

const maxPublicKeysResponseBytes = 1 << 20

type publicKeyFetchResult struct {
	keys model.YggdrasilPublicKeys
	ok   bool
}

func (s *Service) refreshFallbackPublicKeys(ctx context.Context, endpoints []map[string]any) error {
	sources := fallbacksvc.PublicKeySources(endpoints)
	results := make([]publicKeyFetchResult, len(sources))
	var wg sync.WaitGroup
	for index, source := range sources {
		wg.Add(1)
		go func(index int, source fallbacksvc.PublicKeySource) {
			defer wg.Done()
			results[index] = s.fetchFallbackPublicKeys(ctx, source)
		}(index, source)
	}
	wg.Wait()
	for index, result := range results {
		if !result.ok {
			continue
		}
		if err := s.Redis.SetFallbackPublicKeys(ctx, sources[index].ID, result.keys, redisstore.FallbackPublicKeysTTL); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) fetchFallbackPublicKeys(ctx context.Context, source fallbacksvc.PublicKeySource) publicKeyFetchResult {
	if source.DiscoveryURL != "" {
		if body, ok := s.getPublicKeysResponse(ctx, source.DiscoveryURL); ok {
			if keys, err := fallbacksvc.ParseDiscoveryPublicKeys(body); err == nil {
				return publicKeyFetchResult{keys: keys, ok: true}
			}
		}
	}
	if body, ok := s.getPublicKeysResponse(ctx, source.ServicesPublicURL); ok {
		if keys, err := fallbacksvc.ParseServicesPublicKeys(body); err == nil {
			return publicKeyFetchResult{keys: keys, ok: true}
		}
	}
	return publicKeyFetchResult{}
}

func (s *Service) getPublicKeysResponse(ctx context.Context, rawURL string) ([]byte, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: httpTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPublicKeysResponseBytes+1))
	if err != nil || len(body) > maxPublicKeysResponseBytes {
		return nil, false
	}
	return body, true
}
