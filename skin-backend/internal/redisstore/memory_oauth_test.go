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

func TestMemoryStoreOAuthAccessTokenTargetedInvalidationExactly(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	tokens := []redisstore.OAuthAccessToken{
		{TokenHash: "client-a-grant-a", ClientID: "client-a", UserID: "user-a", GrantID: "grant-a"},
		{TokenHash: "client-a-grant-b", ClientID: "client-a", UserID: "user-b", GrantID: "grant-b"},
		{TokenHash: "client-b-grant-a", ClientID: "client-b", UserID: "user-a", GrantID: "grant-c"},
		{TokenHash: "client-b-app", ClientID: "client-b"},
	}
	for _, token := range tokens {
		if err := store.SetOAuthAccessToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.DeleteOAuthAccessTokensByGrant(ctx, "grant-a"); err != nil {
		t.Fatal(err)
	}
	assertOAuthAccessPresence(t, store, map[string]bool{
		"client-a-grant-a": false,
		"client-a-grant-b": true,
		"client-b-grant-a": true,
		"client-b-app":     true,
	})
	if err := store.DeleteOAuthAccessTokensByUser(ctx, "user-a"); err != nil {
		t.Fatal(err)
	}
	assertOAuthAccessPresence(t, store, map[string]bool{
		"client-a-grant-a": false,
		"client-a-grant-b": true,
		"client-b-grant-a": false,
		"client-b-app":     true,
	})
	if err := store.DeleteOAuthAccessTokensByClient(ctx, "client-b"); err != nil {
		t.Fatal(err)
	}
	assertOAuthAccessPresence(t, store, map[string]bool{
		"client-a-grant-a": false,
		"client-a-grant-b": true,
		"client-b-grant-a": false,
		"client-b-app":     false,
	})
}

func assertOAuthAccessPresence(t *testing.T, store redisstore.Store, want map[string]bool) {
	t.Helper()
	for hash, present := range want {
		_, err := store.GetOAuthAccessToken(context.Background(), hash)
		if present && err != nil {
			t.Fatalf("oauth access token %q should exist: %v", hash, err)
		}
		if !present && !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("oauth access token %q should be absent, got %v", hash, err)
		}
	}
}
