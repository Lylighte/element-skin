package yggdrasil_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilValidateRefreshSignoutAndInvalidateEdgeCases(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-edge@test.com", "Password123", "YggEdge", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_edge_profile", "YggEdgeProfile")
	otherUser := testutil.CreateUser(t, db, "ygg-edge-other@test.com", "Password123", "YggEdgeOther", false)
	otherProfile := testutil.CreateProfile(t, db, otherUser.ID, "ygg_edge_other_profile", "YggEdgeOtherProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}
	profileID := profile.ID

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "bound_edge_access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Invalidate(ctx, ""); err != nil {
		t.Fatalf("empty invalidate should be a no-op: %v", err)
	}
	if token, err := redis.GetYggToken(ctx, "bound_edge_access"); err != nil || token.AccessToken != "bound_edge_access" {
		t.Fatalf("empty invalidate must not delete existing tokens: token=%#v err=%v", token, err)
	}
	if err := ygg.Validate(ctx, "bound_edge_access", "wrong-client"); !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("validate should reject wrong client token, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, "bound_edge_access", "client", profile.ID, false); !yggError(err, 400, "IllegalArgumentException", "Access token already has a profile assigned.") {
		t.Fatalf("refresh should reject selecting a profile on already-bound token, got %v", err)
	}

	oldProfileID := profile.ID
	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "expired_edge_access", ClientToken: "client", UserID: user.ID, ProfileID: &oldProfileID, CreatedAt: database.NowMS() - int64(16*24*time.Hour/time.Millisecond)}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := ygg.Validate(ctx, "expired_edge_access", "client"); !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("validate should reject expired token by created_at even if redis key exists, got %v", err)
	}

	if err := redis.SetYggToken(ctx, model.Token{AccessToken: "unbound_edge_access", ClientToken: "client", UserID: user.ID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := ygg.Refresh(ctx, "unbound_edge_access", "wrong-client", "", false); !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("refresh should reject wrong client token, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, "unbound_edge_access", "client", otherProfile.ID, false); !yggError(err, 403, "ForbiddenOperationException", "Invalid profile.") {
		t.Fatalf("refresh should reject selecting a foreign profile, got %v", err)
	}

	if err := ygg.Signout(ctx, user.Email, "wrong-password"); !yggError(err, 403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.") {
		t.Fatalf("signout should reject bad credentials, got %v", err)
	}
}

func TestYggdrasilClosedDatabasePropagatesExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-closed-db@test.com", "Password123", "YggClosedDB", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_closed_db_profile", "YggClosedDBProfile")
	cache := testutil.NewMemoryRedis()
	profileID := profile.ID
	token := model.Token{
		AccessToken: "closed_db_access",
		ClientToken: "closed_db_client",
		UserID:      user.ID,
		ProfileID:   &profileID,
		CreatedAt:   database.NowMS(),
	}
	if err := cache.SetYggToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "authenticate", call: func() error {
			_, err := ygg.Authenticate(ctx, user.Email, "Password123", "client", false)
			return err
		}},
		{name: "refresh", call: func() error {
			_, err := ygg.Refresh(ctx, token.AccessToken, token.ClientToken, "", false)
			return err
		}},
		{name: "validate", call: func() error {
			return ygg.Validate(ctx, token.AccessToken, token.ClientToken)
		}},
		{name: "token", call: func() error {
			_, err := ygg.Token(ctx, token.AccessToken)
			return err
		}},
		{name: "invalidate", call: func() error {
			return ygg.Invalidate(ctx, token.AccessToken)
		}},
		{name: "signout", call: func() error {
			return ygg.Signout(ctx, user.Email, "Password123")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil || !strings.Contains(err.Error(), "closed pool") {
				t.Fatalf("%s closed DB error mismatch: %v", tc.name, err)
			}
		})
	}
}
