package redisstore

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *RedisStore) SetOAuthAccessToken(ctx context.Context, token OAuthAccessToken, ttl time.Duration) error {
	raw, err := json.Marshal(token)
	if err != nil {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, s.oauthAccessKey(token.TokenHash), raw, ttl)
	for _, indexKey := range s.oauthAccessIndexKeys(token) {
		pipe.SAdd(ctx, indexKey, token.TokenHash)
		if ttl > 0 {
			pipe.Expire(ctx, indexKey, ttl)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) GetOAuthAccessToken(ctx context.Context, tokenHash string) (OAuthAccessToken, error) {
	var token OAuthAccessToken
	if err := s.getJSON(ctx, s.oauthAccessKey(tokenHash), &token); err != nil {
		return OAuthAccessToken{}, err
	}
	return token, nil
}

func (s *RedisStore) DeleteOAuthAccessToken(ctx context.Context, tokenHash string) error {
	token, err := s.GetOAuthAccessToken(ctx, tokenHash)
	if err != nil && !errors.Is(err, ErrCacheMiss) {
		return err
	}
	pipe := s.client.TxPipeline()
	pipe.Del(ctx, s.oauthAccessKey(tokenHash))
	if err == nil {
		for _, indexKey := range s.oauthAccessIndexKeys(token) {
			pipe.SRem(ctx, indexKey, tokenHash)
		}
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) DeleteOAuthAccessTokensByClient(ctx context.Context, clientID string) error {
	return s.deleteOAuthAccessTokensByIndex(ctx, s.oauthAccessClientIndexKey(clientID))
}

func (s *RedisStore) DeleteOAuthAccessTokensByGrant(ctx context.Context, grantID string) error {
	return s.deleteOAuthAccessTokensByIndex(ctx, s.oauthAccessGrantIndexKey(grantID))
}

func (s *RedisStore) DeleteOAuthAccessTokensByUser(ctx context.Context, userID string) error {
	return s.deleteOAuthAccessTokensByIndex(ctx, s.oauthAccessUserIndexKey(userID))
}

func (s *RedisStore) deleteOAuthAccessTokensByIndex(ctx context.Context, indexKey string) error {
	hashes, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := s.client.TxPipeline()
	for _, hash := range hashes {
		pipe.Del(ctx, s.oauthAccessKey(hash))
	}
	pipe.Del(ctx, indexKey)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisStore) oauthAccessIndexKeys(token OAuthAccessToken) []string {
	keys := []string{s.oauthAccessClientIndexKey(token.ClientID)}
	if token.GrantID != "" {
		keys = append(keys, s.oauthAccessGrantIndexKey(token.GrantID))
	}
	if token.UserID != "" {
		keys = append(keys, s.oauthAccessUserIndexKey(token.UserID))
	}
	return keys
}

func (s *RedisStore) oauthAccessKey(tokenHash string) string {
	return s.key("oauth", "access", tokenHash)
}

func (s *RedisStore) oauthAccessClientIndexKey(clientID string) string {
	return s.key("oauth", "access-index", "client", clientID)
}

func (s *RedisStore) oauthAccessGrantIndexKey(grantID string) string {
	return s.key("oauth", "access-index", "grant", grantID)
}

func (s *RedisStore) oauthAccessUserIndexKey(userID string) string {
	return s.key("oauth", "access-index", "user", userID)
}
