package oauth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceClientCredentialsIssueAppOnlyActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials@test.com", "Password123", "OAuthClientCredentials", false)
	admin := testutil.CreateUser(t, db, "oauth-client-credentials-admin@test.com", "Password123", "OAuthClientCredentialsAdmin", true, true)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Server plugin",
		RedirectURI:     "https://server.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if clientID == "" || clientSecret == "" {
		t.Fatalf("client credentials response missing secret: %#v", clientRes)
	}
	grantClientPermission(t, db, clientID, "minecraft_profile.read.public")
	grantClientPermission(t, db, clientID, "minecraft_session.hasjoined.server")

	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "minecraft_session.hasjoined.server",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken != "" || token.TokenType != "Bearer" || token.ExpiresIn != 3600 ||
		token.Scope != "minecraft_session.hasjoined.server" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "minecraft_session.hasjoined.server" {
		t.Fatalf("client credentials token mismatch: %#v", token)
	}
	appActor, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || appActor.UserID != "" || appActor.SubjectID != permissiondb.SubjectIDForClient(clientID) ||
		appActor.SessionKind != permission.SessionKindClient || appActor.Entrypoint != permission.EntrypointAPI ||
		!appActor.Has(permission.MustDefinitionByCode("minecraft_session.hasjoined.server")) ||
		appActor.Has(permission.MustDefinitionByCode("minecraft_profile.read.public")) {
		t.Fatalf("app-only actor mismatch: ok=%v actor=%#v", ok, appActor)
	}
	introspection, err := svc.Introspect(ctx, adminActor, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if introspection["active"] != true || introspection["client_id"] != clientID ||
		introspection["subject_id"] != permissiondb.SubjectIDForClient(clientID) ||
		introspection["scope"] != "minecraft_session.hasjoined.server" {
		t.Fatalf("client credentials introspection mismatch: %#v", introspection)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, token.AccessToken); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.ActorForBearer(ctx, token.AccessToken); err != nil || ok {
		t.Fatalf("revoked app-only token should not authenticate: ok=%v err=%v", ok, err)
	}

	token, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "minecraft_session.hasjoined.server",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Permissions.SetPermissionOverrideForSubject(
		ctx,
		permissiondb.SubjectIDForClient(clientID),
		permission.MustDefinitionByCode("minecraft_session.hasjoined.server"),
		"deny",
		"",
	); err != nil {
		t.Fatal(err)
	}
	appActor, ok, err = svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || appActor.Has(permission.MustDefinitionByCode("minecraft_session.hasjoined.server")) {
		t.Fatalf("app-only actor should be narrowed after permission revoke: ok=%v actor=%#v", ok, appActor)
	}
	if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusDisabled, database.NowMS()); err != nil || !ok {
		t.Fatalf("disable client after token issue: ok=%v err=%v", ok, err)
	}
	appActor, ok, err = svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if ok || appActor.SubjectID != "" {
		t.Fatalf("disabled client token should not authenticate: ok=%v actor=%#v", ok, appActor)
	}
}

func TestServiceClientCredentialsReviewApprovalGrantsRequestedClientScopesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-client-credentials-invite@test.com", "Password123", "OAuthInviteClient", true, true)
	admin := testutil.CreateUser(t, db, "oauth-client-credentials-invite-admin@test.com", "Password123", "OAuthInviteAdmin", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:        "Invite manager",
		RedirectURI: "https://invite-manager.example/callback",
		ClientType:  oauth.ClientTypeConfidential,
		PermissionCodes: []string{
			"account.read.self",
			"invite.read.any",
			"invite.create.any",
			"invite.delete.any",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)

	reviewed, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive, "")
	if err != nil {
		t.Fatal(err)
	}
	if reviewed["status"] != oauth.StatusActive {
		t.Fatalf("reviewed client status mismatch: %#v", reviewed)
	}
	var selfOverrideCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM subject_permission_overrides
		WHERE subject_id=$1 AND permission_id=$2
	`, permissiondb.SubjectIDForClient(clientID), int64(permission.MustDefinitionByCode("account.read.self").ID)).Scan(&selfOverrideCount); err != nil {
		t.Fatal(err)
	}
	if selfOverrideCount != 0 {
		t.Fatalf("review should not grant user-context permission to client subject: count=%d", selfOverrideCount)
	}

	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "invite.read.any invite.create.any invite.delete.any",
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" ||
		token.RefreshToken != "" ||
		token.TokenType != "Bearer" ||
		token.ExpiresIn != 3600 ||
		token.Scope != "invite.create.any invite.delete.any invite.read.any" ||
		len(token.Permissions) != 3 ||
		token.Permissions[0] != "invite.create.any" ||
		token.Permissions[1] != "invite.delete.any" ||
		token.Permissions[2] != "invite.read.any" {
		t.Fatalf("review-approved invite client token mismatch: %#v", token)
	}
	appActor, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok ||
		appActor.UserID != "" ||
		appActor.SubjectID != permissiondb.SubjectIDForClient(clientID) ||
		appActor.SessionKind != permission.SessionKindClient ||
		appActor.Entrypoint != permission.EntrypointAPI ||
		!appActor.Has(permission.MustDefinitionByCode("invite.read.any")) ||
		!appActor.Has(permission.MustDefinitionByCode("invite.create.any")) ||
		!appActor.Has(permission.MustDefinitionByCode("invite.delete.any")) {
		t.Fatalf("review-approved invite app actor mismatch: ok=%v actor=%#v", ok, appActor)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "account.delete.any",
	})
	assertHTTPError(t, err, 403, "permission denied")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "account.read.self",
	})
	assertHTTPError(t, err, 403, "permission denied")
}

func TestServiceClientCredentialsRejectsPublicClientAndExcessScopeExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials-reject@test.com", "Password123", "OAuthClientReject", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	publicClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Public app",
		RedirectURI:     "https://public.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activateOAuthClient(t, db, publicClient["client_id"].(string))
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType: "client_credentials",
		ClientID:  publicClient["client_id"].(string),
	})
	assertHTTPError(t, err, 400, "client_credentials requires a confidential client")

	confidential, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Confidential app",
		RedirectURI:     "https://confidential.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	activateOAuthClient(t, db, confidential["client_id"].(string))
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     confidential["client_id"].(string),
		ClientSecret: confidential["client_secret"].(string),
		Scope:        "minecraft_session.hasjoined.server",
	})
	assertHTTPError(t, err, 403, "permission denied")
}

func TestServiceClientCredentialsDefaultScopeAndInactiveClientExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-credentials-default@test.com", "Password123", "OAuthClientDefault", true, true)
	if err := db.Permissions.SetSubjectPermissionOverride(
		ctx,
		user.ID,
		permission.MustDefinitionByCode("minecraft_session.hasjoined.server"),
		"allow",
		"",
	); err != nil {
		t.Fatal(err)
	}
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Default scope app",
		RedirectURI:     "https://default-scope.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public", "minecraft_session.hasjoined.server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	assertHTTPError(t, err, 400, "invalid client_id")
	activateOAuthClient(t, db, clientID)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	assertHTTPError(t, err, 403, "permission denied")

	grantClientPermission(t, db, clientID, "minecraft_profile.read.public")
	grantClientPermission(t, db, clientID, "minecraft_session.hasjoined.server")
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" ||
		token.RefreshToken != "" ||
		token.TokenType != "Bearer" ||
		token.ExpiresIn != 3600 ||
		token.Scope != "minecraft_profile.read.public minecraft_session.hasjoined.server" ||
		len(token.Permissions) != 2 ||
		token.Permissions[0] != "minecraft_profile.read.public" ||
		token.Permissions[1] != "minecraft_session.hasjoined.server" {
		t.Fatalf("default client credentials token mismatch: %#v", token)
	}
}

func TestServiceOAuthRevokedGrantRejectsAuthorizationCodeAndRefreshExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-revoked-grant-token@test.com", "Password123", "OAuthRevokedGrantToken", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Revoked grant token app",
		RedirectURI:     "https://revoked-grant-token.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	clientSecret := created["client_secret"].(string)
	activateOAuthClient(t, db, clientID)

	blockedVerifier := "revoked-grant-code-verifier"
	blockedApproval, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://revoked-grant-token.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(blockedVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	grants, err := svc.ListGrants(ctx, actor, 10)
	if err != nil {
		t.Fatal(err)
	}
	blockedGrantID := grants[0]["id"].(string)
	if err := svc.RevokeGrant(ctx, actor, blockedGrantID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         blockedApproval["code"].(string),
		CodeVerifier: blockedVerifier,
	})
	assertHTTPError(t, err, 400, "invalid authorization code")

	allowedVerifier := "active-grant-refresh-verifier"
	allowedApproval, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://revoked-grant-token.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(allowedVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         allowedApproval["code"].(string),
		CodeVerifier: allowedVerifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	grants, err = svc.ListGrants(ctx, actor, 10)
	if err != nil {
		t.Fatal(err)
	}
	activeGrantID := grants[0]["id"].(string)
	if err := svc.RevokeGrant(ctx, actor, activeGrantID); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	var activeRefreshCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM oauth_refresh_tokens
		WHERE client_id=$1 AND user_id=$2 AND revoked_at IS NULL
	`, clientID, user.ID).Scan(&activeRefreshCount); err != nil {
		t.Fatal(err)
	}
	if activeRefreshCount != 1 {
		t.Fatalf("revoked grant refresh failure should not rotate token, active refresh count=%d want 1", activeRefreshCount)
	}
}

func TestServiceTokenErrorBranchesAndBearerInvalidationExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-token-errors@test.com", "Password123", "OAuthTokenErrors", false)
	admin := testutil.CreateUser(t, db, "oauth-token-errors-admin@test.com", "Password123", "OAuthTokenErrorsAdmin", true, true)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	confidential, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Token error app",
		RedirectURI:     "https://token-errors.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := confidential["client_id"].(string)
	clientSecret := confidential["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	other, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Other token app",
		RedirectURI:     "https://other-token.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherID := other["client_id"].(string)
	otherSecret := other["client_secret"].(string)
	activateOAuthClient(t, db, otherID)

	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "password"})
	assertHTTPError(t, err, 400, "unsupported grant_type")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "client_credentials", ClientID: clientID, ClientSecret: "wrong"})
	assertHTTPError(t, err, 400, "invalid client_secret")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "authorization_code", ClientID: clientID, ClientSecret: clientSecret, Code: "missing-code", CodeVerifier: "valid-verifier-abcdefghijklmnopqrstuvwxyz"})
	assertHTTPError(t, err, 400, "invalid authorization code")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "refresh_token", ClientID: clientID, ClientSecret: clientSecret, RefreshToken: "missing-refresh"})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	for _, tc := range []struct {
		name     string
		verifier string
		detail   string
	}{
		{name: "wrong verifier", verifier: "wrong-verifier-abcdefghijklmnopqrstuvwxyz", detail: "invalid code_verifier"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifier := "token-verifier-abcdefghijklmnopqrstuvwxyz"
			approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://token-errors.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge(verifier),
				CodeChallengeMethod: "S256",
			})
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "authorization_code",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         approved["code"].(string),
				CodeVerifier: tc.verifier,
			})
			assertHTTPError(t, err, 400, tc.detail)
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "authorization_code",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         approved["code"].(string),
				CodeVerifier: verifier,
			})
			assertHTTPError(t, err, 400, "invalid authorization code")
		})
	}
	refreshVerifier := "refresh-revoke-verifier-abcdefghijklmnopqrstuvwxyz"
	approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://token-errors.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(refreshVerifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	refreshToken, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         approved["code"].(string),
		CodeVerifier: refreshVerifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, otherID, otherSecret, refreshToken.RefreshToken); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("wrong client refresh revoke error mismatch: %#v", err)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshToken.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshToken.RefreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	if err := svc.RevokeToken(ctx, clientID, clientSecret, "missing-token"); err != nil {
		t.Fatalf("revoking a missing token should be a no-op: %v", err)
	}
	if _, err := svc.Introspect(ctx, actor, "missing-token"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("introspect without permission error mismatch: %#v", err)
	}
	inactive, err := svc.Introspect(ctx, adminActor, "missing-token")
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) != 1 || inactive["active"] != false {
		t.Fatalf("missing token introspection mismatch: %#v", inactive)
	}
	if _, ok, err := svc.ActorForBearer(ctx, "missing-token"); err != nil || ok {
		t.Fatalf("missing bearer should not authenticate: ok=%v err=%v", ok, err)
	}

	now := database.NowMS()
	rawAccess := "raw-access-token"
	token := redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawAccess),
		ClientID:      otherID,
		UserID:        user.ID,
		GrantID:       "grant-1",
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     now + int64(time.Hour/time.Millisecond),
		CreatedAt:     now,
	}
	if err := svc.Redis.SetOAuthAccessToken(ctx, token, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, rawAccess); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("wrong client revoke error mismatch: %#v", err)
	}
	if err := svc.RevokeToken(ctx, otherID, otherSecret, rawAccess); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := svc.ActorForBearer(ctx, rawAccess); err != nil || ok {
		t.Fatalf("revoked wrong-client test token should not authenticate: ok=%v err=%v", ok, err)
	}

	expiredRaw := "expired-access-token"
	expiredToken := redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(expiredRaw),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("minecraft_session.hasjoined.server").ID)},
		ExpiresAt:     now - 1000,
		CreatedAt:     now - 2000,
	}
	if err := svc.Redis.SetOAuthAccessToken(ctx, expiredToken, time.Hour); err != nil {
		t.Fatal(err)
	}
	introspection, err := svc.Introspect(ctx, adminActor, expiredRaw)
	if err != nil {
		t.Fatal(err)
	}
	if len(introspection) != 1 || introspection["active"] != false {
		t.Fatalf("expired introspection mismatch: %#v", introspection)
	}
	if _, ok, err := svc.ActorForBearer(ctx, expiredRaw); err != nil || ok {
		t.Fatalf("expired bearer should not authenticate: ok=%v err=%v", ok, err)
	}
}

func TestServiceOAuthAccessRedisFailuresReturnExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-redis-fail@test.com", "Password123", "OAuthRedisFail", false)
	admin := testutil.CreateUser(t, db, "oauth-redis-fail-admin@test.com", "Password123", "OAuthRedisFailAdmin", true, true)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	healthy := newOAuthService(db)
	clientRes, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Redis failure app",
		RedirectURI:     "https://redis-fail.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"minecraft_profile.read.public"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	grantClientPermission(t, db, clientID, "minecraft_profile.read.public")

	forced := errors.New("oauth access cache unavailable")
	failingRedis := redisstore.NewMemoryStore()
	failingRedis.Err = forced
	failingSvc := oauth.Service{DB: db, Redis: failingRedis}
	_, err = failingSvc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "client_credentials",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        "minecraft_profile.read.public",
	})
	if !errors.Is(err, forced) {
		t.Fatalf("client credentials should return exact redis set error: %v", err)
	}
	if _, ok, err := failingSvc.ActorForBearer(ctx, "any-token"); !errors.Is(err, forced) || ok {
		t.Fatalf("ActorForBearer redis error mismatch: ok=%v err=%v", ok, err)
	}
	if _, err := failingSvc.Introspect(ctx, adminActor, "any-token"); !errors.Is(err, forced) {
		t.Fatalf("Introspect redis error mismatch: %v", err)
	}
	if err := failingSvc.RevokeToken(ctx, clientID, clientSecret, "any-token"); !errors.Is(err, forced) {
		t.Fatalf("RevokeToken redis error mismatch: %v", err)
	}
}

func TestServiceOAuthTokenIssuanceRedisFailuresKeepExactDatabaseState(t *testing.T) {
	t.Run("authorization code creates refresh token and consumes code before access cache failure", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-code-redis-fail@test.com", "Password123", "OAuthCodeRedisFail", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		healthy := newOAuthService(db)
		created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
			Name:            "Code redis fail app",
			RedirectURI:     "https://code-redis-fail.example/callback",
			ClientType:      oauth.ClientTypeConfidential,
			PermissionCodes: []string{"account.read.self"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		clientSecret := created["client_secret"].(string)
		activateOAuthClient(t, db, clientID)
		verifier := "code-redis-fail-verifier"
		approved, err := healthy.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
			ResponseType:        "code",
			ClientID:            clientID,
			RedirectURI:         "https://code-redis-fail.example/callback",
			Scope:               "account.read.self",
			CodeChallenge:       pkceChallenge(verifier),
			CodeChallengeMethod: "S256",
			State:               "redis-fail-state",
		})
		if err != nil {
			t.Fatal(err)
		}
		code := approved["code"].(string)
		forced := errors.New("oauth access cache unavailable after code exchange")
		failingRedis := redisstore.NewMemoryStore()
		failingRedis.Err = forced
		failing := oauth.Service{DB: db, Redis: failingRedis}
		_, err = failing.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "authorization_code",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Code:         code,
			RedirectURI:  "https://code-redis-fail.example/callback",
			CodeVerifier: verifier,
		})
		if !errors.Is(err, forced) {
			t.Fatalf("authorization code redis failure mismatch: got=%v want=%v", err, forced)
		}
		var consumedAt *int64
		if err := db.Pool.QueryRow(ctx, `SELECT consumed_at FROM oauth_authorization_codes WHERE code_hash=$1`, util.HashRefreshToken(code)).Scan(&consumedAt); err != nil {
			t.Fatal(err)
		}
		if consumedAt == nil || *consumedAt <= 0 {
			t.Fatalf("authorization code should be consumed before cache failure, consumed_at=%v", consumedAt)
		}
		var refreshCount int
		if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE client_id=$1 AND user_id=$2`, clientID, user.ID).Scan(&refreshCount); err != nil {
			t.Fatal(err)
		}
		if refreshCount != 1 {
			t.Fatalf("refresh token row count after failed access cache write=%d want=1", refreshCount)
		}
	})

	t.Run("refresh token rotates before access cache failure", func(t *testing.T) {
		db, _ := testutil.NewTestAppTB(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "oauth-refresh-redis-fail@test.com", "Password123", "OAuthRefreshRedisFail", false)
		actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
		if err != nil {
			t.Fatal(err)
		}
		healthy := newOAuthService(db)
		created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
			Name:            "Refresh redis fail app",
			RedirectURI:     "https://refresh-redis-fail.example/callback",
			ClientType:      oauth.ClientTypeConfidential,
			PermissionCodes: []string{"account.read.self"},
		})
		if err != nil {
			t.Fatal(err)
		}
		clientID := created["client_id"].(string)
		clientSecret := created["client_secret"].(string)
		activateOAuthClient(t, db, clientID)
		verifier := "refresh-redis-fail-verifier"
		approved, err := healthy.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
			ResponseType:        "code",
			ClientID:            clientID,
			RedirectURI:         "https://refresh-redis-fail.example/callback",
			Scope:               "account.read.self",
			CodeChallenge:       pkceChallenge(verifier),
			CodeChallengeMethod: "S256",
		})
		if err != nil {
			t.Fatal(err)
		}
		token, err := healthy.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "authorization_code",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Code:         approved["code"].(string),
			RedirectURI:  "https://refresh-redis-fail.example/callback",
			CodeVerifier: verifier,
		})
		if err != nil {
			t.Fatal(err)
		}
		forced := errors.New("oauth access cache unavailable after refresh")
		failingRedis := redisstore.NewMemoryStore()
		failingRedis.Err = forced
		failing := oauth.Service{DB: db, Redis: failingRedis}
		_, err = failing.IssueToken(ctx, oauth.TokenRequest{
			GrantType:    "refresh_token",
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RefreshToken: token.RefreshToken,
		})
		if !errors.Is(err, forced) {
			t.Fatalf("refresh redis failure mismatch: got=%v want=%v", err, forced)
		}
		old, err := db.OAuth.GetRefreshToken(ctx, util.HashRefreshToken(token.RefreshToken))
		if err != nil {
			t.Fatal(err)
		}
		if old == nil || old.RevokedAt == nil || *old.RevokedAt <= 0 {
			t.Fatalf("old refresh token should be revoked after rotate-before-cache-failure: %#v", old)
		}
		var activeRefreshCount int
		if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE client_id=$1 AND user_id=$2 AND revoked_at IS NULL`, clientID, user.ID).Scan(&activeRefreshCount); err != nil {
			t.Fatal(err)
		}
		if activeRefreshCount != 1 {
			t.Fatalf("active refresh token row count=%d want=1", activeRefreshCount)
		}
	})
}
