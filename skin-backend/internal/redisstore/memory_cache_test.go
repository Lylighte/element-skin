package redisstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreCachesAndInvalidatesPublicData(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty setting should miss, got %v", err)
	}
	if err := store.SetSetting(ctx, "site_name", "Cached Setting", time.Minute); err != nil {
		t.Fatal(err)
	}
	setting, err := store.GetSetting(ctx, "site_name")
	if err != nil || setting != "Cached Setting" {
		t.Fatalf("setting cache mismatch: %q err=%v", setting, err)
	}
	if err := store.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated setting should miss, got %v", err)
	}

	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty settings should miss, got %v", err)
	}
	if err := store.SetPublicSettings(ctx, map[string]any{"site_name": "Cached"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPublicSettings(ctx)
	if err != nil || got["site_name"] != "Cached" {
		t.Fatalf("settings cache mismatch: %#v err=%v", got, err)
	}
	got["site_name"] = "mutated"
	again, _ := store.GetPublicSettings(ctx)
	if again["site_name"] != "Cached" {
		t.Fatalf("cache should return cloned data, got %#v", again)
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "a", Type: "image", StoragePath: "a.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	homepageMedia, err := store.GetPublicHomepageMedia(ctx)
	if err != nil || len(homepageMedia) != 1 || homepageMedia[0].StoragePath != "a.png" {
		t.Fatalf("homepage media cache mismatch: %#v err=%v", homepageMedia, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated settings should miss, got %v", err)
	}
	if err := store.InvalidatePublicHomepageMedia(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated homepage media should miss, got %v", err)
	}
}

func TestMemoryStoreAuthCacheLifecycle(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	until := time.Now().Add(time.Hour).UnixMilli()
	auth := redisstore.AuthUser{ID: "u1", BannedUntil: &until}
	if err := store.SetAuthUser(ctx, auth, time.Minute); err != nil {
		t.Fatal(err)
	}
	cached, err := store.GetAuthUser(ctx, "u1")
	if err != nil || !cached.Banned(time.Now()) {
		t.Fatalf("auth cache mismatch: %#v err=%v", cached, err)
	}
	if err := store.InvalidateAuthUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAuthUser(ctx, "u1"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated auth cache should miss, got %v", err)
	}
}
