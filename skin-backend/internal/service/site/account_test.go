package site_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestAccountMeReturnsCountsAndUpdateMePersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-account-service@test.com", "Password123", "SiteAccountService", false)

	if err := svc.UpdateMe(ctx, user.ID, map[string]any{"email": "updated-account@test.com", "display_name": "UpdatedAccount", "preferred_language": "en_US", "avatar_hash": "avatar_hash"}); err != nil {
		t.Fatal(err)
	}
	me, err := svc.Me(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if me["email"] != "updated-account@test.com" || me["display_name"] != "UpdatedAccount" || me["lang"] != "en_US" ||
		me["profile_count"] != 0 || me["texture_count"] != 0 {
		t.Fatalf("Me response mismatch: %#v", me)
	}
}

func TestAccountRejectsInvalidUpdatesAndWrongPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newSiteService(db, testutil.TestConfig())
	user := testutil.CreateUser(t, db, "site-account-invalid@test.com", "Password123", "SiteAccountInvalid", false)
	other := testutil.CreateUser(t, db, "site-account-invalid-other@test.com", "Password123", "SiteAccountInvalidOther", false)

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"invalid email", map[string]any{"email": "not-an-email"}, "Invalid email format"},
		{"duplicate display name", map[string]any{"display_name": other.DisplayName}, "Username already exists"},
		{"blank display name", map[string]any{"display_name": "   "}, "Username cannot be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateMe(ctx, user.ID, tc.body)
			var httpErr util.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status != 400 || httpErr.Detail != tc.want {
				t.Fatalf("UpdateMe should reject %s exactly, got %#v", tc.name, err)
			}
		})
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email || unchanged.DisplayName != user.DisplayName {
		t.Fatalf("invalid updates should not mutate account: user=%#v err=%v", unchanged, err)
	}

	err = svc.ChangePassword(ctx, user.ID, "WrongPassword", "NewPassword123")
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 403 || httpErr.Detail != "旧密码错误" {
		t.Fatalf("wrong old password should reject exactly, got %#v", err)
	}
	afterWrongPassword, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || afterWrongPassword == nil || !util.VerifyPassword("Password123", afterWrongPassword.Password) {
		t.Fatalf("wrong old password should not change hash: user=%#v err=%v", afterWrongPassword, err)
	}

	err = svc.ChangePassword(ctx, "missing-user", "Password123", "NewPassword123")
	if !errors.As(err, &httpErr) || httpErr.Status != 404 || httpErr.Detail != "用户不存在" {
		t.Fatalf("missing user password change should reject exactly, got %#v", err)
	}
}
