package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestSessionRotateRefreshIsSingleUse(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	testutil.CreateUser(t, db, "site-session-service@test.com", "Password123", "SiteSessionService", false)
	login, err := svc.Login(ctx, "site-session-service@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["refresh_token"] == "" || rotated["refresh_token"] == login["refresh_token"] {
		t.Fatalf("rotated refresh should be new and non-empty: %#v", rotated)
	}
	if _, err := svc.RotateRefresh(ctx, login["refresh_token"].(string)); err == nil {
		t.Fatal("old refresh token should be consumed")
	}
}

func TestSessionRotateRefreshRejectsExpiredTokenAndConsumesIt(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-expired@test.com", "Password123", "SiteSessionExpired", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()-1, database.NowMS()-2); err != nil {
		t.Fatal(err)
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if !httpError(err, 401, "refresh token expired") || rotated != nil {
		t.Fatalf("expired refresh should be rejected exactly: rotated=%#v err=%v", rotated, err)
	}
	if row, err := db.Tokens.GetRefresh(ctx, hash); err != nil || row != nil {
		t.Fatalf("expired refresh token should still be consumed on failed rotation: row=%#v err=%v", row, err)
	}
}

func TestSessionRotateRefreshRejectsTokenAfterUserDeletion(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	user := testutil.CreateUser(t, db, "site-session-deleted-user@test.com", "Password123", "SiteSessionDeletedUser", false)
	raw, hash, err := util.GenerateRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(ctx, hash, user.ID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Users.Delete(ctx, user.ID); err != nil || !ok {
		t.Fatalf("delete user mismatch: ok=%v err=%v", ok, err)
	}

	rotated, err := svc.RotateRefresh(ctx, raw)
	if !httpError(err, 401, "invalid refresh token") || rotated != nil {
		t.Fatalf("refresh for deleted user should be rejected exactly: rotated=%#v err=%v", rotated, err)
	}
	if row, err := db.Tokens.GetRefresh(ctx, hash); err != nil || row != nil {
		t.Fatalf("deleting user should remove refresh tokens: row=%#v err=%v", row, err)
	}
}

func TestSessionIssueAndRotateUseConfiguredRefreshLifetime(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	svc := newSiteService(db, cfg)
	testutil.CreateUser(t, db, "site-session-lifetime@test.com", "Password123", "SiteSessionLifetime", true)
	if err := db.Settings.Set(ctx, "jwt_expire_days", "3"); err != nil {
		t.Fatal(err)
	}

	login, err := svc.Login(ctx, "site-session-lifetime@test.com", "Password123")
	if err != nil {
		t.Fatal(err)
	}
	if login["refresh_max_age_seconds"] != 3*24*3600 || login["is_admin"] != true || login["is_super_admin"] != false {
		t.Fatalf("login should use configured refresh lifetime and roles: %#v", login)
	}
	rotated, err := svc.RotateRefresh(ctx, login["refresh_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if rotated["refresh_max_age_seconds"] != 3*24*3600 || rotated["is_admin"] != true || rotated["is_super_admin"] != false {
		t.Fatalf("rotated session should preserve configured refresh lifetime and roles: %#v", rotated)
	}
}
