package redisstore

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *RedisStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *RedisStore) SetVerificationCode(ctx context.Context, email, typ, code string, ttl time.Duration) error {
	return s.client.Set(ctx, s.verificationKey(email, typ), code, ttl).Err()
}

func (s *RedisStore) SetVerificationCodeIfAbsent(ctx context.Context, email, typ, code string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, s.verificationKey(email, typ), code, ttl).Result()
}

func (s *RedisStore) GetVerificationCode(ctx context.Context, email, typ string) (string, error) {
	code, err := s.client.Get(ctx, s.verificationKey(email, typ)).Result()
	if err == redis.Nil {
		return "", ErrCacheMiss
	}
	return code, err
}

var consumeVerificationCodeScript = redis.NewScript(`
local value = redis.call("GET", KEYS[1])
if not value or string.lower(value) ~= string.lower(ARGV[1]) then
	return 0
end
redis.call("DEL", KEYS[1])
return 1
`)

func (s *RedisStore) ConsumeVerificationCode(ctx context.Context, email, typ, code string) (bool, error) {
	result, err := consumeVerificationCodeScript.Run(
		ctx,
		s.client,
		[]string{s.verificationKey(email, typ)},
		code,
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *RedisStore) DeleteVerificationCode(ctx context.Context, email, typ string) error {
	return s.client.Del(ctx, s.verificationKey(email, typ)).Err()
}
