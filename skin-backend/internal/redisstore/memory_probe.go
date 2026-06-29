package redisstore

import (
	"context"
	"encoding/json"
	"sort"
	"time"
)

func (s *MemoryStore) probeHistoryKey() string {
	return s.key("probe", "history")
}

func (s *MemoryStore) AppendProbeSamples(_ context.Context, samples []ProbeSample, retention time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	if len(samples) == 0 {
		return nil
	}
	key := s.probeHistoryKey()
	current := s.probeHistory(key)
	current = append(current, samples...)
	cutoff := s.now().Add(-retention).UnixMilli()
	if retention > 0 {
		filtered := current[:0]
		for _, sample := range current {
			if sample.CheckedAt >= cutoff {
				filtered = append(filtered, sample)
			}
		}
		current = filtered
	}
	sort.Slice(current, func(i, j int) bool { return current[i].CheckedAt < current[j].CheckedAt })
	return s.set(key, current, 0)
}

func (s *MemoryStore) GetProbeHistory(_ context.Context, since time.Time) ([]ProbeSample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return nil, s.Err
	}
	all := s.probeHistory(s.probeHistoryKey())
	cutoff := since.UnixMilli()
	out := make([]ProbeSample, 0, len(all))
	for _, sample := range all {
		if sample.CheckedAt >= cutoff {
			out = append(out, sample)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CheckedAt < out[j].CheckedAt })
	return out, nil
}

func (s *MemoryStore) InvalidateProbeHistory(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Err != nil {
		return s.Err
	}
	delete(s.items, s.probeHistoryKey())
	return nil
}

func (s *MemoryStore) probeHistory(key string) []ProbeSample {
	v, err := s.get(key)
	if err != nil {
		return nil
	}
	b, _ := json.Marshal(v)
	var out []ProbeSample
	_ = json.Unmarshal(b, &out)
	return out
}
