package user_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestCreateWithoutProfilePersistsOnlyUser(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	missing, err := store.GetByEmail(ctx, "missing-create-no-profile@test.com")
	if err != nil || missing != nil {
		t.Fatalf("missing GetByEmail = %#v, %v; want nil, nil", missing, err)
	}
	hash, err := util.HashPassword("CreateNoProfile1")
	if err != nil {
		t.Fatal(err)
	}
	u := model.User{ID: "create_no_profile", Email: "create-no-profile@test.com", Password: hash, DisplayName: "CreateNoProfile"}
	if err := store.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetByID(ctx, u.ID)
	if err != nil || got == nil || got.ID != u.ID || got.Email != u.Email {
		t.Fatalf("Create without profile mismatch: got=%#v err=%v", got, err)
	}
	var profileCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=$1`, u.ID).Scan(&profileCount); err != nil {
		t.Fatal(err)
	}
	if profileCount != 0 {
		t.Fatalf("Create without profile should not create profile: got=%d", profileCount)
	}
}

func TestCreateWithProfileRollsBackWhenDisplayNameConflicts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	existing := testutil.CreateUser(t, db, "create-profile-conflict-existing@test.com", "Password123", "CreateProfileConflict", false)
	hash, err := util.HashPassword("CreateProfileConflict1")
	if err != nil {
		t.Fatal(err)
	}

	err = store.CreateWithProfile(ctx,
		model.User{ID: "create_profile_conflict_user", Email: "create-profile-conflict@test.com", Password: hash, DisplayName: existing.DisplayName},
		model.Profile{ID: "create_profile_conflict_profile", UserID: "create_profile_conflict_user", Name: "ConflictProf", TextureModel: "default"},
		"",
		"",
	)
	if !errors.Is(err, user.ErrDisplayNameConflict) {
		t.Fatalf("CreateWithProfile conflict error=%v; want ErrDisplayNameConflict", err)
	}
	if got, err := store.GetByID(ctx, "create_profile_conflict_user"); err != nil || got != nil {
		t.Fatalf("conflicting CreateWithProfile should not insert user: user=%#v err=%v", got, err)
	}
	if got, err := db.Profiles.GetByID(ctx, "create_profile_conflict_profile"); err != nil || got != nil {
		t.Fatalf("conflicting CreateWithProfile should not insert profile: profile=%#v err=%v", got, err)
	}
}
