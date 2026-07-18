package redisstore

import (
	"context"
	"encoding/json"
	"time"
)

func (s *MemoryStore) stateKey(token string) string {
	return s.key("state", token)
}

func (s *MemoryStore) SetState(_ context.Context, token string, value map[string]any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.stateKey(token), value, ttl)
}

func (s *MemoryStore) PopState(_ context.Context, token string) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.stateKey(token)
	value, err := s.get(key)
	if err != nil {
		return nil, err
	}
	delete(s.items, key)
	out, ok := value.(map[string]any)
	if ok {
		return out, nil
	}
	b, _ := json.Marshal(value)
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}
