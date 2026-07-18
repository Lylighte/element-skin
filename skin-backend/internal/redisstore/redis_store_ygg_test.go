package redisstore

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
)

func TestRedisStoreYggTokenAtomicLifecycleAndIndexes(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	profileID := "profile-1"
	if err := store.TrimYggTokensByUser(ctx, "missing-user", 2); err != nil {
		t.Fatalf("trimming a missing user should be a no-op: %v", err)
	}
	tokens := []model.Token{
		{AccessToken: "access-1", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 1},
		{AccessToken: "access-2", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 2},
		{AccessToken: "access-3", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 3},
		{AccessToken: "access-4", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 4},
	}
	for _, token := range tokens {
		if err := store.SetYggToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := store.GetYggToken(ctx, "access-2"); err != nil || got.UserID != "user-1" ||
		got.ProfileID == nil || *got.ProfileID != profileID || got.CreatedAt != 2 {
		t.Fatalf("stored token=%#v err=%v", got, err)
	}

	replacement := model.Token{AccessToken: "access-new", ClientToken: "client", UserID: "user-1", ProfileID: &profileID, CreatedAt: 5}
	replaced, err := store.ReplaceYggToken(ctx, "access-2", replacement, time.Minute)
	if err != nil || !replaced {
		t.Fatalf("replace result=%v err=%v, want true nil", replaced, err)
	}
	if _, err := store.GetYggToken(ctx, "access-2"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("old token should be removed atomically, got %v", err)
	}
	if got, err := store.GetYggToken(ctx, replacement.AccessToken); err != nil || got.CreatedAt != replacement.CreatedAt {
		t.Fatalf("replacement token=%#v err=%v", got, err)
	}
	if replaced, err := store.ReplaceYggToken(ctx, "missing", replacement, time.Minute); err != nil || replaced {
		t.Fatalf("replace missing result=%v err=%v, want false nil", replaced, err)
	}

	if err := store.TrimYggTokensByUser(ctx, "user-1", 2); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access-1", "access-3"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, ErrCacheMiss) {
			t.Fatalf("%s should be trimmed, got %v", access, err)
		}
	}
	for _, access := range []string{"access-4", "access-new"} {
		if got, err := store.GetYggToken(ctx, access); err != nil || got.AccessToken != access {
			t.Fatalf("newest token %s should remain: token=%#v err=%v", access, got, err)
		}
	}

	if err := store.DeleteYggToken(ctx, "access-new"); err != nil {
		t.Fatal(err)
	}
	if err := store.DeleteYggToken(ctx, "access-new"); err != nil {
		t.Fatalf("deleting a missing token should be idempotent: %v", err)
	}
	if err := store.DeleteYggTokensByUser(ctx, "user-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, "access-4"); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("user token deletion should remove remaining tokens, got %v", err)
	}

	finalToken := model.Token{AccessToken: "access-final", ClientToken: "client", UserID: "user-1", CreatedAt: 6}
	if err := store.SetYggToken(ctx, finalToken, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := store.TrimYggTokensByUser(ctx, "user-1", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetYggToken(ctx, finalToken.AccessToken); !errors.Is(err, ErrCacheMiss) {
		t.Fatalf("keep=0 should remove every token, got %v", err)
	}
}
