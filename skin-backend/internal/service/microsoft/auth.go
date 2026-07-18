package microsoft

import (
	"net/url"
)

func MicrosoftAuthorizationURL(clientID, redirectURI, state string) string {
	q := url.Values{}
	q.Set("client_id", clientID)
	q.Set("response_type", "code")
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "XboxLive.signin offline_access")
	q.Set("state", state)
	return "https://login.live.com/oauth20_authorize.srf?" + q.Encode()
}
