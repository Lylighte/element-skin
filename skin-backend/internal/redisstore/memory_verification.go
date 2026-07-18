package redisstore

import (
	"context"
	"strings"
	"time"
)

func (s *MemoryStore) verificationKey(email, typ string) string {
	return s.key("verification", strings.ToLower(strings.TrimSpace(typ)), strings.ToLower(strings.TrimSpace(email)))
}

func (s *MemoryStore) SetVerificationCode(_ context.Context, email, typ, code string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.set(s.verificationKey(email, typ), code, ttl)
}

func (s *MemoryStore) SetVerificationCodeIfAbsent(_ context.Context, email, typ, code string, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.verificationKey(email, typ)
	if _, err := s.get(key); err == nil {
		return false, nil
	} else if err != ErrCacheMiss {
		return false, err
	}
	return true, s.set(key, code, ttl)
}

func (s *MemoryStore) GetVerificationCode(_ context.Context, email, typ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.get(s.verificationKey(email, typ))
	if err != nil {
		return "", err
	}
	code, _ := v.(string)
	return code, nil
}

func (s *MemoryStore) ConsumeVerificationCode(_ context.Context, email, typ, code string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.verificationKey(email, typ)
	v, err := s.get(key)
	if err == ErrCacheMiss {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	stored, _ := v.(string)
	if !strings.EqualFold(stored, code) {
		return false, nil
	}
	delete(s.items, key)
	return true, nil
}

func (s *MemoryStore) DeleteVerificationCode(_ context.Context, email, typ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.verificationKey(email, typ))
	return nil
}
