package redisstore

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestRedisStoreOAuthAccessAndPermissionCacheLifecycle(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()
	token := OAuthAccessToken{
		TokenHash:     "redis-access-hash",
		ClientID:      "client-1",
		UserID:        "user-1",
		GrantID:       "grant-1",
		PermissionIDs: []int64{11, 22, 33},
		ExpiresAt:     5000,
		CreatedAt:     1000,
	}

	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing oauth access token error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetOAuthAccessToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetOAuthAccessToken(ctx, token.TokenHash)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, token) {
		t.Fatalf("oauth access token mismatch:\n got=%#v\nwant=%#v", got, token)
	}
	if err := store.DeleteOAuthAccessToken(ctx, token.TokenHash); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted oauth access token error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetOAuthAccessToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	server.FastForward(time.Minute)
	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("expired oauth access token error=%v, want ErrCacheMiss", err)
	}

	value, found, err := store.GetPermissionCache(ctx, "subject-1")
	if err != nil || found || value != "" {
		t.Fatalf("missing permission cache mismatch: value=%q found=%v err=%v", value, found, err)
	}
	if err := store.SetPermissionCache(ctx, "subject-1", "encoded-permissions", time.Minute); err != nil {
		t.Fatal(err)
	}
	value, found, err = store.GetPermissionCache(ctx, "subject-1")
	if err != nil || !found || value != "encoded-permissions" {
		t.Fatalf("permission cache mismatch: value=%q found=%v err=%v", value, found, err)
	}
	if err := store.DeletePermissionCache(ctx, "subject-1"); err != nil {
		t.Fatal(err)
	}
	value, found, err = store.GetPermissionCache(ctx, "subject-1")
	if err != nil || found || value != "" {
		t.Fatalf("deleted permission cache mismatch: value=%q found=%v err=%v", value, found, err)
	}
}
