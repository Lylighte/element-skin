package redisstore

import (
	"context"
	"encoding/json"
	"time"

	"element-skin/backend/internal/model"
)

func (s *MemoryStore) GetSetting(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("settings", key))
	if err != nil {
		return "", err
	}
	out, _ := v.(string)
	return out, nil
}

func (s *MemoryStore) SetSetting(_ context.Context, key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("settings", key), value, ttl)
}

func (s *MemoryStore) InvalidateSettings(ctx context.Context) error {
	return s.DeleteByPrefix(ctx, "settings:")
}

func (s *MemoryStore) GetPublicSettings(context.Context) (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "settings"))
	if err != nil {
		return nil, err
	}
	if out, ok := v.(map[string]any); ok {
		return out, nil
	}
	return map[string]any{}, nil
}

func (s *MemoryStore) SetPublicSettings(_ context.Context, value map[string]any, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "settings"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicSettings(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "settings"))
	return nil
}

func (s *MemoryStore) GetPublicHomepageMedia(context.Context) ([]model.HomepageMedia, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.key("public", "homepage-media"))
	if err != nil {
		return nil, err
	}
	b, _ := json.Marshal(v)
	var out []model.HomepageMedia
	_ = json.Unmarshal(b, &out)
	if out == nil {
		out = []model.HomepageMedia{}
	}
	return out, nil
}

func (s *MemoryStore) SetPublicHomepageMedia(_ context.Context, value []model.HomepageMedia, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.key("public", "homepage-media"), value, ttl)
}

func (s *MemoryStore) InvalidatePublicHomepageMedia(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("public", "homepage-media"))
	return nil
}
