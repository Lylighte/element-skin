package microsoft

import (
	"context"

	"element-skin/backend/internal/util"
)

type MicrosoftAuthClient interface {
	ExchangeCodeForToken(ctx context.Context, code string) (map[string]any, error)
	AuthenticateXBL(ctx context.Context, msAccessToken string) (token string, userHash string, err error)
	AuthenticateXSTS(ctx context.Context, xblToken string) (token string, userHash string, err error)
	AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error)
	CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error)
	GetMinecraftProfile(ctx context.Context, mcAccessToken string) (map[string]any, error)
}

type MicrosoftAuthFlow struct {
	Client MicrosoftAuthClient
}

func (f MicrosoftAuthFlow) Complete(ctx context.Context, code string) (map[string]any, error) {
	tokenData, err := f.Client.ExchangeCodeForToken(ctx, code)
	if err != nil {
		return nil, err
	}
	msAccess, _ := tokenData["access_token"].(string)
	if msAccess == "" {
		return nil, util.HTTPError{Status: 400, Detail: "Microsoft token response missing access_token"}
	}
	xblToken, _, err := f.Client.AuthenticateXBL(ctx, msAccess)
	if err != nil {
		return nil, err
	}
	xstsToken, userHash, err := f.Client.AuthenticateXSTS(ctx, xblToken)
	if err != nil {
		return nil, err
	}
	mcAccess, err := f.Client.AuthenticateMinecraft(ctx, userHash, xstsToken)
	if err != nil {
		return nil, err
	}
	hasGame, err := f.Client.CheckGameOwnership(ctx, mcAccess)
	if err != nil {
		return nil, err
	}
	profile, err := f.Client.GetMinecraftProfile(ctx, mcAccess)
	if err != nil {
		return nil, err
	}
	return map[string]any{"mc_access_token": mcAccess, "has_game": hasGame, "profile": profile}, nil
}
