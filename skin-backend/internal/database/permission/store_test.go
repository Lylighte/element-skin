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
	var bitIndex int
	var resourceID int
	var actionID int
	var scopeID int
	if err := db.Pool.QueryRow(ctx, `
		SELECT id,bit_index,resource_id,action_id,scope_id
		FROM permissions
		WHERE code='permission_protected.manage.any'
	`).Scan(&id, &bitIndex, &resourceID, &actionID, &scopeID); err != nil {
		t.Fatal(err)
	}
	if id != int64(def.ID) || bitIndex != def.BitIndex || resourceID != int(def.Resource.ID) || actionID != int(def.Action.ID) || scopeID != int(def.Scope.ID) {
		t.Fatalf("seeded permission mismatch: id=%#x/%#x bit=%d/%d resource=%d/%d action=%d/%d scope=%d/%d",
			id, int64(def.ID), bitIndex, def.BitIndex, resourceID, def.Resource.ID, actionID, def.Action.ID, scopeID, def.Scope.ID)
	}
	var roleCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM roles WHERE system_role=TRUE`).Scan(&roleCount); err != nil {
		t.Fatal(err)
	}
	if roleCount != len(core.Roles) {
		t.Fatalf("system role count mismatch: got=%d want=%d", roleCount, len(core.Roles))
	}
}

func TestEffectivePermissionsIncludeDefaultUserRoleAndExactOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-user@test.com", "pw", "PermissionUser", false)
	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(before, "texture.update_visibility.owned") {
		t.Fatal("default user should be allowed to update visibility of owned textures")
	}
	if has(before, "permission_protected.manage.any") {
		t.Fatal("default user must not manage protected permission subjects")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, core.MustDefinitionByCode("texture.update_visibility.owned"), "deny", ""); err != nil {
		t.Fatal(err)
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(after, "texture.update_visibility.owned") {
		t.Fatal("deny override should remove texture.update_visibility.owned exactly")
	}
	if !has(after, "texture.update_metadata.owned") {
		t.Fatal("deny override should not remove neighboring texture.update_metadata.owned")
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

func TestSessionPolicyAndBanNarrowPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-ygg@test.com", "pw", "PermissionYgg", false)
	unbanned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindYggdrasil,
		Entrypoint:  core.EntrypointYggdrasil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(unbanned, "yggdrasil_server.join.bound_profile") || !has(unbanned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("yggdrasil session should include join and hasjoined before ban policy")
	}
	if has(unbanned, "account.read.self") {
		t.Fatal("yggdrasil session should not include dashboard account permissions")
	}
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, bannedUntil, user.ID); err != nil {
		t.Fatal(err)
	}
	banned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if has(banned, "yggdrasil_server.join.bound_profile") {
		t.Fatal("ban policy should revoke only yggdrasil_server.join.bound_profile")
	}
	if !has(banned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("ban policy should keep yggdrasil_server.hasjoined.bound_profile")
	}
}

func TestDelegationPolicyIntersectsSubjectClientAndGrantExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-delegated@test.com", "pw", "PermissionDelegated", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	allowedByUser := core.MustDefinitionByCode("texture.update_visibility.owned")
	notAllowedByUser := core.MustDefinitionByCode("account.ban.any")
	clientOnlyMissing := core.MustDefinitionByCode("profile.create.owned")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_clients (id,owner_user_id,name,status,created_at,updated_at)
		VALUES ('client-1',$1,'Client','active',$2,$2)
	`, user.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_client_permissions (client_id,permission_id,created_at)
		VALUES ('client-1',$1,$3),('client-1',$2,$3)
	`, int64(allowedByUser.ID), int64(notAllowedByUser.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id,user_id,subject_id,client_id,status,created_at)
		VALUES ('grant-1',$1,$2,'client-1','active',$3)
	`, user.ID, permissiondb.SubjectIDForUser(user.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_grant_permissions (grant_id,permission_id,created_at)
		VALUES ('grant-1',$1,$4),('grant-1',$2,$4),('grant-1',$3,$4)
	`, int64(allowedByUser.ID), int64(notAllowedByUser.ID), int64(clientOnlyMissing.ID), now); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:       core.SessionKindWeb,
		Entrypoint:        core.EntrypointDashboard,
		DelegatedClientID: "client-1",
		DelegatedGrantID:  "grant-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, allowedByUser.Code) {
		t.Fatal("delegated permissions should keep permission allowed by user, client and grant")
	}
	if has(bits, notAllowedByUser.Code) {
		t.Fatal("delegated permissions must remove permission missing from subject effective permissions")
	}
	if has(bits, clientOnlyMissing.Code) {
		t.Fatal("delegated permissions must remove permission missing from client allow list")
	}
}

func has(bits core.BitSet, code string) bool {
	return bits.Has(core.MustDefinitionByCode(code).BitIndex)
}
