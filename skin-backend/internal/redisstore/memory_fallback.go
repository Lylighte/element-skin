package redisstore

import (
	"context"
	"time"
)

func (s *MemoryStore) MarkFallbackRequest(_ context.Context, endpoint, request string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key("fallback", "request", endpoint, request)
	if _, err := s.get(key); err == nil {
		return false, nil
	} else if err != ErrCacheMiss {
		return false, err
	}
	return true, s.set(key, true, ttl)
}

func (s *MemoryStore) DeleteFallbackRequest(_ context.Context, endpoint, request string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.key("fallback", "request", endpoint, request))
	return nil
}
