package redisstore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *RedisStore) permCacheKey(subjectID string) string {
	return s.prefix + "perm:eff:" + subjectID
}

func (s *RedisStore) GetPermissionCache(ctx context.Context, subjectID string) (string, bool, error) {
	raw, err := s.client.Get(ctx, s.permCacheKey(subjectID)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return raw, true, nil
}

func (s *RedisStore) SetPermissionCache(ctx context.Context, subjectID, encoded string, ttl time.Duration) error {
	return s.client.Set(ctx, s.permCacheKey(subjectID), encoded, ttl).Err()
}

func (s *RedisStore) DeletePermissionCache(ctx context.Context, subjectID string) error {
	return s.client.Del(ctx, s.permCacheKey(subjectID)).Err()
}
