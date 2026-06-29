package redisstore

import (
	"context"
	"time"
)

func (s *RedisStore) fallbackRequestKey(endpoint, request string) string {
	return s.key("fallback", "request", endpoint, request)
}

func (s *RedisStore) MarkFallbackRequest(ctx context.Context, endpoint, request string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.fallbackRequestKey(endpoint, request), "1", ttl).Result()
}

func (s *RedisStore) DeleteFallbackRequest(ctx context.Context, endpoint, request string) error {
	return s.client.Del(ctx, s.fallbackRequestKey(endpoint, request)).Err()
}
