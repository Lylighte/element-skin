package redisstore

import (
	"context"
	"encoding/json"
	"time"
)

func (s *MemoryStore) authUserKey(userID string) string {
	return s.key("auth", "user", "v2", userID)
}

func (s *MemoryStore) GetAuthUser(_ context.Context, userID string) (AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.authUserKey(userID))
	if err != nil {
		return AuthUser{}, err
	}
	b, _ := json.Marshal(v)
	var out AuthUser
	_ = json.Unmarshal(b, &out)
	return out, nil
}

func (s *MemoryStore) SetAuthUser(_ context.Context, user AuthUser, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.authUserKey(user.ID), user, ttl)
}

func (s *MemoryStore) InvalidateAuthUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.authUserKey(userID))
	return nil
}
