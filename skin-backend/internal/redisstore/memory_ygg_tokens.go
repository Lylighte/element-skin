package redisstore

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"element-skin/backend/internal/model"
)

func (s *MemoryStore) SetYggToken(_ context.Context, token model.Token, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.set(s.yggTokenKey(token.AccessToken), token, ttl); err != nil {
		return err
	}
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	index[token.AccessToken] = token.CreatedAt
	return s.set(s.yggUserTokensKey(token.UserID), index, ttl)
}

func (s *MemoryStore) GetYggToken(_ context.Context, access string) (model.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getYggToken(access)
}

func (s *MemoryStore) getYggToken(access string) (model.Token, error) {
	v, err := s.get(s.yggTokenKey(access))
	if err != nil {
		return model.Token{}, err
	}
	b, _ := json.Marshal(v)
	var token model.Token
	_ = json.Unmarshal(b, &token)
	return token, nil
}

func (s *MemoryStore) ReplaceYggToken(_ context.Context, oldAccess string, token model.Token, ttl time.Duration) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, err := s.getYggToken(oldAccess)
	if err == ErrCacheMiss {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := s.deleteYggToken(old); err != nil {
		return false, err
	}
	if err := s.set(s.yggTokenKey(token.AccessToken), token, ttl); err != nil {
		return false, err
	}
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return false, err
	}
	index[token.AccessToken] = token.CreatedAt
	if err := s.set(s.yggUserTokensKey(token.UserID), index, ttl); err != nil {
		return false, err
	}
	return true, nil
}

func (s *MemoryStore) DeleteYggToken(_ context.Context, access string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.getYggToken(access)
	if err == ErrCacheMiss {
		return nil
	}
	if err != nil {
		return err
	}
	return s.deleteYggToken(token)
}

func (s *MemoryStore) deleteYggToken(token model.Token) error {
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.yggTokenKey(token.AccessToken))
	index, err := s.yggTokenIndex(token.UserID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	delete(index, token.AccessToken)
	if len(index) == 0 {
		delete(s.items, s.yggUserTokensKey(token.UserID))
		return nil
	}
	return s.setPreservingExpiration(s.yggUserTokensKey(token.UserID), index)
}

func (s *MemoryStore) DeleteYggTokensByUser(_ context.Context, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	index, err := s.yggTokenIndex(userID)
	if err != nil && err != ErrCacheMiss {
		return err
	}
	for access := range index {
		delete(s.items, s.yggTokenKey(access))
	}
	delete(s.items, s.yggUserTokensKey(userID))
	return nil
}

func (s *MemoryStore) TrimYggTokensByUser(_ context.Context, userID string, keep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if keep <= 0 {
		if s.Err != nil {
			return s.Err
		}
		index, err := s.yggTokenIndex(userID)
		if err != nil && err != ErrCacheMiss {
			return err
		}
		for access := range index {
			delete(s.items, s.yggTokenKey(access))
		}
		delete(s.items, s.yggUserTokensKey(userID))
		return nil
	}
	index, err := s.yggTokenIndex(userID)
	if err == ErrCacheMiss || len(index) <= keep {
		return nil
	}
	if err != nil {
		return err
	}
	type tokenRef struct {
		access    string
		createdAt int64
	}
	refs := make([]tokenRef, 0, len(index))
	for access, createdAt := range index {
		refs = append(refs, tokenRef{access: access, createdAt: createdAt})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].createdAt < refs[j].createdAt })
	for _, ref := range refs[:len(refs)-keep] {
		delete(s.items, s.yggTokenKey(ref.access))
		delete(index, ref.access)
	}
	return s.setPreservingExpiration(s.yggUserTokensKey(userID), index)
}

func (s *MemoryStore) yggTokenIndex(userID string) (map[string]int64, error) {
	v, err := s.get(s.yggUserTokensKey(userID))
	if err != nil {
		return map[string]int64{}, err
	}
	b, _ := json.Marshal(v)
	var index map[string]int64
	_ = json.Unmarshal(b, &index)
	if index == nil {
		index = map[string]int64{}
	}
	return index, nil
}
