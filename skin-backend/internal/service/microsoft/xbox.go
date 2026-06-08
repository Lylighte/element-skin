package microsoft

import (
	"context"
	"fmt"
)

func (c MicrosoftHTTPClient) AuthenticateXBL(ctx context.Context, msAccessToken string) (string, string, error) {
	var out map[string]any
	err := c.postJSON(ctx, "https://user.auth.xboxlive.com/user/authenticate", map[string]any{
		"Properties":   map[string]any{"AuthMethod": "RPS", "SiteName": "user.auth.xboxlive.com", "RpsTicket": "d=" + msAccessToken},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}, "", &out)
	if err != nil {
		return "", "", err
	}
	return tokenAndUHS(out)
}

func (c MicrosoftHTTPClient) AuthenticateXSTS(ctx context.Context, xblToken string) (string, string, error) {
	var out map[string]any
	err := c.postJSON(ctx, "https://xsts.auth.xboxlive.com/xsts/authorize", map[string]any{
		"Properties":   map[string]any{"SandboxId": "RETAIL", "UserTokens": []string{xblToken}},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}, "", &out)
	if err != nil {
		return "", "", err
	}
	return tokenAndUHS(out)
}

func tokenAndUHS(data map[string]any) (string, string, error) {
	token, _ := data["Token"].(string)
	claims, _ := data["DisplayClaims"].(map[string]any)
	xui, _ := claims["xui"].([]any)
	if token == "" || len(xui) == 0 {
		return "", "", fmt.Errorf("xbox response missing token or user hash")
	}
	first, _ := xui[0].(map[string]any)
	uhs, _ := first["uhs"].(string)
	if uhs == "" {
		return "", "", fmt.Errorf("xbox response missing user hash")
	}
	return token, uhs, nil
}
