package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"element-skin/backend/internal/model"

	"github.com/redis/go-redis/v9"
)

func (s *RedisStore) SetYggToken(ctx context.Context, token model.Token, ttl time.Duration) error {
	value := yggTokenFromModel(token)
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.yggTokenKey(token.AccessToken), b, ttl)
	pipe.ZAdd(ctx, s.yggUserTokensKey(token.UserID), redis.Z{Score: float64(token.CreatedAt), Member: token.AccessToken})
	pipe.Expire(ctx, s.yggUserTokensKey(token.UserID), ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetYggToken(ctx context.Context, access string) (model.Token, error) {
	var token yggToken
	if err := s.getJSON(ctx, s.yggTokenKey(access), &token); err != nil {
		return model.Token{}, err
	}
	return token.model(), nil
}

var replaceYggTokenScript = redis.NewScript(`
local old = redis.call("GET", KEYS[1])
if not old then
  return 0
end
local decoded = cjson.decode(old)
redis.call("SET", KEYS[2], ARGV[1], "PX", ARGV[2])
redis.call("DEL", KEYS[1])
redis.call("ZREM", ARGV[3] .. decoded.user_id .. ARGV[4], ARGV[5])
redis.call("ZADD", KEYS[3], ARGV[6], ARGV[7])
redis.call("PEXPIRE", KEYS[3], ARGV[2])
return 1
`)

func (s *RedisStore) ReplaceYggToken(ctx context.Context, oldAccess string, token model.Token, ttl time.Duration) (bool, error) {
	value := yggTokenFromModel(token)
	b, err := json.Marshal(value)
	if err != nil {
		return false, err
	}
	res, err := replaceYggTokenScript.Run(ctx, s.client, []string{
		s.yggTokenKey(oldAccess),
		s.yggTokenKey(token.AccessToken),
		s.yggUserTokensKey(token.UserID),
	}, string(b), ttl.Milliseconds(), s.key("ygg", "user")+":", ":tokens", oldAccess, token.CreatedAt, token.AccessToken).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (s *RedisStore) DeleteYggToken(ctx context.Context, access string) error {
	token, err := s.GetYggToken(ctx, access)
	if errors.Is(err, ErrCacheMiss) {
		return nil
	}
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.yggTokenKey(access))
	pipe.ZRem(ctx, s.yggUserTokensKey(token.UserID), access)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) DeleteYggTokensByUser(ctx context.Context, userID string) error {
	key := s.yggUserTokensKey(userID)
	tokens, err := s.client.ZRange(ctx, key, 0, -1).Result()
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	for _, access := range tokens {
		pipe.Del(ctx, s.yggTokenKey(access))
	}
	pipe.Del(ctx, key)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) TrimYggTokensByUser(ctx context.Context, userID string, keep int) error {
	if keep <= 0 {
		return s.DeleteYggTokensByUser(ctx, userID)
	}
	key := s.yggUserTokensKey(userID)
	count, err := s.client.ZCard(ctx, key).Result()
	if err != nil {
		return err
	}
	excess := count - int64(keep)
	if excess <= 0 {
		return nil
	}
	tokens, err := s.client.ZRange(ctx, key, 0, excess-1).Result()
	if err != nil {
		return err
	}
	pipe := s.client.Pipeline()
	for _, access := range tokens {
		pipe.Del(ctx, s.yggTokenKey(access))
		pipe.ZRem(ctx, key, access)
	}
	_, err = pipe.Exec(ctx)
	return err
}
