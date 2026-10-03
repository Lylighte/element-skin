package database_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestInitMigratesProfileNamesWithoutChangingProfiles(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	useCaseSensitiveProfileNames(t, db)
	user := testutil.CreateUser(t, db, "profile-name-migration@test.com", "Password123", "NameMigration", false)
	skin, cape := "migration_skin", "migration_cape"
	want := model.Profile{ID: "migration_profile", UserID: user.ID, Name: "MixedCase", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape}
	if err := db.Profiles.Create(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := db.Init(t.Context()); err != nil {
			t.Fatal(err)
		}
		got, err := db.Profiles.GetByName(t.Context(), "mixedcase")
		if err != nil || !reflect.DeepEqual(got, &want) {
			t.Fatalf("migration changed profile=%#v err=%v; want %#v", got, err, want)
		}
	}
	duplicate := want
	duplicate.ID = "migration_rejected"
	duplicate.Name = "MIXEDCASE"
	if err := db.Profiles.Create(t.Context(), duplicate); !profilestore.IsNameConflict(err) {
		t.Fatalf("migrated constraint accepted case conflict: %v", err)
	}
	if got, err := db.Profiles.GetByID(t.Context(), duplicate.ID); err != nil || got != nil {
		t.Fatalf("rejected post-migration create persisted profile=%#v err=%v", got, err)
	}
}

func TestInitRejectsLegacyCaseConflictsAndRollsBackUntilRepaired(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	useCaseSensitiveProfileNames(t, db)
	owner := testutil.CreateUser(t, db, "legacy-case-owner@test.com", "Password123", "LegacyOwner", false)
	other := testutil.CreateUser(t, db, "legacy-case-other@test.com", "Password123", "LegacyOther", false)
	skin, cape := "legacy_skin", "legacy_cape"
	want := []model.Profile{
		{ID: "legacy_case_first", UserID: owner.ID, Name: "CasePlayer", TextureModel: "slim", SkinHash: &skin},
		{ID: "legacy_case_second", UserID: other.ID, Name: "caseplayer", TextureModel: "default", CapeHash: &cape},
	}
	for _, item := range want {
		if err := db.Profiles.Create(t.Context(), item); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		err := db.Init(t.Context())
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" || pgErr.ConstraintName != "profiles_name_key" ||
			!strings.Contains(err.Error(), "(caseplayer)") || !strings.Contains(err.Error(), "resolve conflicting names before restarting") {
			t.Fatalf("legacy conflict error=%#v; want actionable wrapped PostgreSQL name conflict", err)
		}
		for _, item := range want {
			got, err := db.Profiles.GetByID(t.Context(), item.ID)
			if err != nil || !reflect.DeepEqual(got, &item) {
				t.Fatalf("failed migration changed profile=%#v err=%v; want %#v", got, err, item)
			}
		}
		var oldConstraint bool
		if err := db.Pool.QueryRow(t.Context(), `
			SELECT EXISTS(SELECT 1 FROM pg_constraint
			WHERE conrelid='profiles'::regclass AND conname='profiles_name_key' AND contype='u')
		`).Scan(&oldConstraint); err != nil || !oldConstraint {
			t.Fatalf("failed migration removed legacy constraint: exists=%t err=%v", oldConstraint, err)
		}
		var count int
		if err := db.Pool.QueryRow(t.Context(), `SELECT COUNT(*) FROM profiles`).Scan(&count); err != nil || count != 2 {
			t.Fatalf("failed migration changed row count=%d err=%v; want 2", count, err)
		}
	}

	// Simulate the operator's chosen rename; identity and ownership stay intact.
	want[1].Name = "OtherPlayer"
	if updated, err := db.Profiles.UpdateName(t.Context(), want[1].ID, want[1].Name); err != nil || !updated {
		t.Fatalf("repair rename updated=%t err=%v", updated, err)
	}
	if err := db.Init(t.Context()); err != nil {
		t.Fatalf("migration after repair: %v", err)
	}
	for _, item := range want {
		got, err := db.Profiles.GetByName(t.Context(), strings.ToLower(item.Name))
		if err != nil || !reflect.DeepEqual(got, &item) {
			t.Fatalf("repaired migration changed profile=%#v err=%v; want %#v", got, err, item)
		}
	}
	if updated, err := db.Profiles.UpdateName(t.Context(), want[1].ID, "CASEPLAYER"); updated || !profilestore.IsNameConflict(err) {
		t.Fatalf("repaired migration failed to enforce uniqueness: updated=%t err=%v", updated, err)
	}
}

func useCaseSensitiveProfileNames(t *testing.T, db *database.DB) {
	t.Helper()
	if _, err := db.Pool.Exec(t.Context(), `
		DROP INDEX profiles_name_key;
		ALTER TABLE profiles ADD CONSTRAINT profiles_name_key UNIQUE(name);
	`); err != nil {
		t.Fatal(err)
	}
}
