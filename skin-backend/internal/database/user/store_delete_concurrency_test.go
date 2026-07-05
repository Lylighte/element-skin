package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

func TestDeleteDoesNotDeadlockWithConcurrentTextureReupload(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	target := testutil.CreateUser(t, db, "delete-reupload-race@test.com", "Password123", "DeleteReuploadRace", false)
	const hash = "delete_reupload_race"
	if err := db.Textures.AddToLibrary(ctx, target.ID, hash, "skin", "Original", true, "default"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM user_textures
		WHERE user_id=$1 AND hash=$2 AND texture_type='skin'
	`, target.ID, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		UPDATE skin_library
		SET usage_count=0
		WHERE skin_hash=$1 AND texture_type='skin'
	`, hash); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION pause_delete_reupload_insert() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_advisory_xact_lock(74628391);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER pause_delete_reupload_insert
		BEFORE INSERT ON user_textures
		FOR EACH ROW
		WHEN (NEW.hash = 'delete_reupload_race')
		EXECUTE FUNCTION pause_delete_reupload_insert();
	`); err != nil {
		t.Fatal(err)
	}

	blocker, err := db.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(74628391)`); err != nil {
		t.Fatal(err)
	}
	var blockerPID int
	if err := blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID); err != nil {
		t.Fatal(err)
	}

	reuploadResult := make(chan error, 1)
	go func() {
		reuploadResult <- db.Textures.AddToLibrary(
			context.Background(),
			target.ID,
			hash,
			"skin",
			"Reuploaded",
			false,
			"slim",
		)
	}()
	reuploadPID := waitForBlockedBackend(t, db.Pool, blockerPID, reuploadResult)

	type deleteResult struct {
		deleted bool
		err     error
	}
	deleteResults := make(chan deleteResult, 1)
	go func() {
		deleted, err := store.Delete(context.Background(), target.ID)
		deleteResults <- deleteResult{deleted: deleted, err: err}
	}()
	_ = waitForBlockedBackend(t, db.Pool, reuploadPID, deleteResults)

	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-reuploadResult; err != nil {
		t.Fatalf("concurrent reupload failed: %v", err)
	}
	deleted := <-deleteResults
	if deleted.err != nil || !deleted.deleted {
		t.Fatalf("concurrent delete=%v, %v; want true, nil", deleted.deleted, deleted.err)
	}
	if got, err := store.GetByID(ctx, target.ID); err != nil || got != nil {
		t.Fatalf("deleted user still exists: user=%#v err=%v", got, err)
	}
	if exists, err := db.Textures.Exists(ctx, hash, "skin"); err != nil || exists {
		t.Fatalf("deleted user's library texture exists=%v err=%v; want false, nil", exists, err)
	}
	if owned, err := db.Textures.VerifyOwnership(ctx, target.ID, hash, "skin"); err != nil || owned {
		t.Fatalf("deleted user's texture ownership exists=%v err=%v; want false, nil", owned, err)
	}
}

func waitForBlockedBackend[T any](
	t *testing.T,
	db interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	blockerPID int,
	result <-chan T,
) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		select {
		case <-result:
			t.Fatal("database operation completed before the expected lock was released")
		default:
		}
		var blockedPID int
		err := db.QueryRow(t.Context(), `
			SELECT pid
			FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid))
			ORDER BY pid
			LIMIT 1
		`, blockerPID).Scan(&blockedPID)
		if err == nil {
			return blockedPID
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			t.Fatal(err)
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for a backend to block on pid %d", blockerPID)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
