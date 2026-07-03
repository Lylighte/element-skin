package profile_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestProfilesCreateListAndClearTextureExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "site-profiles-service@test.com", "Password123", "SiteProfilesService", false)
	created, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ProfileSvc", "slim")
	if err != nil {
		t.Fatal(err)
	}
	if created["name"] != "ProfileSvc" || created["model"] != "slim" {
		t.Fatalf("CreateProfile response mismatch: %#v", created)
	}
	list, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["name"] != "ProfileSvc" || list["next_cursor"] != "" {
		t.Fatalf("ListMyProfiles mismatch: %#v", list)
	}

	skin := "profile_service_skin"
	cape := "profile_service_cape"
	if err := db.Profiles.UpdateSkin(ctx, created["id"].(string), &skin); err != nil {
		t.Fatal(err)
	}
	if err := db.Profiles.UpdateCape(ctx, created["id"].(string), &cape); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearProfileTexture(ctx, testUserActor(user.ID), created["id"].(string), "skin"); err != nil {
		t.Fatal(err)
	}
	cleared, err := db.Profiles.GetByID(ctx, created["id"].(string))
	if err != nil || cleared == nil || cleared.SkinHash != nil || cleared.CapeHash == nil || *cleared.CapeHash != cape {
		t.Fatalf("ClearProfileTexture should clear only skin: profile=%#v err=%v", cleared, err)
	}

	deletable := testutil.CreateProfile(t, db, user.ID, "site_profile_owned_delete", "OwnedDelete")
	if err := svc.DeleteProfile(ctx, testUserActor(user.ID), deletable.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, deletable.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfile should remove owned profile exactly: profile=%#v err=%v", got, err)
	}
}

func TestProfilesRejectInvalidProfileInputsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "site-profile-invalid@test.com", "Password123", "ProfileInvalid", false)
	other := testutil.CreateUser(t, db, "site-profile-invalid-other@test.com", "Password123", "ProfileInvalidOther", false)
	existing := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_existing", "ExistingSvc")
	target := testutil.CreateProfile(t, db, user.ID, "site_profile_invalid_target", "TargetSvc")
	foreign := testutil.CreateProfile(t, db, other.ID, "site_profile_invalid_foreign", "ForeignSvc")

	for _, tc := range []struct {
		name string
		call func() error
		code int
		want string
	}{
		{"empty create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "", "default")
			return err
		}, 400, "name required"},
		{"invalid create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), "bad-name!", "default")
			return err
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate create name", func() error {
			_, err := svc.CreateProfile(ctx, testUserActor(user.ID), existing.Name, "default")
			return err
		}, 400, "角色名已被占用，请换一个名称"},
		{"empty update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "")
		}, 400, "name required"},
		{"invalid update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, "bad-name!")
		}, 400, "角色名只能包含字母、数字、下划线，长度1-16字符"},
		{"duplicate update name", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), target.ID, existing.Name)
		}, 400, "角色名已被占用"},
		{"foreign update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), foreign.ID, "StolenProfileSvc")
		}, 403, "not allowed"},
		{"missing update", func() error {
			return svc.UpdateProfile(ctx, testUserActor(user.ID), "missing-profile", "MissingProfileSvc")
		}, 404, "profile not found"},
		{"foreign delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), foreign.ID)
		}, 403, "not allowed"},
		{"missing delete", func() error {
			return svc.DeleteProfile(ctx, testUserActor(user.ID), "missing-profile")
		}, 404, "profile not found"},
		{"invalid clear texture type", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), target.ID, "elytra")
		}, 400, "Invalid texture_type"},
		{"foreign clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), foreign.ID, "skin")
		}, 403, "not allowed"},
		{"missing clear texture", func() error {
			return svc.ClearProfileTexture(ctx, testUserActor(user.ID), "missing-profile", "skin")
		}, 404, "profile not found"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !httpError(err, tc.code, tc.want) {
				t.Fatalf("%s should reject exactly, got %#v", tc.name, err)
			}
		})
	}

	unchanged, err := db.Profiles.GetByID(ctx, target.ID)
	if err != nil || unchanged == nil || unchanged.Name != target.Name {
		t.Fatalf("invalid profile mutations should not change target: profile=%#v err=%v", unchanged, err)
	}
	foreignAfter, err := db.Profiles.GetByID(ctx, foreign.ID)
	if err != nil || foreignAfter == nil || foreignAfter.Name != foreign.Name {
		t.Fatalf("foreign profile should remain unchanged: profile=%#v err=%v", foreignAfter, err)
	}
}

func TestConcurrentProfileNameWritesReturnExactBusinessConflict(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-name-race@test.com", "Password123", "ProfileNameRace", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_profile_name_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_profile_name_insert
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_profile_name_write();
	`); err != nil {
		t.Fatal(err)
	}

	createErrors := runConcurrentProfileWrites(2, func() error {
		_, err := svc.CreateProfile(context.Background(), testUserActor(user.ID), "ConcurrentCreate", "default")
		return err
	})
	assertOneProfileWriteConflict(t, createErrors, "角色名已被占用，请换一个名称")
	var createCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE name='ConcurrentCreate'`).Scan(&createCount); err != nil {
		t.Fatal(err)
	}
	if createCount != 1 {
		t.Fatalf("concurrent create stored %d target names; want exactly 1", createCount)
	}

	if _, err := db.Pool.Exec(ctx, `
		DROP TRIGGER delay_profile_name_insert ON profiles;
		CREATE TRIGGER delay_profile_name_update
		BEFORE UPDATE OF name ON profiles
		FOR EACH ROW EXECUTE FUNCTION delay_profile_name_write();
	`); err != nil {
		t.Fatal(err)
	}
	first := testutil.CreateProfile(t, db, user.ID, "profile_name_race_first", "RaceFirst")
	second := testutil.CreateProfile(t, db, user.ID, "profile_name_race_second", "RaceSecond")
	profileIDs := []string{first.ID, second.ID}
	var index int
	var mu sync.Mutex
	renameErrors := runConcurrentProfileWrites(2, func() error {
		mu.Lock()
		profileID := profileIDs[index]
		index++
		mu.Unlock()
		return svc.UpdateProfile(context.Background(), testUserActor(user.ID), profileID, "ConcurrentRename")
	})
	assertOneProfileWriteConflict(t, renameErrors, "角色名已被占用")
	var renamedCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE name='ConcurrentRename'),
			COUNT(*) FILTER (WHERE name IN ('RaceFirst','RaceSecond'))
		FROM profiles
		WHERE id = ANY($1)
	`, profileIDs).Scan(&renamedCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if renamedCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent rename state: renamed=%d original=%d; want 1 and 1", renamedCount, originalCount)
	}
}

func TestCreateProfileMapsDatabaseIDConflictExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-id-conflict@test.com", "Password123", "ProfileIDConflict", false)
	existing := testutil.CreateProfile(t, db, user.ID, "forced_profile_id_conflict", "ExistingID")
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION force_profile_id_conflict() RETURNS trigger AS $$
		BEGIN
			IF NEW.name = 'ForcedUUID' THEN
				NEW.id := 'forced_profile_id_conflict';
			END IF;
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER force_profile_id_conflict
		BEFORE INSERT ON profiles
		FOR EACH ROW EXECUTE FUNCTION force_profile_id_conflict();
	`); err != nil {
		t.Fatal(err)
	}

	result, err := svc.CreateProfile(ctx, testUserActor(user.ID), "ForcedUUID", "slim")
	if result != nil || !httpError(err, 400, "角色 UUID 冲突，无法新建角色") {
		t.Fatalf("forced profile ID conflict result=%#v err=%#v; want nil and exact 400", result, err)
	}
	if stored, err := db.Profiles.GetByName(ctx, "ForcedUUID"); err != nil || stored != nil {
		t.Fatalf("forced UUID conflict persisted target name: profile=%#v err=%v", stored, err)
	}
	unchanged, err := db.Profiles.GetByID(ctx, existing.ID)
	if err != nil || unchanged == nil || unchanged.Name != "ExistingID" || unchanged.TextureModel != "default" {
		t.Fatalf("forced UUID conflict changed existing profile: profile=%#v err=%v", unchanged, err)
	}
}

func TestCreateProfileOfflineModeUsesDeterministicIDAndRejectsIDConflict(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-offline-mode@test.com", "Password123", "ProfileOfflineMode", false)
	other := testutil.CreateUser(t, db, "profile-offline-conflict@test.com", "Password123", "ProfileOfflineConflict", false)
	if err := db.Settings.Set(ctx, "profile_uuid_mode", "offline"); err != nil {
		t.Fatal(err)
	}
	if err := svc.Settings.InvalidateCache(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := svc.CreateProfile(ctx, testUserActor(user.ID), "OfflineProfile", "wide")
	if err != nil {
		t.Fatal(err)
	}
	if created["id"] != util.OfflineUUIDNoDash("OfflineProfile") || created["model"] != "default" {
		t.Fatalf("offline profile creation mismatch: %#v", created)
	}
	conflictingID := util.OfflineUUIDNoDash("OfflineClash")
	testutil.CreateProfile(t, db, other.ID, conflictingID, "DifferentName")
	result, err := svc.CreateProfile(ctx, testUserActor(user.ID), "OfflineClash", "slim")
	if result != nil || !httpError(err, 400, "角色 UUID 冲突，无法新建角色") {
		t.Fatalf("offline profile ID conflict result=%#v err=%#v", result, err)
	}
	if stored, err := db.Profiles.GetByName(ctx, "OfflineClash"); err != nil || stored != nil {
		t.Fatalf("offline ID conflict must not create requested name: profile=%#v err=%v", stored, err)
	}
}

func TestUpdateProfileReturnsNotFoundWhenProfileIsDeletedAfterRead(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-update-delete-race@test.com", "Password123", "ProfileUpdateDeleteRace", false)
	target := testutil.CreateProfile(t, db, user.ID, "profile_update_delete_race", "DeleteRace")

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `SELECT 1, pg_backend_pid() FROM profiles WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- svc.UpdateProfile(context.Background(), testUserActor(user.ID), target.ID, target.Name)
	}()
	waitForBlockedDatabaseOperation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !httpError(err, 404, "profile not found") {
		t.Fatalf("profile deleted after read should return exact not found error, got %#v", err)
	}
}

func TestClearProfileTextureReturnsNotFoundWhenProfileIsDeletedAfterRead(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-clear-delete-race@test.com", "Password123", "ProfileClearRace", false)
	target := testutil.CreateProfile(t, db, user.ID, "profile_clear_delete_race", "ClearRace")
	skin := "clear_race_skin"
	if err := db.Profiles.UpdateSkin(ctx, target.ID, &skin); err != nil {
		t.Fatal(err)
	}

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	var one, lockHolderPID int
	if err := tx.QueryRow(ctx, `SELECT 1, pg_backend_pid() FROM profiles WHERE id=$1 FOR UPDATE`, target.ID).Scan(&one, &lockHolderPID); err != nil {
		t.Fatal(err)
	}

	result := make(chan error, 1)
	go func() {
		result <- svc.ClearProfileTexture(context.Background(), testUserActor(user.ID), target.ID, "skin")
	}()
	waitForBlockedDatabaseOperation(t, db.Pool, lockHolderPID, result)
	if _, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !httpError(err, 404, "profile not found") {
		t.Fatalf("profile deleted before texture update should return exact not found error, got %#v", err)
	}
}

func TestProfilesCursorsAndAdminDeleteByID(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "site-profile-cursor@test.com", "Password123", "ProfileCursor", false)
	profile := testutil.CreateProfile(t, db, user.ID, "site_profile_admin_delete", "AdminDeleteProfileSvc")

	if _, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), "not-base64", 10); !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("invalid profile cursor should reject exactly, got %#v", err)
	}

	adminActor := testActorWithCodes("admin-delete-profile", "profile.delete.any")
	if err := svc.DeleteProfileByID(ctx, adminActor, profile.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || got != nil {
		t.Fatalf("DeleteProfileByID should remove profile regardless of owner: profile=%#v err=%v", got, err)
	}
	if err := svc.DeleteProfileByID(ctx, adminActor, profile.ID); !httpError(err, 404, "profile not found") {
		t.Fatalf("DeleteProfileByID missing profile should reject exactly, got %#v", err)
	}
}

func TestProfileServiceAdminListsRejectIncompleteCursorsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testActorWithCodes("admin-profile-incomplete-cursor", "profile.read.any")
	cursor := util.EncodeCursor(map[string]any{"unexpected": "value"})

	if result, err := svc.ListAllProfiles(ctx, actor, cursor, 10, ""); result != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListAllProfiles incomplete cursor result=%#v err=%#v", result, err)
	}
	if result, err := svc.ListProfilesByUser(ctx, actor, "any-user", cursor, 10); result != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListProfilesByUser incomplete cursor result=%#v err=%#v", result, err)
	}
}

func TestPrivateProfileListRejectsIncompleteCursor(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "private-cursor@test.com", "Password123", "PrivateCursor", false)

	profileResult, err := svc.ListMyProfiles(ctx, testUserActor(user.ID), util.EncodeCursor(map[string]any{
		"unexpected": "value",
	}), 10)
	if profileResult != nil || !httpError(err, 400, "Invalid cursor") {
		t.Fatalf("ListMyProfiles result=%#v err=%#v; want nil and exact invalid cursor", profileResult, err)
	}
}

func TestSetProfileTextureSkipsExactNoOpWrites(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-noop@test.com", "Password123", "ProfileNoop", false)
	profile := testutil.CreateProfile(t, db, user.ID, "profile_noop_values", "ProfileNoopValues")
	empty := testutil.CreateProfile(t, db, user.ID, "profile_noop_empty", "ProfileNoopEmpty")
	adminActor := testActorWithCodes("profile-texture-admin", "profile.update.any")
	skin := "same_skin_hash"
	cape := "same_cape_hash"
	if err := db.Profiles.UpdateSkin(ctx, profile.ID, &skin); err != nil {
		t.Fatalf("seed skin: %v", err)
	}
	if err := db.Profiles.UpdateCape(ctx, profile.ID, &cape); err != nil {
		t.Fatalf("seed cape: %v", err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION reject_profile_noop_updates() RETURNS trigger AS $$
		BEGIN
			RAISE EXCEPTION 'profile update should not run' USING ERRCODE='23514';
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER reject_profile_noop_updates
		BEFORE UPDATE ON profiles
		FOR EACH ROW
		WHEN (OLD.id IN ('profile_noop_values', 'profile_noop_empty'))
		EXECUTE FUNCTION reject_profile_noop_updates();
	`); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name        string
		profileID   string
		textureType string
		hash        *string
	}{
		{name: "same skin", profileID: profile.ID, textureType: "skin", hash: &skin},
		{name: "same cape", profileID: profile.ID, textureType: "cape", hash: &cape},
		{name: "already clear skin", profileID: empty.ID, textureType: "skin", hash: nil},
		{name: "already clear cape", profileID: empty.ID, textureType: "cape", hash: nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := svc.SetProfileTexture(ctx, adminActor, tc.profileID, tc.textureType, tc.hash); err != nil {
				t.Fatalf("exact no-op should skip database update: %v", err)
			}
		})
	}

	different := "different_skin_hash"
	err := svc.SetProfileTexture(ctx, adminActor, profile.ID, "skin", &different)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("changed value should hit rejection trigger exactly, got %#v", err)
	}
}

func TestSetProfileTextureCapeAndMissingProfileExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	user := testutil.CreateUser(t, db, "profile-set-cape@test.com", "Password123", "ProfileSetCape", false)
	profile := testutil.CreateProfile(t, db, user.ID, "profile_set_cape", "ProfileSetCape")
	actor := testActorWithCodes("profile-set-cape-admin", "profile.update.any")
	cape := "new_cape_hash"

	if err := svc.SetProfileTexture(ctx, actor, profile.ID, "cape", &cape); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Profiles.GetByID(ctx, profile.ID)
	if err != nil || updated == nil || updated.CapeHash == nil || *updated.CapeHash != cape || updated.SkinHash != nil {
		t.Fatalf("SetProfileTexture cape mismatch: profile=%#v err=%v", updated, err)
	}
	if err := svc.SetProfileTexture(ctx, actor, "missing-set-cape", "skin", nil); !httpError(err, 404, "profile not found") {
		t.Fatalf("SetProfileTexture missing profile mismatch: %#v", err)
	}
}

func TestProfileServiceClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testUserActor("closed-profile-user")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "create profile", call: func() error {
			result, err := svc.CreateProfile(ctx, actor, "ClosedProfile", "default")
			if result != nil {
				t.Fatalf("CreateProfile closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "list profiles", call: func() error {
			result, err := svc.ListMyProfiles(ctx, actor, "", 10)
			if result != nil {
				t.Fatalf("ListMyProfiles closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update profile", call: func() error {
			return svc.UpdateProfile(ctx, actor, "closed-profile", "ClosedProfile")
		}},
		{name: "delete profile", call: func() error {
			return svc.DeleteProfile(ctx, actor, "closed-profile")
		}},
		{name: "clear profile texture", call: func() error {
			return svc.ClearProfileTexture(ctx, actor, "closed-profile", "skin")
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}

func TestProfileServiceAdminClosedDatabaseReturnsExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := newProfileService(db)
	actor := testActorWithCodes("closed-profile-admin", "profile.read.any", "profile.update.any", "profile.delete.any")
	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "list all profiles", call: func() error {
			result, err := svc.ListAllProfiles(ctx, actor, "", 10, "")
			if result != nil {
				t.Fatalf("ListAllProfiles closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "list profiles by user", call: func() error {
			result, err := svc.ListProfilesByUser(ctx, actor, "closed-profile-user", "", 10)
			if result != nil {
				t.Fatalf("ListProfilesByUser closed database returned result=%#v", result)
			}
			return err
		}},
		{name: "update any profile", call: func() error {
			return svc.UpdateAnyProfile(ctx, actor, "closed-profile", "ClosedProfile")
		}},
		{name: "delete profile by id", call: func() error {
			return svc.DeleteProfileByID(ctx, actor, "closed-profile")
		}},
		{name: "set profile texture", call: func() error {
			hash := "closed-profile-skin"
			return svc.SetProfileTexture(ctx, actor, "closed-profile", "skin", &hash)
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); !closedPoolError(err) {
				t.Fatalf("%s closed database error=%v; want closed pool", tc.name, err)
			}
		})
	}
}

func waitForBlockedDatabaseOperation(t *testing.T, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, lockHolderPID int, result <-chan error) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case err := <-result:
			t.Fatalf("database operation completed before row-lock release: %#v", err)
		default:
		}
		var waiting bool
		if err := db.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT 1 FROM pg_stat_activity
				WHERE $1 = ANY(pg_blocking_pids(pid))
			)
		`, lockHolderPID).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("database operation did not reach the expected row-lock wait")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func runConcurrentProfileWrites(count int, write func() error) []error {
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- write()
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	out := make([]error, 0, count)
	for err := range results {
		out = append(out, err)
	}
	return out
}

func assertOneProfileWriteConflict(t *testing.T, results []error, detail string) {
	t.Helper()
	successes := 0
	conflicts := 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case httpError(err, 400, detail):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent profile result: %#v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent profile writes: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
}

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}

func newProfileService(db *database.DB) profilesvc.Service {
	redis := testutil.NewMemoryRedis()
	return profilesvc.Service{DB: db, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}
