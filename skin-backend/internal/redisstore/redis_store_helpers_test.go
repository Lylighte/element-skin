package redisstore

import (
	"context"
	"testing"

	"element-skin/backend/internal/config"

	"github.com/alicebob/miniredis/v2"
)

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	server := miniredis.RunT(t)
	cfg := config.Defaults()
	cfg.RedisAddr = server.Addr()
	cfg.RedisPassword = ""
	cfg.RedisDB = 0
	cfg.RedisKeyPrefix = "redisstore:test:"
	store, err := Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close redis store: %v", err)
		}
	})
	return store, server
}
