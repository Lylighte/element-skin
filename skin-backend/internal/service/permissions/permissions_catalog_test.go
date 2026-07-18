package permissions_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
)

func TestPermissionServiceUserPermissionsReturnsExactCatalogRolesAndOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-read@test.com", "Password123", "AdminPermsRead", true)
	target := testutil.CreateUser(t, db, "target-perms-read@test.com", "Password123", "TargetPermsRead", false)
	actor := actorWithPermissions(adminUser.ID, "permission.read.any")
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}

	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleModerator, actor.SubjectID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, target.ID, permission.MustDefinitionByCode("notice.create.any"), "allow", actor.SubjectID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, target.ID, permission.MustDefinitionByCode("texture.delete.owned"), "deny", actor.SubjectID); err != nil {
		t.Fatal(err)
	}

	got, err := svc.UserPermissions(ctx, actor, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !stringSliceEqual(got.Roles, []string{permission.RoleModerator, permission.RoleUser}) {
		t.Fatalf("roles mismatch: got=%v", got.Roles)
	}
	if !containsExact(got.EffectivePermissions, "notice.create.any") {
		t.Fatalf("effective permissions should include override allow: %v", got.EffectivePermissions)
	}
	if containsExact(got.EffectivePermissions, "texture.delete.owned") {
		t.Fatalf("effective permissions should exclude override deny: %v", got.EffectivePermissions)
	}
	wantOverrides := []permissionssvc.PermissionOverrideResponse{
		{PermissionCode: "texture.delete.owned", Effect: "deny"},
		{PermissionCode: "notice.create.any", Effect: "allow"},
	}
	if len(got.Overrides) != 2 {
		t.Fatalf("override count=%d want 2: %#v", len(got.Overrides), got.Overrides)
	}
	for _, want := range wantOverrides {
		if !hasOverride(got.Overrides, want.PermissionCode, want.Effect) {
			t.Fatalf("missing override %#v in %#v", want, got.Overrides)
		}
	}
	if len(got.Catalog.Permissions) != len(permission.Definitions) || len(got.Catalog.Roles) != len(permission.Roles) {
		t.Fatalf("catalog size mismatch: permissions=%d/%d roles=%d/%d",
			len(got.Catalog.Permissions), len(permission.Definitions), len(got.Catalog.Roles), len(permission.Roles))
	}
	first := got.Catalog.Permissions[0]
	def := permission.Definitions[0]
	if first.ID != int64(def.ID) || first.Code != def.Code || first.BitIndex != def.BitIndex ||
		first.Resource != def.Resource.Code || first.Action != def.Action.Code || first.Scope != def.Scope.Code {
		t.Fatalf("first catalog permission mismatch: got=%#v want=%#v", first, def)
	}
}
