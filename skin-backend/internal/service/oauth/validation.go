package oauth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/url"
)

func validPKCE(verifier, challenge string) bool {
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func validHTTPURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != ""
}

func validClientStatus(status string) bool {
	switch status {
	case StatusPending, StatusActive, StatusRejected, StatusDisabled:
		return true
	default:
		return false
	}
}
