package permission_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

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
