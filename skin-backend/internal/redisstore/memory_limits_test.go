package redisstore_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreRateLimitRejectsAfterExactThreshold(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	for i := 0; i < 2; i++ {
		res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
		if err != nil || !res.Allowed {
			t.Fatalf("hit %d should be allowed: %#v err=%v", i+1, res, err)
		}
	}
	res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
	if err != nil || res.Allowed || res.Remaining != 0 {
		t.Fatalf("third hit should be rejected: %#v err=%v", res, err)
	}
}

func TestMemoryStoreConcurrentRateLimitAllowsExactThreshold(t *testing.T) {
	store := redisstore.NewMemoryStore()
	const attempts = 25
	const limit = 7
	type result struct {
		value redisstore.RateLimitResult
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := store.HitRateLimit(context.Background(), "concurrent-memory", limit, time.Minute)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	rejected := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent memory rate limit failed: %v", result.err)
		}
		if result.value.Allowed {
			allowed++
		} else {
			rejected++
		}
		if result.value.Remaining < 0 || result.value.Remaining > limit-1 || result.value.RetryAfter <= 0 {
			t.Fatalf("concurrent memory rate limit returned invalid metadata: %#v", result.value)
		}
	}
	if allowed != limit || rejected != attempts-limit {
		t.Fatalf("concurrent memory rate limit allowed=%d rejected=%d; want %d and %d", allowed, rejected, limit, attempts-limit)
	}
	final, err := store.HitRateLimit(context.Background(), "concurrent-memory", limit, time.Minute)
	if err != nil || final.Allowed || final.Remaining != 0 {
		t.Fatalf("final memory rate-limit state=%#v err=%v; want rejected with zero remaining", final, err)
	}
}
