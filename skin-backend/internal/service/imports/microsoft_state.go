package imports

import (
	"context"
	"errors"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

const (
	microsoftStateKindOAuth   = "oauth_state"
	microsoftStateKindProfile = "profile"
	microsoftStateKindImport  = "import"
)

func (s MicrosoftImportWorkflow) popState(ctx context.Context, token, kind, invalidDetail string) (map[string]any, error) {
	session, err := s.States.PopState(ctx, token)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, util.HTTPError{Status: 400, Detail: invalidDetail}
	}
	if err != nil {
		return nil, err
	}
	if session["kind"] != kind {
		return nil, util.HTTPError{Status: 400, Detail: invalidDetail}
	}
	return session, nil
}

func requireMicrosoftStateOwner(session map[string]any, userID, detail string) error {
	if session["user_id"] != userID {
		return util.HTTPError{Status: 403, Detail: detail}
	}
	return nil
}

func randomMicrosoftToken(length int) (string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return "", err
	}
	token := id
	for len(token) < length {
		next, err := util.GenerateUUIDNoDash()
		if err != nil {
			return "", err
		}
		token += next
	}
	return token[:length], nil
}
