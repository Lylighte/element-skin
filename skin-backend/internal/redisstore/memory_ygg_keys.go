package redisstore

func (s *MemoryStore) yggTokenKey(access string) string {
	return s.key("ygg", "token", access)
}

func (s *MemoryStore) yggUserTokensKey(userID string) string {
	return s.key("ygg", "user", userID, "tokens")
}

func (s *MemoryStore) yggSessionKey(serverID string) string {
	return s.key("ygg", "session", serverID)
}
