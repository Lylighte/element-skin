package account_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
)

func TestAccountServiceDeleteSelfRejectsProtectedRoleAndDeletesPlainUserExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	protected := testutil.CreateUser(t, db, "self-delete-protected@test.com", "Password123", "SelfDeleteProtected", true, true)
	plain := testutil.CreateUser(t, db, "self-delete-plain@test.com", "Password123", "SelfDeletePlain", false)

	if err := svc.DeleteSelf(ctx, actorWithPermissions(protected.ID, "account.delete.self")); !httpErrorIs(err, http.StatusForbidden, "protected subjects cannot delete their own account") {
		t.Fatalf("protected self delete mismatch: %#v", err)
	}
	if got, err := db.Users.GetByID(ctx, protected.ID); err != nil || got == nil {
		t.Fatalf("protected self delete must keep user: user=%#v err=%v", got, err)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: plain.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "self_delete_ygg", UserID: plain.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	client := createAccountOAuthClient(t, db, plain.ID, "self-delete-owned-client", "account.read.self")
	grant := model.OAuthGrant{
		ID:        "self-delete-grant",
		UserID:    plain.ID,
		SubjectID: permissiondb.SubjectIDForUser(plain.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 4100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
		TokenHash: "self-delete-refresh",
		ClientID:  client.ID,
		UserID:    plain.ID,
		GrantID:   grant.ID,
		ExpiresAt: 9000,
		CreatedAt: 4200,
	}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSelf(ctx, actorWithPermissions(plain.ID, "account.delete.self")); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Users.GetByID(ctx, plain.ID); err != nil || got != nil {
		t.Fatalf("DeleteSelf should remove plain user exactly: user=%#v err=%v", got, err)
	}
	if _, err := cache.GetAuthUser(ctx, plain.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("DeleteSelf should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "self_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("DeleteSelf should revoke ygg tokens exactly, got %v", err)
	}
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, client.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForClient(client.ID), 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, grant.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE token_hash=$1`, "self-delete-refresh", 0)
}

func TestAccountServiceDeleteSelfPreservesUserWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "self-delete-ygg-fail@test.com", "Password123", "SelfDeleteYggFail", false)
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	err := svc.DeleteSelf(ctx, actorWithPermissions(user.ID, "account.delete.self"))
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("DeleteSelf ygg revocation failure should be returned exactly, got %v", err)
	}
	if got, err := db.Users.GetByID(ctx, user.ID); err != nil || got == nil || got.Email != user.Email {
		t.Fatalf("failed DeleteSelf must preserve user: user=%#v err=%v", got, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("DeleteSelf should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}
