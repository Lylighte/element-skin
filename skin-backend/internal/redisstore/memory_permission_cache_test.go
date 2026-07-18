package redisstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStorePermissionCacheLifecycleAndErrors(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	value, found, err := store.GetPermissionCache(ctx, "subject-1")
	if err != nil || found || value != "" {
		t.Fatalf("missing permission cache mismatch: value=%q found=%v err=%v", value, found, err)
	}
	if err := store.SetPermissionCache(ctx, "subject-1", "encoded-permissions", time.Minute); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 1 {
		t.Fatalf("memory store length=%d, want 1", store.Len())
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
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	for _, op := range []struct {
		name string
		run  func() error
	}{
		{name: "set", run: func() error { return store.SetPermissionCache(ctx, "subject-1", "x", time.Minute) }},
		{name: "delete", run: func() error { return store.DeletePermissionCache(ctx, "subject-1") }},
		{name: "oauth delete", run: func() error { return store.DeleteOAuthAccessToken(ctx, "access-hash-1") }},
	} {
		t.Run(op.name, func(t *testing.T) {
			store.Err = errors.New("forced memory error")
			if err := op.run(); err == nil || err.Error() != "forced memory error" {
				t.Fatalf("%s should return forced memory error, got %v", op.name, err)
			}
			store.Err = nil
		})
	}
	store.Err = errors.New("forced memory error")
	if _, found, err := store.GetPermissionCache(ctx, "subject-1"); err == nil || found {
		t.Fatalf("permission cache get should return forced error and found=false: found=%v err=%v", found, err)
	}
}
