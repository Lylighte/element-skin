package redisstore_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreOAuthAccessTokenLifecycleAndTTL(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	token := redisstore.OAuthAccessToken{
		TokenHash:     "access-hash-1",
		ClientID:      "client-1",
		UserID:        "user-1",
		GrantID:       "grant-1",
		PermissionIDs: []int64{101, 202},
		ExpiresAt:     5000,
		CreatedAt:     1000,
	}

	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("missing oauth access token should miss, got %v", err)
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
	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("deleted oauth access token should miss, got %v", err)
	}

	if err := store.SetOAuthAccessToken(ctx, token, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.GetOAuthAccessToken(ctx, token.TokenHash); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("expired oauth access token should miss, got %v", err)
	}
}
