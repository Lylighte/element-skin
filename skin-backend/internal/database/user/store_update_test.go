package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestUpdateRollsBackAllUserFieldsWhenOneFieldFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "user-update-atomic@test.com", "Password123", "UserUpdateAtomic", false)
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE users
		ADD CONSTRAINT preferred_language_zh_only CHECK (preferred_language = 'zh_CN')
	`); err != nil {
		t.Fatal(err)
	}

	err := store.Update(ctx, target.ID, map[string]any{
		"email":              "changed-update-atomic@test.com",
		"display_name":       "ChangedUpdateAtomic",
		"preferred_language": "en_US",
	})
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("Update error = %#v; want PostgreSQL 23514", err)
	}
	got, err := store.GetByID(ctx, target.ID)
	if err != nil || got == nil ||
		got.Email != target.Email ||
		got.DisplayName != target.DisplayName ||
		got.PreferredLanguage != target.PreferredLanguage {
		t.Fatalf("failed update changed user: user=%#v err=%v want=%#v", got, err, target)
	}
}

func TestUpdateIgnoresUnsupportedFieldsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "user-update-ignored@test.com", "Password123", "UserUpdateIgnored", false)

	if err := store.Update(ctx, target.ID, map[string]any{"unknown_field": "changed"}); err != nil {
		t.Fatalf("unsupported-only update should be a no-op, got %v", err)
	}
	got, err := store.GetByID(ctx, target.ID)
	if err != nil || got == nil ||
		got.ID != target.ID ||
		got.Email != target.Email ||
		got.DisplayName != target.DisplayName ||
		got.PreferredLanguage != target.PreferredLanguage ||
		got.AvatarHash != nil {
		t.Fatalf("unsupported-only update mutated user: user=%#v err=%v want=%#v", got, err, target)
	}
}

func TestUpdateRollsBackWhenDisplayNameConflicts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	existing := testutil.CreateUser(t, db, "user-update-name-existing@test.com", "Password123", "ExistingDisplayName", false)
	target := testutil.CreateUser(t, db, "user-update-name-target@test.com", "Password123", "TargetDisplayName", false)

	err := store.Update(ctx, target.ID, map[string]any{
		"email":        "changed-name-target@test.com",
		"display_name": existing.DisplayName,
	})
	if !errors.Is(err, user.ErrDisplayNameConflict) {
		t.Fatalf("display name conflict error=%v; want ErrDisplayNameConflict", err)
	}
	got, getErr := store.GetByID(ctx, target.ID)
	if getErr != nil || got == nil ||
		got.Email != target.Email ||
		got.DisplayName != target.DisplayName ||
		got.PreferredLanguage != target.PreferredLanguage ||
		got.AvatarHash != nil {
		t.Fatalf("display name conflict mutated target: user=%#v err=%v want=%#v", got, getErr, target)
	}
	takenByExisting, err := store.IsDisplayNameTaken(ctx, existing.DisplayName, existing.ID)
	if err != nil || takenByExisting {
		t.Fatalf("existing user should be excluded from own display name check: taken=%v err=%v", takenByExisting, err)
	}
	takenByTarget, err := store.IsDisplayNameTaken(ctx, existing.DisplayName, target.ID)
	if err != nil || !takenByTarget {
		t.Fatalf("target should see existing display name as taken: taken=%v err=%v", takenByTarget, err)
	}
}
