package redisstore

import (
	"context"
	"time"

	"element-skin/backend/internal/model"
)

func (s *RedisStore) SetYggSession(ctx context.Context, session model.Session, ttl time.Duration) error {
	return s.setJSON(ctx, s.yggSessionKey(session.ServerID), yggSessionFromModel(session), ttl)
}

func (s *RedisStore) GetYggSession(ctx context.Context, serverID string) (model.Session, error) {
	var session yggSession
	if err := s.getJSON(ctx, s.yggSessionKey(serverID), &session); err != nil {
		return model.Session{}, err
	}
	return session.model(), nil
}
