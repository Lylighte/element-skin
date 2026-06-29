package redisstore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"
)

type memoryItem struct {
	value     any
	expiresAt time.Time
}

type MemoryStore struct {
	mu     sync.Mutex
	now    func() time.Time
	items  map[string]memoryItem
	closed bool
	Err    error
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{now: time.Now, items: map[string]memoryItem{}}
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

func (s *MemoryStore) key(parts ...string) string {
	return strings.Join(parts, ":")
}

func (s *MemoryStore) get(key string) (any, error) {
	if s.Err != nil {
		return nil, s.Err
	}
	item, ok := s.items[key]
	if !ok {
		return nil, ErrCacheMiss
	}
	if !item.expiresAt.IsZero() && !item.expiresAt.After(s.now()) {
		delete(s.items, key)
		return nil, ErrCacheMiss
	}
	return cloneValue(item.value), nil
}

func (s *MemoryStore) set(key string, value any, ttl time.Duration) error {
	if s.Err != nil {
		return s.Err
	}
	var expiresAt time.Time
	if ttl > 0 {
		expiresAt = s.now().Add(ttl)
	}
	s.items[key] = memoryItem{value: cloneValue(value), expiresAt: expiresAt}
	return nil
}

func (s *MemoryStore) setPreservingExpiration(key string, value any) error {
	if s.Err != nil {
		return s.Err
	}
	item, ok := s.items[key]
	if !ok {
		return s.set(key, value, 0)
	}
	item.value = cloneValue(value)
	s.items[key] = item
	return nil
}

func cloneValue(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}
func (s *MemoryStore) DeleteByPrefix(_ context.Context, prefix string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	for key := range s.items {
		if strings.HasPrefix(key, prefix) {
			delete(s.items, key)
		}
	}
	return nil
}
