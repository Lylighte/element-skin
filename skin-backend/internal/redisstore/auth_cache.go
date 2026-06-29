package redisstore

import (
	"context"
	"time"
)

func (s *RedisStore) authUserKey(userID string) string {
	return s.key("auth", "user", "v2", userID)
}

func (s *RedisStore) GetAuthUser(ctx context.Context, userID string) (AuthUser, error) {
	var out AuthUser
	if err := s.getJSON(ctx, s.authUserKey(userID), &out); err != nil {
		return AuthUser{}, err
	}
	return out, nil
}

func (s *RedisStore) SetAuthUser(ctx context.Context, user AuthUser, ttl time.Duration) error {
	return s.setJSON(ctx, s.authUserKey(user.ID), user, ttl)
}

func (s *RedisStore) InvalidateAuthUser(ctx context.Context, userID string) error {
	return s.client.Del(ctx, s.authUserKey(userID)).Err()
}
