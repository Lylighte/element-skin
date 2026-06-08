package microsoft

import (
	"context"
	"net/url"
	"strings"
)

func (c MicrosoftHTTPClient) ExchangeCodeForToken(ctx context.Context, code string) (map[string]any, error) {
	form := url.Values{}
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", c.RedirectURI)
	form.Set("grant_type", "authorization_code")
	var out map[string]any
	err := c.do(ctx, "POST", "https://login.microsoftonline.com/consumers/oauth2/v2.0/token", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", "", &out)
	return out, err
}
