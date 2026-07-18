package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"element-skin/backend/internal/model"
)

func (s *MemoryStore) fallbackPublicKeysKey(sourceID string) string {
	return s.key("fallback", "publickeys", sourceID)
}

func (s *MemoryStore) SetFallbackPublicKeys(_ context.Context, sourceID string, keys model.YggdrasilPublicKeys, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.fallbackPublicKeysKey(sourceID), keys, ttl)
}

func (s *MemoryStore) GetFallbackPublicKeys(_ context.Context, sourceIDs []string) (map[string]model.YggdrasilPublicKeys, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]model.YggdrasilPublicKeys, len(sourceIDs))
	seen := make(map[string]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		if _, exists := seen[sourceID]; exists {
			continue
		}
		seen[sourceID] = struct{}{}
		value, err := s.get(s.fallbackPublicKeysKey(sourceID))
		if err != nil {
			if errors.Is(err, ErrCacheMiss) {
				continue
			}
			return nil, err
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		var keys model.YggdrasilPublicKeys
		if err := json.Unmarshal(encoded, &keys); err != nil {
			return nil, err
		}
		out[sourceID] = keys
	}
	return out, nil
}

func (s *MemoryStore) InvalidateFallbackPublicKeys(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "fallback:publickeys:")
}
