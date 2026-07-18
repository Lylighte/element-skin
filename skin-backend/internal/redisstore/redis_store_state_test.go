package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedisStoreStateIsAtomicSingleUseAndExpiresExactly(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()

	if _, err := store.PopState(ctx, "missing-state"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing state error=%v, want ErrCacheMiss", err)
	}

	state := map[string]any{
		"kind":    "oauth_state",
		"user_id": "user-1",
		"profile": map[string]any{
			"id":   "profile-1",
			"name": "PlayerOne",
		},
	}
	if err := store.SetState(ctx, "oauth-token", state, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.PopState(ctx, "oauth-token")
	if err != nil {
		t.Fatal(err)
	}
	if got["kind"] != "oauth_state" || got["user_id"] != "user-1" {
		t.Fatalf("state scalar fields mismatch: %#v", got)
	}
	profile, ok := got["profile"].(map[string]any)
	if !ok || profile["id"] != "profile-1" || profile["name"] != "PlayerOne" {
		t.Fatalf("state nested profile mismatch: %#v", got)
	}
	if _, err := store.PopState(ctx, "oauth-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should be consumed atomically, got %v", err)
	}

	if err := store.SetState(ctx, "expires-token", map[string]any{"kind": "profile"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	if _, err := store.PopState(ctx, "expires-token"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("state token should expire at TTL boundary, got %v", err)
	}
}
