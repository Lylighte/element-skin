package redisstore_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreFallbackRequestLoopGuardLifecycle(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !first {
		t.Fatalf("first fallback request should be marked as new: first=%v err=%v", first, err)
	}
	duplicate, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || duplicate {
		t.Fatalf("duplicate fallback request should be rejected until cleared: duplicate=%v err=%v", duplicate, err)
	}
	otherEndpoint, err := store.MarkFallbackRequest(ctx, "https://other.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !otherEndpoint {
		t.Fatalf("same request on different endpoint should have independent guard: first=%v err=%v", otherEndpoint, err)
	}
	if err := store.DeleteFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server"); err != nil {
		t.Fatal(err)
	}
	afterDelete, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !afterDelete {
		t.Fatalf("deleted fallback guard should allow request again: first=%v err=%v", afterDelete, err)
	}

	expiringStore := redisstore.NewMemoryStore()
	expiringFirst, err := expiringStore.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:uuid", time.Nanosecond)
	if err != nil || !expiringFirst {
		t.Fatalf("first expiring fallback mark mismatch: first=%v err=%v", expiringFirst, err)
	}
	time.Sleep(time.Millisecond)
	afterTTL, err := expiringStore.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:uuid", time.Minute)
	if err != nil || !afterTTL {
		t.Fatalf("expired fallback guard should allow request again: first=%v err=%v", afterTTL, err)
	}
}
