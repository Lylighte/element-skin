package permissions_test

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
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

func TestPermissionServiceSetAndClearOverrideInvalidatesAuthCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-write@test.com", "Password123", "AdminPermsWrite", true)
	target := testutil.CreateUser(t, db, "target-perms-write@test.com", "Password123", "TargetPermsWrite", false)
	cache := redisstore.NewMemoryStore()
	svc := permissionssvc.PermissionService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}

	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("auth cache should be invalidated after set, err=%v", err)
	}
	overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 1 || overrides[0].PermissionID != permission.MustDefinitionByCode("notice.create.any").ID ||
		overrides[0].PermissionCode != "notice.create.any" || overrides[0].Effect != "allow" || overrides[0].CreatedAt <= 0 {
		t.Fatalf("stored override mismatch: %#v", overrides)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, actor, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("auth cache should be invalidated after clear, err=%v", err)
	}
	overrides, err = db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Fatalf("override should be cleared exactly: %#v", overrides)
	}
}

func TestPermissionServiceRejectsUnauthorizedAndInvalidOverridePathsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-errors@test.com", "Password123", "AdminPermsErrors", true)
	target := testutil.CreateUser(t, db, "target-perms-errors@test.com", "Password123", "TargetPermsErrors", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	reader := actorWithPermissions(adminUser.ID, "permission.read.any")

	if _, err := svc.UserPermissions(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("read without permission error mismatch: %#v", err)
	}
	if _, err := svc.UserPermissions(ctx, reader, "missing-user"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("missing user read error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "missing.permission.any", "allow"); !httpErrorIs(err, http.StatusNotFound, "permission not found") {
		t.Fatalf("missing permission set error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "notice.create.any", "maybe"); !httpErrorIs(err, http.StatusBadRequest, "effect must be allow or deny") {
		t.Fatalf("invalid effect error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, reader, target.ID, "notice.create.any", "allow"); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("grant without permission error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, reader, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusNotFound, "permission override not found") {
		t.Fatalf("clear missing override error mismatch: %#v", err)
	}

	writer := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := svc.SetUserPermissionOverride(ctx, writer, "missing-user", "notice.create.any", "allow"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("set missing user error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, writer, "missing-user", "notice.create.any"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("clear missing user error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, writer, target.ID, "missing.permission.any"); !httpErrorIs(err, http.StatusNotFound, "permission not found") {
		t.Fatalf("clear missing permission error mismatch: %#v", err)
	}
}

func TestPermissionServiceProtectsProtectedPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-protected@test.com", "Password123", "AdminProtected", true)
	target := testutil.CreateUser(t, db, "target-protected@test.com", "Password123", "TargetProtected", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	grantOnly := actorWithPermissions(adminUser.ID, "permission.grant.any")

	err := svc.SetUserPermissionOverride(ctx, grantOnly, target.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected management permission must be transferred") {
		t.Fatalf("protected management grant error mismatch: %#v", err)
	}
	selfManager := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission_protected.manage.any")
	err = svc.SetUserPermissionOverride(ctx, selfManager, adminUser.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected management permission must be transferred") {
		t.Fatalf("self protected management grant error mismatch: %#v", err)
	}
	manager := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any", "permission_protected.manage.any")
	err = svc.SetUserPermissionOverride(ctx, manager, target.ID, "permission_protected.manage.any", "allow")
	if !httpErrorIs(err, http.StatusBadRequest, "protected management permission must be transferred") {
		t.Fatalf("manager protected management grant error mismatch: %#v", err)
	}
	if overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID); err != nil || len(overrides) != 0 {
		t.Fatalf("rejected protected management grants must not mutate overrides: overrides=%#v err=%v", overrides, err)
	}
	if err := svc.SetUserPermissionOverride(ctx, grantOnly, target.ID, "cache.invalidate.system", "allow"); !httpErrorIs(err, http.StatusForbidden, "protected permission management required") {
		t.Fatalf("system-scope grant without manage error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, manager, target.ID, "cache.invalidate.system", "allow"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, manager, target.ID, "cache.invalidate.system"); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionServiceClearRequiresOppositePermissionForDenyOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-clear-deny@test.com", "Password123", "AdminClearDeny", true)
	target := testutil.CreateUser(t, db, "target-clear-deny@test.com", "Password123", "TargetClearDeny", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	revoker := actorWithPermissions(adminUser.ID, "permission.revoke.any")
	granter := actorWithPermissions(adminUser.ID, "permission.grant.any")

	if err := svc.SetUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any", "deny"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("clear deny with revoke permission should fail: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, granter, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionServiceClearRequiresRevokePermissionForAllowOverrides(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-clear-allow@test.com", "Password123", "AdminClearAllow", true)
	target := testutil.CreateUser(t, db, "target-clear-allow@test.com", "Password123", "TargetClearAllow", false)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	granter := actorWithPermissions(adminUser.ID, "permission.grant.any")
	revoker := actorWithPermissions(adminUser.ID, "permission.revoke.any")

	if err := svc.SetUserPermissionOverride(ctx, granter, target.ID, "notice.create.any", "allow"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, granter, target.ID, "notice.create.any"); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("clear allow with grant permission should fail: %#v", err)
	}
	overrides, err := db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil || len(overrides) != 1 || overrides[0].PermissionCode != "notice.create.any" || overrides[0].Effect != "allow" {
		t.Fatalf("failed clear must preserve allow override: overrides=%#v err=%v", overrides, err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, revoker, target.ID, "notice.create.any"); err != nil {
		t.Fatal(err)
	}
	overrides, err = db.Permissions.SubjectPermissionOverridesForUser(ctx, target.ID)
	if err != nil || len(overrides) != 0 {
		t.Fatalf("revoker should clear allow override exactly: overrides=%#v err=%v", overrides, err)
	}
}

func TestPermissionServiceClosedDatabasePropagatesExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-closed@test.com", "Password123", "AdminPermsClosed", true)
	svc := permissionssvc.PermissionService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "permission.read.any", "permission.grant.any", "permission.revoke.any")
	db.Close()

	if _, err := svc.UserPermissions(ctx, actor, adminUser.ID); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UserPermissions closed db error mismatch: %#v", err)
	}
	if err := svc.SetUserPermissionOverride(ctx, actor, adminUser.ID, "notice.create.any", "allow"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("SetUserPermissionOverride closed db error mismatch: %#v", err)
	}
	if err := svc.ClearUserPermissionOverride(ctx, actor, adminUser.ID, "notice.create.any"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("ClearUserPermissionOverride closed db error mismatch: %#v", err)
	}
}

func actorWithPermissions(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   permissiondb.SubjectIDForUser(userID),
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}

func containsExact(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func stringSliceEqual(got, want []string) bool {
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

func hasOverride(overrides []permissionssvc.PermissionOverrideResponse, code, effect string) bool {
	for _, override := range overrides {
		if override.PermissionCode == code && override.Effect == effect && override.CreatedAt > 0 {
			return true
		}
	}
	return false
}
