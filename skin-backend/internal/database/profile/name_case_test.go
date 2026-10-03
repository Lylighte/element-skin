package profile_test

import (
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestProfileNamesAreGloballyUniqueIgnoringCase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	owner := testutil.CreateUser(t, db, "profile-case-owner@test.com", "Password123", "CaseOwner", false)
	other := testutil.CreateUser(t, db, "profile-case-other@test.com", "Password123", "CaseOther", false)
	skin, cape := "case_skin", "case_cape"
	original := model.Profile{ID: "case_original", UserID: owner.ID, Name: "CasePlayer", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape}
	if err := db.Profiles.Create(ctx, original); err != nil {
		t.Fatal(err)
	}
	conflicting := model.Profile{ID: "case_duplicate", UserID: other.ID, Name: "caseplayer", TextureModel: "default"}
	assertProfileNameConflict(t, db.Profiles.Create(ctx, conflicting))
	if got, err := db.Profiles.GetByID(ctx, conflicting.ID); err != nil || got != nil {
		t.Fatalf("rejected create persisted profile=%#v err=%v", got, err)
	}

	for _, name := range []string{"CasePlayer", "caseplayer", "CASEPLAYER"} {
		got, err := db.Profiles.GetByName(ctx, name)
		if err != nil || !reflect.DeepEqual(got, &original) {
			t.Fatalf("lookup %q profile=%#v err=%v; want %#v", name, got, err, original)
		}
	}
	got, err := db.Profiles.SearchByNames(ctx, []string{"caseplayer", "CASEPLAYER", "Missing"}, 100)
	if err != nil || !reflect.DeepEqual(got, []model.Profile{original}) {
		t.Fatalf("bulk lookup profiles=%#v err=%v; want exactly original profile", got, err)
	}

	target := testutil.CreateProfile(t, db, other.ID, "case_rename_target", "OtherPlayer")
	updated, err := db.Profiles.UpdateName(ctx, target.ID, "CASEPLAYER")
	assertProfileNameConflict(t, err)
	if updated {
		t.Fatal("rejected unrestricted rename reported a changed row")
	}
	_, err = db.Profiles.UpdateOwnedName(ctx, target.ID, other.ID, "caseplayer", true)
	assertProfileNameConflict(t, err)
	stored, err := db.Profiles.GetByID(ctx, target.ID)
	if err != nil || !reflect.DeepEqual(stored, &target) {
		t.Fatalf("rejected renames changed target=%#v err=%v; want %#v", stored, err, target)
	}
	stored, err = db.Profiles.GetByID(ctx, original.ID)
	if err != nil || !reflect.DeepEqual(stored, &original) {
		t.Fatalf("conflicts changed original=%#v err=%v; want %#v", stored, err, original)
	}

	result, err := db.Profiles.UpdateOwnedName(ctx, original.ID, owner.ID, "CASEPLAYER", true)
	if err != nil || result != (profile.OwnedNameUpdateResult{Found: true, Owned: true, Updated: true}) {
		t.Fatalf("case-only rename result=%#v err=%v", result, err)
	}
	original.Name = "CASEPLAYER"
	stored, err = db.Profiles.GetByName(ctx, "caseplayer")
	if err != nil || !reflect.DeepEqual(stored, &original) {
		t.Fatalf("case-only rename changed identity or assets: profile=%#v err=%v", stored, err)
	}
	if count, err := db.Profiles.CountByUser(ctx, other.ID); err != nil || count != 1 {
		t.Fatalf("conflicting writes changed other user's profile count=%d err=%v", count, err)
	}
}

func assertProfileNameConflict(t *testing.T, err error) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "profiles_name_key" || !profile.IsNameConflict(err) {
		t.Fatalf("name conflict error=%#v; want PostgreSQL 23505 profiles_name_key", err)
	}
}
