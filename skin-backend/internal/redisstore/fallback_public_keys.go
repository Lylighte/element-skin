package redisstore

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/model"
)

const FallbackPublicKeysTTL = 48 * time.Hour

func (s *RedisStore) fallbackPublicKeysKey(sourceID string) string {
	return s.key("fallback", "publickeys", sourceID)
}

func (s *RedisStore) SetFallbackPublicKeys(ctx context.Context, sourceID string, keys model.YggdrasilPublicKeys, ttl time.Duration) error {
	return s.setJSON(ctx, s.fallbackPublicKeysKey(sourceID), keys, ttl)
}

func (s *RedisStore) GetFallbackPublicKeys(ctx context.Context, sourceIDs []string) (map[string]model.YggdrasilPublicKeys, error) {
	out := make(map[string]model.YggdrasilPublicKeys, len(sourceIDs))
	seen := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if _, exists := seen[sourceID]; exists {
			continue
		}
		seen[sourceID] = struct{}{}
		var keys model.YggdrasilPublicKeys
		if err := s.getJSON(ctx, s.fallbackPublicKeysKey(sourceID), &keys); err != nil {
			if errors.Is(err, ErrCacheMiss) {
				continue
			}
			return nil, err
		}
		out[sourceID] = keys
	}
	return out, nil
}

func (s *RedisStore) InvalidateFallbackPublicKeys(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "fallback:publickeys:")
}
