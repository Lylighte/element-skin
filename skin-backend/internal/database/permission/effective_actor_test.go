package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestActorForUserExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-for-user@test.com", "pw", "ActorForUser", false)

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindWeb,
		Entrypoint:  core.EntrypointDashboard,
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.SubjectID != permissiondb.SubjectIDForUser(user.ID) {
		t.Fatalf("SubjectID=%q want=%q", actor.SubjectID, permissiondb.SubjectIDForUser(user.ID))
	}
	if actor.UserID != user.ID {
		t.Fatalf("UserID=%q want=%q", actor.UserID, user.ID)
	}
	if actor.SessionKind != core.SessionKindWeb || actor.Entrypoint != core.EntrypointDashboard {
		t.Fatalf("session fields mismatch: kind=%q entrypoint=%q", actor.SessionKind, actor.Entrypoint)
	}
	if actor.Permissions.Empty() {
		t.Fatal("actor should have non-empty permissions")
	}
	if !actor.Permissions.Has(core.MustDefinitionByCode("profile.create.owned").BitIndex) {
		t.Fatal("web actor should have profile.create.owned")
	}
	if actor.Permissions.Has(core.MustDefinitionByCode("notice.create.any").BitIndex) {
		t.Fatal("normal user should not have notice.create.any in actor")
	}
}

func TestActorForUserWithBanPolicy(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-ban@test.com", "pw", "ActorBan", false)
	if err := db.Users.Ban(ctx, user.ID, time.Now().Add(time.Hour).UnixMilli()); err != nil {
		t.Fatal(err)
	}

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	joinBit := core.MustDefinitionByCode("yggdrasil_server.join.bound_profile").BitIndex
	if actor.Permissions.Has(joinBit) {
		t.Fatal("banned user joined via actor should have join permission cleared")
	}
	hasJoinedBit := core.MustDefinitionByCode("yggdrasil_server.hasjoined.bound_profile").BitIndex
	if !actor.Permissions.Has(hasJoinedBit) {
		t.Fatal("banned user should still have hasjoined permission")
	}
}

func TestActorForUserWithDelegationFieldsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "actor-delegation@test.com", "pw", "ActorDelegation", false)
	if err := db.Permissions.EnsureUserSubject(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UnixMilli()
	def := core.MustDefinitionByCode("texture.update_visibility.owned")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_clients (id,owner_user_id,name,status,created_at,updated_at)
		VALUES ('actor-client',$1,'ActorClient','active',$2,$2)
	`, user.ID, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_client_permissions (client_id,permission_id,created_at)
		VALUES ('actor-client',$1,$2)
	`, int64(def.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id,user_id,subject_id,client_id,status,created_at)
		VALUES ('actor-grant',$1,$2,'actor-client','active',$3)
	`, user.ID, permissiondb.SubjectIDForUser(user.ID), now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO delegated_grant_permissions (grant_id,permission_id,created_at)
		VALUES ('actor-grant',$1,$2)
	`, int64(def.ID), now); err != nil {
		t.Fatal(err)
	}

	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:       core.SessionKindWeb,
		Entrypoint:        core.EntrypointDashboard,
		DelegatedGrantID:  "actor-grant",
		DelegatedClientID: "actor-client",
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.DelegationID != "actor-grant" {
		t.Fatalf("DelegationID=%q want=actor-grant", actor.DelegationID)
	}
	if actor.DelegatedClientID != "actor-client" {
		t.Fatalf("DelegatedClientID=%q want=actor-client", actor.DelegatedClientID)
	}
	if !actor.Permissions.Has(def.BitIndex) {
		t.Fatal("delegated actor should have the granted permission")
	}
}
