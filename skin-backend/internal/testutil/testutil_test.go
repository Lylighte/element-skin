package testutil

import (
	"context"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

func TestTestConfigExactDefaults(t *testing.T) {
	t.Setenv("TEST_DATABASE_DSN", "postgresql://example/test")
	cfg := TestConfig()
	if cfg.DatabaseDSN != "postgresql://example/test" || cfg.JWTSecret != "abcdefghijklmnopqrstuvwxyz123456" ||
		cfg.SiteURL != "http://test" || cfg.APIURL != "http://localhost:8000" {
		t.Fatalf("TestConfig mismatch: %#v", cfg)
	}
	if !strings.HasSuffix(cfg.PrivateKeyPath, "private.pem") || !strings.HasSuffix(cfg.PublicKeyPath, "public.pem") {
		t.Fatalf("TestConfig should point at Yggdrasil test keys: %#v", cfg)
	}
}

func TestNewTestAppCreateHelpersExactState(t *testing.T) {
	db, handler := NewTestApp(t)
	if handler == nil {
		t.Fatal("NewTestApp should return a handler")
	}
	ctx := context.Background()
	user := CreateUser(t, db, "", "Password123", "", true)
	emailLocal := strings.TrimSuffix(user.Email, "@example.com")
	displaySuffix := strings.TrimPrefix(user.DisplayName, "User_")
	if len(emailLocal) != 8 || !strings.HasSuffix(user.Email, "@example.com") || !strings.HasPrefix(user.DisplayName, "User_") || len(displaySuffix) != 8 {
		t.Fatalf("CreateUser generated fields mismatch: %#v", user)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, user.ID, "admin"); err != nil || !hasRole {
		t.Fatalf("CreateUser admin role mismatch: hasRole=%v err=%v", hasRole, err)
	}
	stored, err := db.Users.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Email != user.Email || !util.VerifyPassword("Password123", stored.Password) {
		t.Fatalf("CreateUser did not persist expected user: %#v", stored)
	}
	profile := CreateProfile(t, db, user.ID, "", "GeneratedProfile")
	if profile.UserID != user.ID || profile.Name != "GeneratedProfile" || profile.TextureModel != "default" || len(profile.ID) != 32 {
		t.Fatalf("CreateProfile generated fields mismatch: %#v", profile)
	}
	if ok, err := db.Profiles.VerifyOwnership(ctx, user.ID, profile.ID); err != nil || !ok {
		t.Fatalf("CreateProfile should persist ownership: ok=%v err=%v", ok, err)
	}
}

func TestNewTestAppWithMaxConnections(t *testing.T) {
	db, handler := NewTestAppWithMaxConnectionsTB(t, 12)
	if handler == nil {
		t.Fatal("NewTestAppWithMaxConnectionsTB should return a handler")
	}
	if got := db.Pool.Stat().MaxConns(); got != 12 {
		t.Fatalf("MaxConns mismatch: got=%d want=12", got)
	}
}

func TestNewTestAppVariantsReturnExactDatabaseHandlerAndRedisState(t *testing.T) {
	db, handler := NewTestAppTB(t)
	if db == nil || handler == nil {
		t.Fatalf("NewTestAppTB returned db=%#v handler=%#v", db, handler)
	}
	if count, err := db.Users.Count(context.Background()); err != nil || count != 0 {
		t.Fatalf("fresh NewTestAppTB user count=%d err=%v; want 0, nil", count, err)
	}

	db, handler, cache := NewTestAppWithRedisTB(t)
	if db == nil || handler == nil || cache == nil {
		t.Fatalf("NewTestAppWithRedisTB returned db=%#v handler=%#v cache=%#v", db, handler, cache)
	}
	if err := cache.SetSetting(context.Background(), "exact-test-key", "exact-value", time.Minute); err != nil {
		t.Fatal(err)
	}
	value, err := cache.GetSetting(context.Background(), "exact-test-key")
	if err != nil || value != "exact-value" {
		t.Fatalf("memory redis setting mismatch: value=%q err=%v", value, err)
	}

	db, handler, cache = NewTestAppWithMaxConnectionsAndRedisTB(t, 5)
	if handler == nil || cache == nil {
		t.Fatalf("NewTestAppWithMaxConnectionsAndRedisTB handler/cache nil: handler=%#v cache=%#v", handler, cache)
	}
	if got := db.Pool.Stat().MaxConns(); got != 5 {
		t.Fatalf("max connections with redis mismatch: got=%d want=5", got)
	}
}

func TestNewRedisStoreTBStoresAndCleansExactPrefixedKeys(t *testing.T) {
	store := NewRedisStoreTB(t, "elementskin:test:new-redis-store:")
	ctx := context.Background()
	if err := store.SetSetting(ctx, "redis-store-key", "redis-store-value", time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetSetting(ctx, "redis-store-key")
	if err != nil || got != "redis-store-value" {
		t.Fatalf("NewRedisStoreTB setting mismatch: got=%q err=%v", got, err)
	}
}

func TestEnsureTestDatabaseIsIdempotent(t *testing.T) {
	ctx := context.Background()
	dbName := "elementskin_go_test_idempotent"
	t.Cleanup(func() { dropTestDatabase(t, context.Background(), dbName) })
	ensureTestDatabase(t, ctx, dbName)
	ensureTestDatabase(t, ctx, dbName)
	cfg := TestConfig()
	cfg.DatabaseDSN = "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
	db, err := database.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
}
