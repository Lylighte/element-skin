package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestRedisStoreCacheRoundTripsNormalizationAndTTL(t *testing.T) {
	store, server := newTestRedisStore(t)
	ctx := context.Background()

	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("missing setting error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetSetting(ctx, "site_name", "Redis Site", time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetSetting(ctx, "site_name"); err != nil || got != "Redis Site" {
		t.Fatalf("setting=%q err=%v, want Redis Site", got, err)
	}

	public := map[string]any{"site_name": "Redis Site", "allow_register": true}
	if err := store.SetPublicSettings(ctx, public, time.Minute); err != nil {
		t.Fatal(err)
	}
	gotPublic, err := store.GetPublicSettings(ctx)
	if err != nil || gotPublic["site_name"] != "Redis Site" || gotPublic["allow_register"] != true {
		t.Fatalf("public settings=%#v err=%v", gotPublic, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated public settings error=%v, want ErrCacheMiss", err)
	}
	if err := store.SetPublicSettings(ctx, public, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "one", Type: "image", StoragePath: "one.png"}, {ID: "two", Type: "image", StoragePath: "two.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetPublicHomepageMedia(ctx); err != nil || len(got) != 2 || got[0].StoragePath != "one.png" || got[1].StoragePath != "two.png" {
		t.Fatalf("homepage media=%#v err=%v", got, err)
	}

	if err := store.SetVerificationCode(ctx, " User@Example.com ", " RESET ", "ABC12345", time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || got != "ABC12345" {
		t.Fatalf("normalized verification code=%q err=%v", got, err)
	}
	if consumed, err := store.ConsumeVerificationCode(ctx, "USER@EXAMPLE.COM", "RESET", "wrong"); err != nil || consumed {
		t.Fatalf("wrong verification consumption=%v err=%v, want false nil", consumed, err)
	}
	if consumed, err := store.ConsumeVerificationCode(ctx, "USER@EXAMPLE.COM", "RESET", "abc12345"); err != nil || !consumed {
		t.Fatalf("matching verification consumption=%v err=%v, want true nil", consumed, err)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("consumed verification code error=%v, want ErrCacheMiss", err)
	}
	if set, err := store.SetVerificationCodeIfAbsent(ctx, "user@example.com", "reset", "NEWCODE1", time.Minute); err != nil || !set {
		t.Fatalf("set missing verification code=%v err=%v, want true nil", set, err)
	}
	if set, err := store.SetVerificationCodeIfAbsent(ctx, "user@example.com", "reset", "OLDCODE1", time.Minute); err != nil || set {
		t.Fatalf("set existing verification code=%v err=%v, want false nil", set, err)
	}
	if got, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || got != "NEWCODE1" {
		t.Fatalf("set-if-absent overwrote code=%q err=%v", got, err)
	}
	if err := store.DeleteVerificationCode(ctx, "USER@example.com", "RESET"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("deleted verification code error=%v, want ErrCacheMiss", err)
	}

	until := time.Now().Add(time.Hour).UnixMilli()
	auth := AuthUser{ID: "user-1", BannedUntil: &until}
	if err := store.SetAuthUser(ctx, auth, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetAuthUser(ctx, auth.ID); err != nil || got.ID != auth.ID ||
		got.BannedUntil == nil || *got.BannedUntil != until {
		t.Fatalf("auth cache=%#v err=%v", got, err)
	}
	if err := store.InvalidateAuthUser(ctx, auth.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAuthUser(ctx, auth.ID); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("invalidated auth cache error=%v, want ErrCacheMiss", err)
	}

	ip := "203.0.113.7"
	session := model.Session{ServerID: "server-1", AccessToken: "access-1", IP: &ip, CreatedAt: 123}
	if err := store.SetYggSession(ctx, session, time.Minute); err != nil {
		t.Fatal(err)
	}
	if got, err := store.GetYggSession(ctx, session.ServerID); err != nil || got.ServerID != session.ServerID ||
		got.AccessToken != session.AccessToken || got.IP == nil || *got.IP != ip || got.CreatedAt != session.CreatedAt {
		t.Fatalf("session=%#v err=%v", got, err)
	}

	server.FastForward(time.Minute)
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("public settings should expire at TTL boundary, got %v", err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("homepage media should expire at TTL boundary, got %v", err)
	}
	if _, err := store.GetYggSession(ctx, session.ServerID); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("session should expire at TTL boundary, got %v", err)
	}
}
