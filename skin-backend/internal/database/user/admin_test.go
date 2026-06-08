package user_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"
)

func TestAdminTogglesBanAndUnban(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	u := testutil.CreateUser(t, db, "domain-user-admin@test.com", "Password123", "DomainUserAdmin", false)
	next, err := store.ToggleAdmin(ctx, u.ID)
	if err != nil || !next {
		t.Fatalf("toggle should enable admin: next=%v err=%v", next, err)
	}
	if err := store.Ban(ctx, u.ID, 9_999_999_999_999); err != nil {
		t.Fatal(err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || !banned {
		t.Fatalf("user should be banned: banned=%v err=%v", banned, err)
	}
	if err := store.Unban(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if banned, err := store.IsBanned(ctx, u.ID); err != nil || banned {
		t.Fatalf("user should be unbanned: banned=%v err=%v", banned, err)
	}
}
