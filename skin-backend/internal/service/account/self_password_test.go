package account_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountServiceChangePasswordPreservesStateWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "change-password-ygg-fail@test.com", "Password123", "ChangePasswordYggFail", false)
	const refreshHash = "change_password_ygg_fail_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	err := svc.ChangePasswordSelf(ctx, accountActor(t, db, user.ID), "Password123", "NewPassword123")
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("ygg revocation failure should be returned exactly, got %v", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("NewPassword123", unchanged.Password) {
		t.Fatalf("failed password change must preserve old hash: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh == nil || refresh["user_id"] != user.ID {
		t.Fatalf("failed password change must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("password change should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

func TestAccountServiceChangePasswordSelfRevokesTokensAndInvalidatesCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "change-password-success@test.com", "Password123", "ChangePasswordSuccess", false)
	const refreshHash = "change_password_success_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: user.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "change_password_success_ygg", UserID: user.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePasswordSelf(ctx, actorWithPermissions(user.ID, "account_password.update.self"), "Password123", "NewPassword123"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || updated == nil || !util.VerifyPassword("NewPassword123", updated.Password) || util.VerifyPassword("Password123", updated.Password) {
		t.Fatalf("successful password change should persist exact hash: user=%#v err=%v", updated, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh != nil {
		t.Fatalf("successful password change should revoke refresh token: refresh=%#v err=%v", refresh, err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful password change should invalidate auth cache, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "change_password_success_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful password change should revoke ygg token, got %v", err)
	}
}
