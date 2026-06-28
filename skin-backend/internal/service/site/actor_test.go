package site_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func testActor(t testing.TB, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(context.Background(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("create test actor: %v", err)
	}
	return actor
}
