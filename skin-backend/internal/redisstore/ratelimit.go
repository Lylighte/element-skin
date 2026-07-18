package redisstore

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

var rateLimitScript = redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`)

func (s *RedisStore) HitRateLimit(ctx context.Context, key string, limit int, window time.Duration) (RateLimitResult, error) {
	if limit <= 0 {
		return RateLimitResult{Allowed: true}, nil
	}
	values, err := rateLimitScript.Run(ctx, s.client, []string{s.key("ratelimit", key)}, window.Milliseconds()).Slice()
	if err != nil {
		return RateLimitResult{}, err
	}
	count, _ := values[0].(int64)
	ttlMS, _ := values[1].(int64)
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}
	return RateLimitResult{
		Allowed:    int(count) <= limit,
		Remaining:  remaining,
		RetryAfter: time.Duration(ttlMS) * time.Millisecond,
	}, nil
}
