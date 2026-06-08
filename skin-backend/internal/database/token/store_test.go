package token_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/token"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestStoreTokensSessionsAndRefreshLifecycle(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := token.Store{Pool: db.Pool}
	user := testutil.CreateUser(t, db, "domain-token@test.com", "Password123", "DomainToken", false)
	profile := testutil.CreateProfile(t, db, user.ID, "domain_token_profile", "DomainTokenProfile")
	if err := store.Add(ctx, model.Token{AccessToken: "domain_old_access", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if err := store.Add(ctx, model.Token{AccessToken: "domain_new_access", ClientToken: "client", UserID: user.ID, CreatedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if err := store.Cleanup(ctx, user.ID, 15, 1); err != nil {
		t.Fatal(err)
	}
	if old, err := store.Get(ctx, "domain_old_access"); err != nil || old != nil {
		t.Fatalf("old token should be removed: token=%#v err=%v", old, err)
	}
	if newer, err := store.Get(ctx, "domain_new_access"); err != nil || newer == nil || newer.ProfileID != nil {
		t.Fatalf("new token mismatch: token=%#v err=%v", newer, err)
	}
	if err := store.ReplaceSession(ctx, model.Session{ServerID: "domain_server", AccessToken: "domain_new_access", CreatedAt: 30}); err != nil {
		t.Fatal(err)
	}
	sess, err := store.GetSession(ctx, "domain_server")
	if err != nil || sess == nil || sess.AccessToken != "domain_new_access" || sess.CreatedAt != 30 {
		t.Fatalf("session mismatch: session=%#v err=%v", sess, err)
	}
	if err := store.AddRefresh(ctx, "domain_refresh", user.ID, 1000, 40); err != nil {
		t.Fatal(err)
	}
	refresh, err := store.GetRefresh(ctx, "domain_refresh")
	if err != nil || refresh["user_id"] != user.ID {
		t.Fatalf("refresh mismatch: refresh=%#v err=%v", refresh, err)
	}
	consumed, err := store.ConsumeRefresh(ctx, "domain_refresh")
	if err != nil || consumed["token_hash"] != "domain_refresh" {
		t.Fatalf("consume mismatch: refresh=%#v err=%v", consumed, err)
	}
	if again, err := store.ConsumeRefresh(ctx, "domain_refresh"); err != nil || again != nil {
		t.Fatalf("refresh should be single-use: refresh=%#v err=%v", again, err)
	}
}
