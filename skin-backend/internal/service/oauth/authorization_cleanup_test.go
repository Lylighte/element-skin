package oauth_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

func TestServiceDeleteExpiredRevokedGrantsRequiresSystemMaintenanceAndUsesRetentionExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant-cleanup-service@test.com", "Password123", "OAuthGrantCleanupService", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	client, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Cleanup service app",
		RedirectURI:     "https://cleanup-service.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"profile.read.owned"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := client["client_id"].(string)
	now := int64(10_000_000_000)
	expiredRevokedAt := now - int64(oauth.RevokedGrantRetention/time.Millisecond)
	recentRevokedAt := expiredRevokedAt + 1
	expired := model.OAuthGrant{ID: "service-expired-revoked-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: clientID, Status: "revoked", CreatedAt: 1000, RevokedAt: &expiredRevokedAt}
	recent := model.OAuthGrant{ID: "service-recent-revoked-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: clientID, Status: "revoked", CreatedAt: 2000, RevokedAt: &recentRevokedAt}
	active := model.OAuthGrant{ID: "service-active-grant", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: clientID, Status: oauth.StatusActive, CreatedAt: 3000}
	for _, grant := range []model.OAuthGrant{expired, recent, active} {
		if err := db.OAuth.CreateGrant(ctx, grant, permissionIDsFromCodesForTest([]string{"profile.read.owned"})); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := svc.DeleteExpiredRevokedGrants(ctx, actor, now)
	assertHTTPError(t, err, 403, "permission denied")
	if deleted != 0 {
		t.Fatalf("non-system cleanup deleted=%d want 0", deleted)
	}

	deleted, err = svc.DeleteExpiredRevokedGrants(ctx, permission.SystemMaintenanceActor(), now)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("system cleanup deleted=%d want 1", deleted)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, user.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 || grants[0].ID != active.ID || grants[1].ID != recent.ID {
		t.Fatalf("remaining grants mismatch: %#v", grants)
	}
}
