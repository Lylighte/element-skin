package redisstore

import (
	"context"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/config"
)

func TestRedisStoreRejectsCorruptAndUnencodableJSON(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()
	publicKey := store.key("public", "settings")
	server.Set(publicKey, "{not-json")
	if got, err := store.GetPublicSettings(ctx); err == nil || got != nil {
		t.Fatalf("corrupt cached JSON should return a decode error: got=%#v err=%v", got, err)
	}

	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	if err := store.setJSON(ctx, publicKey, cyclic, time.Minute); err == nil {
		t.Fatal("cyclic JSON value should be rejected before writing to Redis")
	}
	if got, err := server.Get(publicKey); err != nil || got != "{not-json" {
		t.Fatalf("failed JSON encoding must not overwrite existing cache: value=%q err=%v", got, err)
	}
}

func TestRedisStoreOpenReturnsExactConnectionError(t *testing.T) {
	cfg := config.Defaults()
	cfg.RedisAddr = "127.0.0.1:1"
	cfg.RedisPassword = ""
	cfg.RedisDB = 0
	cfg.RedisKeyPrefix = "redisstore:open-fail:"
	store, err := Open(context.Background(), cfg)
	if store != nil {
		t.Fatalf("failed Open should not return a store: %#v", store)
	}
	if err == nil || !strings.Contains(err.Error(), "connect redis 127.0.0.1:1") {
		t.Fatalf("Open error mismatch: %v", err)
	}
}
