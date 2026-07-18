package permission_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestSetSubjectPermissionOverrideAllowEffect(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-allow@test.com", "pw", "OverrideAllow", false)
	admin := testutil.CreateUser(t, db, "override-admin@test.com", "pw", "OverrideAdmin", true)
	allowDef := core.MustDefinitionByCode("notice.create.any")
	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(before, allowDef.Code) {
		t.Fatal("normal user should not have notice.create.any before override")
	}
	grantorSubjectID := permissiondb.SubjectIDForUser(admin.ID)
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, allowDef, "allow", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(after, allowDef.Code) {
		t.Fatal("allow override should grant notice.create.any")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, allowDef, "deny", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	afterDeny, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(afterDeny, allowDef.Code) {
		t.Fatal("deny override should supersede allow override")
	}
}

func TestSubjectPermissionOverridesListAndClearExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-list@test.com", "pw", "OverrideList", false)
	admin := testutil.CreateUser(t, db, "override-list-admin@test.com", "pw", "OverrideListAdmin", true)
	denyDef := core.MustDefinitionByCode("texture.update_visibility.owned")
	allowDef := core.MustDefinitionByCode("notice.create.any")
	grantorSubjectID := permissiondb.SubjectIDForUser(admin.ID)

	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, denyDef, "deny", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, allowDef, "allow", grantorSubjectID); err != nil {
		t.Fatal(err)
	}
	overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 2 ||
		overrides[0].PermissionID != allowDef.ID ||
		overrides[0].PermissionCode != "notice.create.any" ||
		overrides[0].Effect != "allow" ||
		overrides[0].CreatedAt <= 0 ||
		overrides[1].PermissionID != denyDef.ID ||
		overrides[1].PermissionCode != "texture.update_visibility.owned" ||
		overrides[1].Effect != "deny" ||
		overrides[1].CreatedAt <= 0 {
		t.Fatalf("override list mismatch: %#v", overrides)
	}

	cleared, err := db.Permissions.ClearSubjectPermissionOverride(ctx, user.ID, allowDef)
	if err != nil || !cleared {
		t.Fatalf("ClearSubjectPermissionOverride allow = %v, %v; want true, nil", cleared, err)
	}
	clearedAgain, err := db.Permissions.ClearSubjectPermissionOverride(ctx, user.ID, allowDef)
	if err != nil || clearedAgain {
		t.Fatalf("ClearSubjectPermissionOverride missing = %v, %v; want false, nil", clearedAgain, err)
	}
	overrides, err = db.Permissions.SubjectPermissionOverridesForUser(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 1 || overrides[0].PermissionCode != "texture.update_visibility.owned" || overrides[0].Effect != "deny" {
		t.Fatalf("override list after clear mismatch: %#v", overrides)
	}
}

func TestSetSubjectPermissionOverrideRejectsInvalidEffect(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-invalid@test.com", "pw", "OverrideInvalid", false)
	def := core.MustDefinitionByCode("notice.create.any")
	err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "invalid", "")
	if err == nil || err.Error() != "permission override effect must be allow or deny" {
		t.Fatalf("invalid override effect should be rejected exactly: %v", err)
	}
}

func TestSetSubjectPermissionOverrideIdempotent(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-idempotent@test.com", "pw", "OverrideIdempotent", false)
	def := core.MustDefinitionByCode("texture.update_visibility.owned")

	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "deny", ""); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if has(bits, def.Code) {
		t.Fatal("first deny should remove the permission")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	bits, err = db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, def.Code) {
		t.Fatal("allow override after deny should restore the permission")
	}
}

func TestSetSubjectPermissionOverrideCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	def := core.MustDefinitionByCode("notice.create.any")
	err := db.Permissions.SetSubjectPermissionOverride(ctx, "nonexistent", def, "allow", "")
	assertCancelled(t, err)
}

func TestSubjectPermissionOverridesForUserRowsScanError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-rows-scan@test.com", "pw", "OverrideRowsScan", false)
	def := core.MustDefinitionByCode("notice.create.any")
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	fc := testutil.NewFaultyConn(db.Pool).WithScanError(testutil.ErrFaultInjected)
	db.Permissions.SetTestConn(fc)
	_, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, user.ID)
	if err != testutil.ErrFaultInjected {
		t.Fatalf("SubjectPermissionOverridesForUser scan error=%v, want injected fault", err)
	}
}

func TestSubjectPermissionOverridesForUserRowsErr(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-rows-err@test.com", "pw", "OverrideRowsErr", false)
	def := core.MustDefinitionByCode("notice.create.any")
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "allow", ""); err != nil {
		t.Fatal(err)
	}
	fc := testutil.NewFaultyConn(db.Pool).WithRowsErr(testutil.ErrFaultInjected)
	db.Permissions.SetTestConn(fc)
	_, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, user.ID)
	if err != testutil.ErrFaultInjected {
		t.Fatalf("SubjectPermissionOverridesForUser rows error=%v, want injected fault", err)
	}
}

func TestSubjectPermissionOverridesForUserQueryError(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "override-query-err@test.com", "pw", "OverrideQueryErr", false)
	if _, err := db.Pool.Exec(ctx, `DROP TABLE subject_permission_overrides CASCADE`); err != nil {
		t.Fatal(err)
	}
	_, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, user.ID)
	assertPostgresError(t, err, "42P01")
}

func TestClearSubjectPermissionOverrideCancelledContext(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	def := core.MustDefinitionByCode("notice.create.any")
	cleared, err := db.Permissions.ClearSubjectPermissionOverride(ctx, "nonexistent", def)
	if cleared {
		t.Fatalf("ClearSubjectPermissionOverride cancelled cleared=%v, want false", cleared)
	}
	assertCancelled(t, err)
}
