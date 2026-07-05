package permissions_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/redisstore"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
)

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
