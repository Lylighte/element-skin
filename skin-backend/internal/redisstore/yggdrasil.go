package redisstore

import (
	"element-skin/backend/internal/model"
)

type yggToken struct {
	AccessToken string  `json:"access_token"`
	ClientToken string  `json:"client_token"`
	UserID      string  `json:"user_id"`
	ProfileID   *string `json:"profile_id,omitempty"`
	CreatedAt   int64   `json:"created_at"`
}

type yggSession struct {
	ServerID    string  `json:"server_id"`
	AccessToken string  `json:"access_token"`
	IP          *string `json:"ip,omitempty"`
	CreatedAt   int64   `json:"created_at"`
}

func yggTokenFromModel(t model.Token) yggToken {
	return yggToken{AccessToken: t.AccessToken, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: t.ProfileID, CreatedAt: t.CreatedAt}
}

func (t yggToken) model() model.Token {
	return model.Token{AccessToken: t.AccessToken, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: t.ProfileID, CreatedAt: t.CreatedAt}
}

func yggSessionFromModel(s model.Session) yggSession {
	return yggSession{ServerID: s.ServerID, AccessToken: s.AccessToken, IP: s.IP, CreatedAt: s.CreatedAt}
}

func (s yggSession) model() model.Session {
	return model.Session{ServerID: s.ServerID, AccessToken: s.AccessToken, IP: s.IP, CreatedAt: s.CreatedAt}
}

func (s *RedisStore) yggTokenKey(access string) string {
	return s.key("ygg", "token", access)
}

func (s *RedisStore) yggUserTokensKey(userID string) string {
	return s.key("ygg", "user", userID, "tokens")
}

func (s *RedisStore) yggSessionKey(serverID string) string {
	return s.key("ygg", "session", serverID)
}
