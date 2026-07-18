package oauth

import (
	"context"
	"errors"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (s Service) RevokeToken(ctx context.Context, clientID, clientSecret, token string) error {
	client, err := s.authenticateClient(ctx, clientID, clientSecret)
	if err != nil {
		return err
	}
	tokenHash := util.HashRefreshToken(token)
	if access, err := s.Redis.GetOAuthAccessToken(ctx, tokenHash); err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		return err
	} else if err == nil {
		if access.ClientID != client.ID {
			return forbidden()
		}
		return s.Redis.DeleteOAuthAccessToken(ctx, tokenHash)
	}
	if refresh, err := s.DB.OAuth.GetRefreshToken(ctx, tokenHash); err != nil {
		return err
	} else if refresh != nil {
		if refresh.ClientID != client.ID {
			return forbidden()
		}
		_, err = s.DB.OAuth.RevokeRefreshToken(ctx, tokenHash, database.NowMS())
		return err
	}
	return nil
}
