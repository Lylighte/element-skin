package oauth

import "context"

func (s Service) revokeClientAuthorizations(ctx context.Context, clientID string, revokedAt int64) error {
	if _, err := s.DB.OAuth.RevokeGrantsByClient(ctx, clientID, revokedAt); err != nil {
		return err
	}
	return s.invalidateClientCredentials(ctx, clientID, revokedAt)
}

func (s Service) invalidateClientCredentials(ctx context.Context, clientID string, revokedAt int64) error {
	if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, clientID); err != nil {
		return err
	}
	if _, err := s.DB.OAuth.RevokeRefreshTokensByClient(ctx, clientID, revokedAt); err != nil {
		return err
	}
	if _, err := s.DB.OAuth.DeleteAuthorizationCodesByClient(ctx, clientID); err != nil {
		return err
	}
	return nil
}

func (s Service) invalidateGrantCredentials(ctx context.Context, grantID string, revokedAt int64) error {
	if err := s.Redis.DeleteOAuthAccessTokensByGrant(ctx, grantID); err != nil {
		return err
	}
	if _, err := s.DB.OAuth.RevokeRefreshTokensByGrant(ctx, grantID, revokedAt); err != nil {
		return err
	}
	if _, err := s.DB.OAuth.DeleteAuthorizationCodesByGrant(ctx, grantID); err != nil {
		return err
	}
	return nil
}
