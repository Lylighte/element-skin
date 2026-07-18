package redisstore

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestRedisStoreRateLimitFallbackGuardAndPrefixDeletion(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()

	if result, err := store.HitRateLimit(ctx, "disabled", 0, time.Minute); err != nil || !result.Allowed || result.Remaining != 0 {
		t.Fatalf("disabled limit result=%#v err=%v", result, err)
	}
	for hit, wantRemaining := range []int{1, 0, 0} {
		result, err := store.HitRateLimit(ctx, "login:ip:203.0.113.8", 2, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		wantAllowed := hit < 2
		if result.Allowed != wantAllowed || result.Remaining != wantRemaining ||
			result.RetryAfter <= 0 || result.RetryAfter > time.Minute {
			t.Fatalf("hit %d result=%#v, want allowed=%v remaining=%d and bounded retry", hit+1, result, wantAllowed, wantRemaining)
		}
	}

	first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute)
	if err != nil || !first {
		t.Fatalf("first fallback mark=%v err=%v, want true nil", first, err)
	}
	duplicate, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute)
	if err != nil || duplicate {
		t.Fatalf("duplicate fallback mark=%v err=%v, want false nil", duplicate, err)
	}
	if err := store.DeleteFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc"); err != nil {
		t.Fatal(err)
	}
	if first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:abc", time.Minute); err != nil || !first {
		t.Fatalf("deleted fallback guard should be reusable: first=%v err=%v", first, err)
	}

	for i := 0; i < 205; i++ {
		key := "bulk_" + strconv.Itoa(i)
		if err := store.SetSetting(ctx, key, "value", time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "keep", Type: "image", StoragePath: "keep.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 205; i++ {
		key := "bulk_" + strconv.Itoa(i)
		if _, err := store.GetSetting(ctx, key); !errors.Is(err, ErrCacheMiss) {
			t.Fatalf("setting %s should be deleted by prefix, got %v", key, err)
		}
	}
	if got, err := store.GetPublicHomepageMedia(ctx); err != nil || len(got) != 1 || got[0].StoragePath != "keep.png" {
		t.Fatalf("prefix deletion must preserve unrelated keys: homepage media=%#v err=%v", got, err)
	}
	if err := store.InvalidatePublicHomepageMedia(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated homepage media error=%v, want ErrCacheMiss", err)
	}
}

func TestRedisStoreConcurrentRateLimitAllowsExactThreshold(t *testing.T) {
	store, _ := newTestRedisStore(t)
	const attempts = 25
	const limit = 7
	type result struct {
		value RateLimitResult
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
			value, err := store.HitRateLimit(context.Background(), "concurrent-redis", limit, time.Minute)
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
			t.Fatalf("concurrent Redis rate limit failed: %v", result.err)
		}
		if result.value.Allowed {
			allowed++
		} else {
			rejected++
		}
		if result.value.Remaining < 0 || result.value.Remaining > limit-1 ||
			result.value.RetryAfter <= 0 || result.value.RetryAfter > time.Minute {
			t.Fatalf("concurrent Redis rate limit returned invalid metadata: %#v", result.value)
		}
	}
	if allowed != limit || rejected != attempts-limit {
		t.Fatalf("concurrent Redis rate limit allowed=%d rejected=%d; want %d and %d", allowed, rejected, limit, attempts-limit)
	}
	final, err := store.HitRateLimit(context.Background(), "concurrent-redis", limit, time.Minute)
	if err != nil || final.Allowed || final.Remaining != 0 {
		t.Fatalf("final Redis rate-limit state=%#v err=%v; want rejected with zero remaining", final, err)
	}
}
