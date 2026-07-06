package union

import (
	"errors"
	"fmt"
)

// HubError describes a non-success HTTP response from the Union Hub.
type HubError struct {
	Status int
	Detail string
}

func (e *HubError) Error() string {
	return fmt.Sprintf("union hub returned HTTP %d: %s", e.Status, e.Detail)
}

// Sentinel errors for the Union client.
var (
	ErrUnionNotConfigured       = errors.New("union hub is not configured")
	ErrMissingSignatureHeaders  = errors.New("missing Union signature headers")
	ErrInvalidSignature         = errors.New("invalid Union signature")
	ErrInvalidPublicKey         = errors.New("invalid Union public key")
	ErrReplay                   = errors.New("nonce already used (replay detected)")
	ErrTimestampOutOfWindow     = errors.New("timestamp outside acceptable window")
	ErrBlacklistEntryNotFound   = errors.New("blacklist entry not found")
	ErrSecurityLevelCodeMissing = errors.New("union hub returned empty security level code")
)
