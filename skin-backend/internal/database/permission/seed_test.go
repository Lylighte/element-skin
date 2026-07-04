package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestSeedDefaultsPersistsCatalogExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	var permissionCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM permissions`).Scan(&permissionCount); err != nil {
		t.Fatal(err)
	}
	if permissionCount != len(core.Definitions) {
		t.Fatalf("permission count mismatch: got=%d want=%d", permissionCount, len(core.Definitions))
	}
	def := core.MustDefinitionByCode("permission_protected.manage.any")
	var id int64
	var resourceID int
	var actionID int
	var scopeID int
	if err := db.Pool.QueryRow(ctx, `
		SELECT id,resource_id,action_id,scope_id
		FROM permissions
		WHERE code='permission_protected.manage.any'
	`).Scan(&id, &resourceID, &actionID, &scopeID); err != nil {
		t.Fatal(err)
	}
	if id != int64(def.ID) || resourceID != int(def.Resource.ID) || actionID != int(def.Action.ID) || scopeID != int(def.Scope.ID) {
		t.Fatalf("seeded permission mismatch: id=%#x/%#x resource=%d/%d action=%d/%d scope=%d/%d",
			id, int64(def.ID), resourceID, def.Resource.ID, actionID, def.Action.ID, scopeID, def.Scope.ID)
	}
	var roleCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE system_role=TRUE`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != len(core.Roles) {
		t.Fatalf("system role count mismatch: got=%d want=%d", roleCount, len(core.Roles))
	}
}

func TestSeedMigratesExistingAdminFlagsToRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "permission-admin@test.com", "pw", "PermissionAdmin", true)
	super := testutil.CreateUser(t, db, "permission-super@test.com", "pw", "PermissionSuper", true, true)
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	adminBits, err := db.Permissions.EffectivePermissionsForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(adminBits, "notice.create.any") {
		t.Fatal("migrated admin should create notices")
	}
	if has(adminBits, "permission_protected.manage.any") {
		t.Fatal("migrated admin must not manage protected permission subjects")
	}
	superBits, err := db.Permissions.EffectivePermissionsForUser(ctx, super.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(superBits, "permission_protected.manage.any") {
		t.Fatal("migrated super admin should manage protected permission subjects")
	}
	if has(superBits, "cache.invalidate.system") {
		t.Fatal("super admin should not receive system-scope permissions")
	}
}

func TestSeedUserSubjectsMigratesIsAdminColumnExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	adminUser := testutil.CreateUser(t, db, "migrate-admin@test.com", "pw", "MigrateAdmin", false)
	normalUser := testutil.CreateUser(t, db, "migrate-normal@test.com", "pw", "MigrateNormal", false)

	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users ADD COLUMN is_admin BOOLEAN DEFAULT FALSE`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET is_admin=TRUE WHERE id=$1`, adminUser.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	hasAdmin, err := db.Permissions.UserHasRole(ctx, adminUser.ID, core.RoleAdmin)
	if err != nil || !hasAdmin {
		t.Fatalf("is_admin=TRUE user should get admin role: has=%v err=%v", hasAdmin, err)
	}
	normalHasAdmin, err := db.Permissions.UserHasRole(ctx, normalUser.ID, core.RoleAdmin)
	if err != nil || normalHasAdmin {
		t.Fatalf("is_admin=FALSE user should not get admin role: has=%v err=%v", normalHasAdmin, err)
	}
}

func TestSeedDefaultsFirstRegisteredUserBecomesSuperAdmin(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	user := testutil.CreateUser(t, db, "first-user@test.com", "pw", "FirstUser", false)
	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("SeedDefaults should ensure exactly one super_admin after removal: got=%d", count)
	}
	hasSuper, err := db.Permissions.UserHasRole(ctx, user.ID, core.RoleSuperAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !hasSuper {
		t.Fatal("the only user should become super_admin when none exists")
	}
}

func TestSeedDefaultsDeduplicatesMultipleSuperAdminsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()

	first := testutil.CreateUser(t, db, "dedup-first@test.com", "pw", "DedupFirst", false)
	second := testutil.CreateUser(t, db, "dedup-second@test.com", "pw", "DedupSecond", false)
	third := testutil.CreateUser(t, db, "dedup-third@test.com", "pw", "DedupThird", false)
	now := time.Now().UnixMilli()

	if _, err := db.Pool.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	for _, u := range []struct{ id, sub string }{
		{first.ID, permissiondb.SubjectIDForUser(first.ID)},
		{second.ID, permissiondb.SubjectIDForUser(second.ID)},
		{third.ID, permissiondb.SubjectIDForUser(third.ID)},
	} {
		if _, err := db.Pool.Exec(ctx, `
			INSERT INTO subject_roles (subject_id,role_id,created_at)
			VALUES ($1,$2,$3)
		`, u.sub, core.RoleSuperAdmin, now); err != nil {
			t.Fatal(err)
		}
	}

	if err := db.Permissions.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM subject_roles WHERE role_id=$1`, core.RoleSuperAdmin).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dedup should keep exactly one super_admin: got=%d", count)
	}
	var keptSubject string
	if err := db.Pool.QueryRow(ctx, `
		SELECT sr.subject_id FROM subject_roles sr
		JOIN permission_subjects ps ON ps.id=sr.subject_id
		JOIN users u ON u.id=ps.user_id
		WHERE sr.role_id=$1
		ORDER BY u.created_at ASC, u.id ASC
		LIMIT 1
	`, core.RoleSuperAdmin).Scan(&keptSubject); err != nil {
		t.Fatal(err)
	}
	if keptSubject != permissiondb.SubjectIDForUser(first.ID) {
		t.Fatalf("dedup should keep earliest user: got=%s want=%s", keptSubject, permissiondb.SubjectIDForUser(first.ID))
	}
}

func TestSeedDefaultsFailsWhenCatalogTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_resources CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenSessionPoliciesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenSubjectRolesTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE subject_roles CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionActionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_actions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenRolePermissionsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE role_permissions CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}

func TestSeedDefaultsFailsWhenPermissionSubjectsTableMissing(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `DROP TABLE permission_subjects CASCADE`); err != nil {
		t.Fatal(err)
	}
	err := db.Permissions.SeedDefaults(ctx)
	assertPostgresError(t, err, "42P01")
}
