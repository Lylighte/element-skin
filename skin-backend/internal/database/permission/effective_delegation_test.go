package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

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
