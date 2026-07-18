package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestStoreMethodsReturnExactClosedPoolErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	db.Close()
	hash, hashErr := util.HashPassword("ClosedPoolPassword123")
	if hashErr != nil {
		t.Fatal(hashErr)
	}

	if got, err := store.GetByEmail(ctx, "closed-pool@test.com"); got == nil || got.ID != "" || got.Email != "" || err == nil || err.Error() != "closed pool" {
		t.Fatalf("GetByEmail closed pool = user=%#v err=%v; want zero user and closed pool", got, err)
	}
	if got, err := store.GetByID(ctx, "closed-user"); got == nil || got.ID != "" || got.Email != "" || err == nil || err.Error() != "closed pool" {
		t.Fatalf("GetByID closed pool = user=%#v err=%v; want zero user and closed pool", got, err)
	}
	if err := store.Create(ctx, model.User{ID: "closed-create", Email: "closed-create@test.com", Password: hash, DisplayName: "ClosedCreate"}); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Create closed pool error=%v; want closed pool", err)
	}
	if err := store.CreateWithProfile(ctx,
		model.User{ID: "closed-create-profile", Email: "closed-create-profile@test.com", Password: hash, DisplayName: "ClosedCreateProfile"},
		model.Profile{ID: "closed-profile", UserID: "closed-create-profile", Name: "ClosedProfile", TextureModel: "default"},
		"", "",
	); err == nil || err.Error() != "closed pool" {
		t.Fatalf("CreateWithProfile closed pool error=%v; want closed pool", err)
	}
	if count, err := store.Count(ctx); count != 0 || err == nil || err.Error() != "closed pool" {
		t.Fatalf("Count closed pool = count=%d err=%v; want 0 and closed pool", count, err)
	}
	if taken, err := store.IsDisplayNameTaken(ctx, "ClosedName", ""); taken || err == nil || err.Error() != "closed pool" {
		t.Fatalf("IsDisplayNameTaken closed pool = taken=%v err=%v; want false and closed pool", taken, err)
	}
	if err := store.Update(ctx, "closed-update", map[string]any{"email": "closed-update@test.com"}); err == nil || err.Error() != "closed pool" {
		t.Fatalf("Update closed pool error=%v; want closed pool", err)
	}
	if err := store.UpdatePassword(ctx, "closed-update-password", hash); err == nil || err.Error() != "closed pool" {
		t.Fatalf("UpdatePassword closed pool error=%v; want closed pool", err)
	}
	if updated, err := store.UpdatePasswordAndRevokeRefresh(ctx, "closed-update-password-refresh", hash); updated || err == nil || err.Error() != "closed pool" {
		t.Fatalf("UpdatePasswordAndRevokeRefresh closed pool = updated=%v err=%v; want false and closed pool", updated, err)
	}
	if deleted, err := store.Delete(ctx, "closed-delete"); deleted || err == nil || err.Error() != "closed pool" {
		t.Fatalf("Delete closed pool = deleted=%v err=%v; want false and closed pool", deleted, err)
	}
	if banned, err := store.IsBanned(ctx, "closed-ban"); banned || err == nil || err.Error() != "closed pool" {
		t.Fatalf("IsBanned closed pool = banned=%v err=%v; want false and closed pool", banned, err)
	}
}

func TestIsEmailConflictMatchesOnlyUsersEmailConstraint(t *testing.T) {
	emailConflict := &pgconn.PgError{Code: "23505", ConstraintName: "users_email_key"}
	otherConflict := &pgconn.PgError{Code: "23505", ConstraintName: "profiles_name_key"}
	if !user.IsEmailConflict(emailConflict) {
		t.Fatal("users_email_key 23505 should be recognized as an email conflict")
	}
	if user.IsEmailConflict(otherConflict) || user.IsEmailConflict(errors.New("duplicate key")) || user.IsEmailConflict(nil) {
		t.Fatal("non-email errors must not be recognized as email conflicts")
	}
}
