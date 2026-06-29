package redisstore

import (
	"context"
	"time"
)

func (s *MemoryStore) HitRateLimit(_ context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return RateLimitResult{}, s.Err
	}
	if limit <= 0 {
		return RateLimitResult{Allowed: true}, nil
	}
	k := s.key("ratelimit", key)
	now := s.now()
	item, ok := s.items[k]
	count := 0
	expiresAt := now.Add(window)
	if ok && (item.expiresAt.IsZero() || item.expiresAt.After(now)) {
		if n, ok := item.value.(int); ok {
			count = n
		}
		expiresAt = item.expiresAt
	}
	count++
	s.items[k] = memoryItem{value: count, expiresAt: expiresAt}
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{Allowed: count <= limit, Remaining: remaining, RetryAfter: expiresAt.Sub(now)}, nil
}
