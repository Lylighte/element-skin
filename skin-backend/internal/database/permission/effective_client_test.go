package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestActorForClientUsesClientSubjectAndAPISessionPolicy(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	clientID := "actor-client-only"
	if err := db.Permissions.EnsureClientSubject(ctx, clientID); err != nil {
		t.Fatal(err)
	}
	serverDef := core.MustDefinitionByCode("minecraft_session.hasjoined.server")
	userContextDef := core.MustDefinitionByCode("account.read.self")
	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO subject_permission_overrides (subject_id, permission_id, effect, created_at)
		VALUES
			($1,$2,'allow',$4),
			($1,$3,'allow',$4)
	`, permissiondb.SubjectIDForClient(clientID), int64(serverDef.ID), int64(userContextDef.ID), time.Now().UnixMilli()); err != nil {
		t.Fatal(err)
	}

	actor, err := db.Permissions.ActorForClient(ctx, clientID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindClient,
		Entrypoint:  core.EntrypointAPI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if actor.SubjectID != permissiondb.SubjectIDForClient(clientID) {
		t.Fatalf("SubjectID=%q want=%q", actor.SubjectID, permissiondb.SubjectIDForClient(clientID))
	}
	if actor.UserID != "" {
		t.Fatalf("client actor UserID=%q want empty", actor.UserID)
	}
	if actor.DelegatedClientID != clientID {
		t.Fatalf("DelegatedClientID=%q want=%q", actor.DelegatedClientID, clientID)
	}
	if !actor.Permissions.Has(serverDef.BitIndex) {
		t.Fatal("client actor should include granted minecraft_session.hasjoined.server")
	}
	if actor.Permissions.Has(userContextDef.BitIndex) {
		t.Fatal("client credentials API policy should remove account.read.self")
	}
}

func TestEnsureClientSubjectIsIdempotentWithExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	clientID := "subject-client"
	if err := db.Permissions.EnsureClientSubject(ctx, clientID); err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.EnsureClientSubject(ctx, clientID); err != nil {
		t.Fatal(err)
	}
	var id, kind, status string
	var userID *string
	if err := db.Pool.QueryRow(ctx, `
		SELECT id, user_id, kind, status
		FROM permission_subjects
		WHERE id=$1
	`, permissiondb.SubjectIDForClient(clientID)).Scan(&id, &userID, &kind, &status); err != nil {
		t.Fatal(err)
	}
	if id != permissiondb.SubjectIDForClient(clientID) || userID != nil || kind != "client" || status != "active" {
		t.Fatalf("client subject fields mismatch: id=%q user_id=%v kind=%q status=%q", id, userID, kind, status)
	}
}
