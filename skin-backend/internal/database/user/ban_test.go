package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestUserBanAndUnbanExactLifecycle(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	u := testutil.CreateUser(t, db, "domain-user-ban@test.com", "Password123", "DomainUserBan", false)

	if banned, err := store.IsBanned(ctx, u.ID); err != nil || banned {
		t.Fatalf("new user ban state = %v, %v; want false, nil", banned, err)
	}
	if err := store.Ban(ctx, u.ID, 9_999_999_999_999); err != nil {
		t.Fatalf("ban existing user: %v", err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || !banned {
		t.Fatalf("banned user state = %v, %v; want true, nil", banned, err)
	}
	if err := store.Unban(ctx, u.ID); err != nil {
		t.Fatalf("unban existing user: %v", err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || banned {
		t.Fatalf("unbanned user state = %v, %v; want false, nil", banned, err)
	}
}

func TestUserBanAndUnbanMissingUserReturnNoRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}

	if err := store.Ban(ctx, "missing-user", 9_999_999_999_999); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user ban error = %v; want pgx.ErrNoRows", err)
	}
	if err := store.Unban(ctx, "missing-user"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing user unban error = %v; want pgx.ErrNoRows", err)
	}
	if banned, err := store.IsBanned(ctx, "missing-user"); err != nil || banned {
		t.Fatalf("missing user ban state = %v, %v; want false, nil", banned, err)
	}
}
