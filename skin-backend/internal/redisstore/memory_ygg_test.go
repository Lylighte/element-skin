package redisstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreYggTokenLifecycleAndTrim(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	profileID := "p1"

	for i := 1; i <= 4; i++ {
		if err := store.SetYggToken(ctx, model.Token{
			AccessToken: "access_" + string(rune('0'+i)),
			ClientToken: "client",
			UserID:      "u1",
			ProfileID:   &profileID,
			CreatedAt:   int64(i),
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.TrimYggTokensByUser(ctx, "u1", 2); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_1", "access_2"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be trimmed, got %v", access, err)
		}
	}
	for _, access := range []string{"access_3", "access_4"} {
		token, err := store.GetYggToken(ctx, access)
		if err != nil || token.UserID != "u1" || token.ProfileID == nil || *token.ProfileID != profileID {
			t.Fatalf("%s should remain: %#v err=%v", access, token, err)
		}
	}

	replaced, err := store.ReplaceYggToken(ctx, "access_3", model.Token{
		AccessToken: "access_new",
		ClientToken: "client",
		UserID:      "u1",
		ProfileID:   &profileID,
		CreatedAt:   5,
	}, time.Minute)
	if err != nil || !replaced {
		t.Fatalf("replace should succeed: replaced=%v err=%v", replaced, err)
	}
	if _, err := store.GetYggToken(ctx, "access_3"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old token should miss after replace, got %v", err)
	}
	if token, err := store.GetYggToken(ctx, "access_new"); err != nil || token.UserID != "u1" {
		t.Fatalf("new token mismatch: %#v err=%v", token, err)
	}

	if err := store.DeleteYggTokensByUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_4", "access_new"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be deleted by user, got %v", access, err)
		}
	}
}

func TestMemoryStoreYggSessionTTL(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	if _, err := store.GetYggSession(ctx, "missing-server"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("missing ygg session should miss, got %v", err)
	}
	ip := "127.0.0.1"
	session := model.Session{ServerID: "active-server", AccessToken: "active-access", IP: &ip, CreatedAt: 1234}
	if err := store.SetYggSession(ctx, session, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetYggSession(ctx, "active-server")
	if err != nil || got.ServerID != session.ServerID || got.AccessToken != session.AccessToken || got.IP == nil || *got.IP != ip || got.CreatedAt != session.CreatedAt {
		t.Fatalf("active ygg session mismatch: got=%#v err=%v want=%#v", got, err, session)
	}
	if err := store.SetYggSession(ctx, model.Session{ServerID: "server", AccessToken: "access", CreatedAt: 1}, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.GetYggSession(ctx, "server"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("expired ygg session should miss, got %v", err)
	}
}

func TestMemoryStoreYggOperationsReturnBackingErrorsExactly(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	forced := errors.New("forced ygg memory error")
	store.Err = forced

	if err := store.SetYggToken(ctx, model.Token{AccessToken: "access", UserID: "user", CreatedAt: 1}, time.Minute); !errors.Is(err, forced) {
		t.Fatalf("SetYggToken error=%v, want forced", err)
	}
	if _, err := store.GetYggToken(ctx, "access"); !errors.Is(err, forced) {
		t.Fatalf("GetYggToken error=%v, want forced", err)
	}
	if replaced, err := store.ReplaceYggToken(ctx, "old", model.Token{AccessToken: "new", UserID: "user", CreatedAt: 2}, time.Minute); !errors.Is(err, forced) || replaced {
		t.Fatalf("ReplaceYggToken mismatch: replaced=%v err=%v", replaced, err)
	}
	if err := store.DeleteYggToken(ctx, "access"); !errors.Is(err, forced) {
		t.Fatalf("DeleteYggToken error=%v, want forced", err)
	}
	if err := store.DeleteYggTokensByUser(ctx, "user"); !errors.Is(err, forced) {
		t.Fatalf("DeleteYggTokensByUser error=%v, want forced", err)
	}
	if err := store.TrimYggTokensByUser(ctx, "user", 0); !errors.Is(err, forced) {
		t.Fatalf("TrimYggTokensByUser error=%v, want forced", err)
	}
	if err := store.SetYggSession(ctx, model.Session{ServerID: "server"}, time.Minute); !errors.Is(err, forced) {
		t.Fatalf("SetYggSession error=%v, want forced", err)
	}
	if _, err := store.GetYggSession(ctx, "server"); !errors.Is(err, forced) {
		t.Fatalf("GetYggSession error=%v, want forced", err)
	}
}
