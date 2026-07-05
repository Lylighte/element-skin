package oauth_test

import (
	"context"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestDeleteRevokedGrantsDeletesOnlyExpiredRevokedRowsAndDependenciesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-grant-cleanup@test.com", "pw", "OAuthGrantCleanup", false)
	client := model.OAuthClient{
		ID:          "client-grant-cleanup",
		OwnerUserID: user.ID,
		Name:        "Grant cleanup client",
		RedirectURI: "https://cleanup.example/callback",
		ClientType:  "public",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	permissions := permissionIDs("profile.read.owned", "notice.read.owned")
	if err := db.OAuth.CreateClient(ctx, client, permissions); err != nil {
		t.Fatal(err)
	}
	oldRevokedAt := int64(1000)
	recentRevokedAt := int64(3000)
	grants := []model.OAuthGrant{
		{ID: "grant-cleanup-old", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "revoked", CreatedAt: 900, RevokedAt: &oldRevokedAt},
		{ID: "grant-cleanup-recent", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "revoked", CreatedAt: 2900, RevokedAt: &recentRevokedAt},
		{ID: "grant-cleanup-active", UserID: user.ID, SubjectID: permissiondb.SubjectIDForUser(user.ID), ClientID: client.ID, Status: "active", CreatedAt: 3100},
	}
	for _, grant := range grants {
		if err := db.OAuth.CreateGrant(ctx, grant, permissions); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
			TokenHash: "refresh-" + grant.ID,
			ClientID:  client.ID,
			UserID:    user.ID,
			GrantID:   grant.ID,
			ExpiresAt: 10000,
			CreatedAt: grant.CreatedAt,
		}); err != nil {
			t.Fatal(err)
		}
		if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
			CodeHash:            "code-" + grant.ID,
			ClientID:            client.ID,
			UserID:              user.ID,
			GrantID:             grant.ID,
			RedirectURI:         client.RedirectURI,
			CodeChallenge:       "challenge",
			CodeChallengeMethod: "S256",
			ExpiresAt:           10000,
			CreatedAt:           grant.CreatedAt,
		}, permissions); err != nil {
			t.Fatal(err)
		}
	}

	deleted, err := db.OAuth.DeleteRevokedGrants(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("DeleteRevokedGrants deleted=%d want 1", deleted)
	}
	assertOAuthGrantExists(t, db, "grant-cleanup-old", false)
	assertOAuthGrantExists(t, db, "grant-cleanup-recent", true)
	assertOAuthGrantExists(t, db, "grant-cleanup-active", true)
	assertOAuthDependencyCount(t, db, "grant-cleanup-old", 0)
	assertOAuthDependencyCount(t, db, "grant-cleanup-recent", 6)
	assertOAuthDependencyCount(t, db, "grant-cleanup-active", 6)

	deleted, err = db.OAuth.DeleteRevokedGrants(ctx, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("second DeleteRevokedGrants deleted=%d want 0", deleted)
	}
}
