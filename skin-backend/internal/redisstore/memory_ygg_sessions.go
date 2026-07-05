package redisstore

import (
	"context"
	"encoding/json"
	"time"

	"element-skin/backend/internal/model"
)

func (s *MemoryStore) SetYggSession(_ context.Context, session model.Session, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.yggSessionKey(session.ServerID), session, ttl)
}

func (s *MemoryStore) GetYggSession(_ context.Context, serverID string) (model.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.yggSessionKey(serverID))
	if err != nil {
		return model.Session{}, err
	}
	b, _ := json.Marshal(v)
	var session model.Session
	_ = json.Unmarshal(b, &session)
	return session, nil
}
