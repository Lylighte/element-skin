package microsoft

import (
	"context"
	"fmt"
)

func (c MicrosoftHTTPClient) AuthenticateMinecraft(ctx context.Context, userHash, xstsToken string) (string, error) {
	var out map[string]any
	if err := c.postJSON(ctx, "https://api.minecraftservices.com/authentication/login_with_xbox", map[string]any{
		"identityToken": "XBL3.0 x=" + userHash + ";" + xstsToken,
	}, "", &out); err != nil {
		return "", err
	}
	token, _ := out["access_token"].(string)
	if token == "" {
		return "", fmt.Errorf("minecraft login response missing access_token")
	}
	return token, nil
}

func (c MicrosoftHTTPClient) CheckGameOwnership(ctx context.Context, mcAccessToken string) (bool, error) {
	var out map[string]any
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/entitlements/mcstore", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return false, err
	}
	items, _ := out["items"].([]any)
	return len(items) > 0, nil
}

func (c MicrosoftHTTPClient) GetMinecraftProfile(ctx context.Context, mcAccessToken string) (map[string]any, error) {
	var out map[string]any
	if err := c.do(ctx, "GET", "https://api.minecraftservices.com/minecraft/profile", nil, "", "Bearer "+mcAccessToken, &out); err != nil {
		return nil, err
	}
	return out, nil
}
