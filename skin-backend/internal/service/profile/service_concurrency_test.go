package profile_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestConcurrentProfileNameWritesReturnExactBusinessConflict(t *testing.T) {
	for _, tc := range []struct {
		name        string
		createNames [2]string
		renameNames [2]string
	}{
		{"same spelling", [2]string{"ConcurrentCreate", "ConcurrentCreate"}, [2]string{"ConcurrentRename", "ConcurrentRename"}},
		{"different case", [2]string{"ConcurrentCreate", "concurrentcreate"}, [2]string{"ConcurrentRename", "concurrentrename"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testConcurrentProfileNameWrites(t, tc.createNames, tc.renameNames)
		})
	}
}

func testConcurrentProfileNameWrites(t *testing.T, createNames, renameNames [2]string) {
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

	createErrors := runConcurrentProfileWrites(2, func(index int) error {
		_, err := svc.CreateProfile(context.Background(), testUserActor(user.ID), createNames[index], "default")
		return err
	})
	assertOneProfileWriteConflict(t, createErrors, "profile_name.reserve.conflict")
	var createCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE lower(name)=lower($1)`, createNames[0]).Scan(&createCount); err != nil {
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
	renameErrors := runConcurrentProfileWrites(2, func(index int) error {
		return svc.UpdateProfile(context.Background(), testUserActor(user.ID), profileIDs[index], renameNames[index])
	})
	assertOneProfileWriteConflict(t, renameErrors, "profile_name.reserve.conflict")
	var renamedCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE lower(name)=lower($2)),
			COUNT(*) FILTER (WHERE name IN ('RaceFirst','RaceSecond'))
		FROM profiles
		WHERE id = ANY($1)
	`, profileIDs, renameNames[0]).Scan(&renamedCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if renamedCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent rename state: renamed=%d original=%d; want 1 and 1", renamedCount, originalCount)
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
	if err := <-result; !httpError(err, 404, "profile.resolve.not_found") {
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
	if err := <-result; !httpError(err, 404, "profile.resolve.not_found") {
		t.Fatalf("profile deleted before texture update should return exact not found error, got %#v", err)
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

func runConcurrentProfileWrites(count int, write func(int) error) []error {
	start := make(chan struct{})
	results := make(chan error, count)
	var wg sync.WaitGroup
	for index := range count {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- write(index)
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
