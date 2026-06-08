package microsoft

import "element-skin/backend/internal/util"

const (
	stateKindOAuth   = "oauth_state"
	stateKindProfile = "profile"
	stateKindImport  = "import"
)

func (h Handler) popState(token, kind, invalidDetail string) (map[string]any, error) {
	session, ok := h.states.Pop(token).(map[string]any)
	if !ok || session["kind"] != kind {
		return nil, util.HTTPError{Status: 400, Detail: invalidDetail}
	}
	return session, nil
}

func requireStateOwner(session map[string]any, userID, detail string) error {
	if session["user_id"] != userID {
		return util.HTTPError{Status: 403, Detail: detail}
	}
	return nil
}

func randomToken(length int) (string, error) {
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
