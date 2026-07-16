package permission_test

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestGrantAndRevokeRoleExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "role-grant-revoke@test.com", "pw", "RoleGrantRevoke", false)
	adminID := testutil.CreateUser(t, db, "role-admin@test.com", "pw", "RoleAdmin", true).ID

	hasModerator, err := db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || hasModerator {
		t.Fatalf("new user should not have moderator role: has=%v err=%v", hasModerator, err)
	}
	if err := db.Permissions.GrantRole(ctx, user.ID, core.RoleModerator, permissiondb.SubjectIDForUser(adminID)); err != nil {
		t.Fatal(err)
	}
	hasModerator, err = db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || !hasModerator {
		t.Fatalf("user should have moderator role after grant: has=%v err=%v", hasModerator, err)
	}
	roles, err := db.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range roles {
		if r == core.RoleModerator {
			found = true
		}
	}
	if !found {
		t.Fatalf("RoleIDsForUser should include moderator: %#v", roles)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, "texture.review.assigned") {
		t.Fatal("granted moderator should have texture.review.assigned")
	}

	revoked, err := db.Permissions.RevokeRole(ctx, user.ID, core.RoleModerator)
	if err != nil || !revoked {
		t.Fatalf("RevokeRole should return revoked=true: revoked=%v err=%v", revoked, err)
	}
	hasModerator, err = db.Permissions.UserHasRole(ctx, user.ID, core.RoleModerator)
	if err != nil || hasModerator {
		t.Fatalf("role should be removed after revoke: has=%v err=%v", hasModerator, err)
	}
	revokedAgain, err := db.Permissions.RevokeRole(ctx, user.ID, core.RoleModerator)
	if err != nil || revokedAgain {
		t.Fatalf("revoking missing role should return revoked=false: revoked=%v err=%v", revokedAgain, err)
	}
}

func TestUserIsProtectedExact(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "protected-subject@test.com", "pw", "ProtectedSubject", false)

	protected, err := db.Permissions.UserIsProtected(ctx, user.ID)
	if err != nil || protected {
		t.Fatalf("normal user should not be protected: protected=%v err=%v", protected, err)
	}
	if _, err := db.Pool.Exec(ctx, `
		UPDATE permission_subjects SET protected=TRUE WHERE id=$1
	`, permissiondb.SubjectIDForUser(user.ID)); err != nil {
		t.Fatal(err)
	}
	protected, err = db.Permissions.UserIsProtected(ctx, user.ID)
	if err != nil || !protected {
		t.Fatalf("protected subject flag should be read exactly: protected=%v err=%v", protected, err)
	}
}

func TestTransferProtectedSubjectMovesFlagExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	actor := testutil.CreateUser(t, db, "transfer-actor@test.com", "pw", "TransferActor", true, true)
	target := testutil.CreateUser(t, db, "transfer-target@test.com", "pw", "TransferTarget", false)
	stale := testutil.CreateUser(t, db, "transfer-stale@test.com", "pw", "TransferStale", true, true)

	if _, err := db.Permissions.TransferProtectedSubject(ctx, actor.ID, target.ID, permissiondb.SubjectIDForUser(actor.ID)); err != nil {
		t.Fatal(err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, actor.ID); err != nil || protected {
		t.Fatalf("actor protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, target.ID); err != nil || !protected {
		t.Fatalf("target protected flag after transfer = %v, %v; want true, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, stale.ID); err != nil || protected {
		t.Fatalf("stale protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permission_subjects WHERE protected=TRUE`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("transfer should leave exactly one protected subject: got=%d", count)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, target.ID); err != nil || !protected {
		t.Fatalf("seed should preserve transferred protected subject: protected=%v err=%v", protected, err)
	}
}

func TestTransferProtectedSubjectAffectedUsersExact(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	actor := testutil.CreateUser(t, db, "transfer-affected-actor@test.com", "pw", "TransferAffectedActor", true, true)
	target := testutil.CreateUser(t, db, "transfer-affected-target@test.com", "pw", "TransferAffectedTarget", false)
	stale := testutil.CreateUser(t, db, "transfer-affected-stale@test.com", "pw", "TransferAffectedStale", true, true)

	affected, err := db.Permissions.TransferProtectedSubject(ctx, actor.ID, target.ID, permissiondb.SubjectIDForUser(actor.ID))
	if err != nil {
		t.Fatal(err)
	}
	expectedAffected := []string{actor.ID, stale.ID, target.ID}
	sort.Strings(expectedAffected)
	if !sameStringSlice(affected, expectedAffected) {
		t.Fatalf("affected users mismatch: got=%#v want=%#v", affected, expectedAffected)
	}
}

func TestGrantInitialProtectedManagerWhenExists(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	first := testutil.CreateUser(t, db, "first-super@test.com", "pw", "FirstSuper", false)
	granted, err := db.Permissions.GrantInitialProtectedManagerIfNone(ctx, first.ID)
	if err != nil || !granted {
		t.Fatalf("first user should become protected manager: granted=%v err=%v", granted, err)
	}
	second := testutil.CreateUser(t, db, "second-super@test.com", "pw", "SecondSuper", false)
	grantedAgain, err := db.Permissions.GrantInitialProtectedManagerIfNone(ctx, second.ID)
	if err != nil || grantedAgain {
		t.Fatalf("second call should not create another protected manager: granted=%v err=%v", grantedAgain, err)
	}
	protected, err := db.Permissions.UserIsProtected(ctx, second.ID)
	if err != nil || protected {
		t.Fatalf("second user should not be protected: protected=%v err=%v", protected, err)
	}
}

func TestGrantInitialProtectedManagerIfNoneConcurrentCurrentModel(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	userIDs := []string{
		testutil.CreateUser(t, db, "concurrent-super-1@test.com", "pw", "ConcurrentSuper1", false).ID,
		testutil.CreateUser(t, db, "concurrent-super-2@test.com", "pw", "ConcurrentSuper2", false).ID,
		testutil.CreateUser(t, db, "concurrent-super-3@test.com", "pw", "ConcurrentSuper3", false).ID,
		testutil.CreateUser(t, db, "concurrent-super-4@test.com", "pw", "ConcurrentSuper4", false).ID,
		testutil.CreateUser(t, db, "concurrent-super-5@test.com", "pw", "ConcurrentSuper5", false).ID,
		testutil.CreateUser(t, db, "concurrent-super-6@test.com", "pw", "ConcurrentSuper6", false).ID,
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE permission_subjects SET protected=FALSE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		DELETE FROM subject_permission_overrides WHERE permission_id=$1
	`, int64(core.MustDefinitionByCode("permission_protected.manage.any").ID)); err != nil {
		t.Fatal(err)
	}

	type result struct {
		userID  string
		granted bool
		err     error
	}
	start := make(chan struct{})
	results := make(chan result, len(userIDs))
	var wg sync.WaitGroup
	for _, userID := range userIDs {
		wg.Add(1)
		go func(userID string) {
			defer wg.Done()
			<-start
			callCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			granted, err := db.Permissions.GrantInitialProtectedManagerIfNone(callCtx, userID)
			results <- result{userID: userID, granted: granted, err: err}
		}(userID)
	}
	close(start)
	wg.Wait()
	close(results)

	grantedCount := 0
	grantedUserID := ""
	for item := range results {
		if item.err != nil {
			t.Fatalf("concurrent GrantInitialProtectedManagerIfNone failed for %s: %v", item.userID, item.err)
		}
		if item.granted {
			grantedCount++
			grantedUserID = item.userID
		}
	}
	if grantedCount != 1 {
		t.Fatalf("concurrent initial protected manager grants=%d, want exactly 1", grantedCount)
	}
	var storedCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permission_subjects WHERE protected=TRUE`).Scan(&storedCount); err != nil {
		t.Fatal(err)
	}
	if storedCount != 1 {
		t.Fatalf("protected subject count=%d, want exactly 1", storedCount)
	}
	for _, userID := range userIDs {
		protected, err := db.Permissions.UserIsProtected(ctx, userID)
		if err != nil {
			t.Fatal(err)
		}
		if (userID == grantedUserID) != protected {
			t.Fatalf("protected subject assignment mismatch for %s: protected=%v winner=%s", userID, protected, grantedUserID)
		}
	}
}

func TestEnsureUserSubjectIdempotent(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ensure-subject@test.com", "pw", "EnsureSubject", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatalf("double EnsureUserSubject should be idempotent: %v", err)
	}
	roles, err := db.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	userCount := 0
	for _, r := range roles {
		if r == core.RoleUser {
			userCount++
		}
	}
	if userCount != 1 {
		t.Fatalf("EnsureUserSubject should assign user role exactly once: roles=%#v", roles)
	}
}

func TestRoleIDsForUserRejectsNonexistentUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	_, err := db.Permissions.RoleIDsForUser(ctx, "nonexistent-user-id")
	assertPostgresError(t, err, "23503")
}

func TestGrantRoleErrorFromEnsureUserSubject(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Permissions.GrantRole(ctx, "nonexistent", core.RoleModerator, "")
	assertCancelled(t, err)
}

func TestRevokeRoleErrorPath(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.RevokeRole(ctx, "nonexistent", core.RoleModerator)
	assertCancelled(t, err)
}

func TestRoleIDsForUserCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.RoleIDsForUser(ctx, "nonexistent")
	assertCancelled(t, err)
}

func TestGrantInitialProtectedManagerIfNoneErrorPath(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := db.Permissions.GrantInitialProtectedManagerIfNone(ctx, "nonexistent")
	assertCancelled(t, err)
}

func TestEnsureUserSubjectConstraintError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "constraint-err@test.com", "pw", "ConstraintErr", false)
	if _, err := db.Pool.Exec(ctx, `DELETE FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForUser(user.ID)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE permission_subjects ADD CONSTRAINT always_reject CHECK (FALSE) NOT VALID`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.EnsureUserSubject(ctx, user.ID)
	assertPostgresError(t, err, "23514")
}

func TestEnsureUserSubjectCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Permissions.EnsureUserSubject(ctx, "nonexistent")
	assertCancelled(t, err)
}

func TestRoleIDsForUserRowsScanError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "roleids-scan-err@test.com", "pw", "RoleIDsScanErr", false)
	fc := testutil.NewFaultyConn(db.Pool).WithScanError(testutil.ErrFaultInjected)
	db.Permissions.SetTestConn(fc)
	_, err := db.Permissions.RoleIDsForUser(ctx, user.ID)
	if err != testutil.ErrFaultInjected {
		t.Fatalf("should return injected Scan error: %v", err)
	}
}

func sameStringSlice(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
