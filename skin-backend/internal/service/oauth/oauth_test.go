package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestServiceAuthorizationCodeFlowNarrowsActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-service@test.com", "Password123", "OAuthService", false)
	admin := testutil.CreateUser(t, db, "oauth-service-admin@test.com", "Password123", "OAuthServiceAdmin", true, true)
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
		Name:            "Service app",
		Description:     "Service app description",
		RedirectURI:     "https://client.example/callback",
		WebsiteURL:      "https://client.example",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "account.update.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	if clientID == "" || clientSecret == "" {
		t.Fatalf("client response should include exact credentials: %#v", clientRes)
	}
	permissions := clientRes["permissions"].([]string)
	if len(permissions) != 2 || permissions[0] != "account.read.self" || permissions[1] != "account.update.self" {
		t.Fatalf("client permissions mismatch: %#v", permissions)
	}

	verifier := "service-verifier-abcdefghijklmnopqrstuvwxyz"
	challenge := pkceChallenge(verifier)
	details, err := svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		State:               "state-service",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	if details.RedirectURI != "https://client.example/callback" || details.State != "state-service" || len(details.Scopes) != 1 {
		t.Fatalf("authorization details mismatch: %#v", details)
	}
	approved, err := svc.ApproveAuthorization(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		State:               "state-service",
		CodeChallenge:       challenge,
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	code := approved["code"].(string)
	redirectURL := approved["redirect_url"].(string)
	if code == "" || !strings.Contains(redirectURL, "state=state-service") {
		t.Fatalf("approve response mismatch: %#v", approved)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Code:         code,
		RedirectURI:  "https://client.example/callback",
		CodeVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.TokenType != "Bearer" || token.Scope != "account.read.self" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "account.read.self" {
		t.Fatalf("token response mismatch: %#v", token)
	}
	delegated, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || delegated.UserID != user.ID || !delegated.Has(permission.MustDefinitionByCode("account.read.self")) || delegated.Has(permission.MustDefinitionByCode("account.update.self")) {
		t.Fatalf("delegated actor mismatch: ok=%v actor=%#v", ok, delegated)
	}
	introspection, err := svc.Introspect(ctx, adminActor, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if introspection["active"] != true || introspection["client_id"] != clientID || introspection["user_id"] != user.ID ||
		introspection["grant_id"] == "" || introspection["scope"] != "account.read.self" {
		t.Fatalf("introspection mismatch: %#v", introspection)
	}
	refreshed, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken == "" || refreshed.AccessToken == token.AccessToken || refreshed.RefreshToken == "" ||
		refreshed.RefreshToken == token.RefreshToken || refreshed.Scope != "account.read.self" {
		t.Fatalf("refresh response mismatch: %#v", refreshed)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: token.RefreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshed.AccessToken); err != nil {
		t.Fatal(err)
	}
	inactive, err := svc.Introspect(ctx, adminActor, refreshed.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if len(inactive) != 1 || inactive["active"] != false {
		t.Fatalf("revoked access introspection mismatch: %#v", inactive)
	}
	if err := svc.RevokeToken(ctx, clientID, clientSecret, refreshed.RefreshToken); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "refresh_token",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RefreshToken: refreshed.RefreshToken,
	})
	assertHTTPError(t, err, 400, "invalid refresh_token")
}

func TestServiceRejectsInvalidAuthorizationRequestExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-invalid@test.com", "Password123", "OAuthInvalid", false)
	other := testutil.CreateUser(t, db, "oauth-invalid-other@test.com", "Password123", "OAuthInvalidOther", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	client, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Invalid app",
		RedirectURI:     "https://client.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := client["client_id"].(string)
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{ResponseType: "token"})
	assertHTTPError(t, err, 400, "response_type must be code")
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge("invalid-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	})
	assertHTTPError(t, err, 400, "invalid client_id")
	activateOAuthClient(t, db, clientID)
	baseReq := oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://client.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge("invalid-verifier-abcdefghijklmnopqrstuvwxyz"),
		CodeChallengeMethod: "S256",
	}
	for _, tc := range []struct {
		name   string
		req    oauth.AuthorizationRequest
		status int
		detail string
		actor  permission.Actor
	}{
		{name: "bad client", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.ClientID = "missing-client" }), status: 400, detail: "invalid client_id", actor: actor},
		{name: "bad redirect", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.RedirectURI = "https://client.example/other" }), status: 400, detail: "invalid redirect_uri", actor: actor},
		{name: "missing pkce", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.CodeChallengeMethod = "plain" }), status: 400, detail: "PKCE S256 is required", actor: actor},
		{name: "empty scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "" }), status: 400, detail: "scope is required", actor: actor},
		{name: "invalid scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "permission.catalog.system" }), status: 400, detail: "invalid scope", actor: actor},
		{name: "actor lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "texture.delete.any" }), status: 403, detail: "permission denied", actor: otherActor},
		{name: "client lacks scope", req: withAuthReq(baseReq, func(req *oauth.AuthorizationRequest) { req.Scope = "account.update.self" }), status: 400, detail: "scope exceeds client permission limit", actor: actor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.AuthorizationDetails(ctx, tc.actor, tc.req)
			assertHTTPError(t, err, tc.status, tc.detail)
		})
	}
}

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

func TestServiceClientManagementReviewSecretDeleteAndAdminListExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-owner-manage@test.com", "Password123", "OAuthOwnerManage", false)
	other := testutil.CreateUser(t, db, "oauth-other-manage@test.com", "Password123", "OAuthOtherManage", false)
	admin := testutil.CreateUser(t, db, "oauth-admin-manage@test.com", "Password123", "OAuthAdminManage", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	otherActor, err := db.Permissions.ActorForUser(ctx, other.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Managed app",
		Description:     "Original description",
		RedirectURI:     "https://managed.example/callback",
		WebsiteURL:      "https://managed.example",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self", "account.update.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	firstSecret := created["client_secret"].(string)
	if clientID == "" || firstSecret == "" || created["status"] != oauth.StatusPending {
		t.Fatalf("created client mismatch: %#v", created)
	}
	if _, err := svc.GetClient(ctx, otherActor, clientID); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("other user get client error mismatch: %#v", err)
	}
	gotClient, err := svc.GetClient(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if gotClient["client_id"] != clientID ||
		gotClient["name"] != "Managed app" ||
		gotClient["description"] != "Original description" ||
		gotClient["redirect_uri"] != "https://managed.example/callback" ||
		gotClient["website_url"] != "https://managed.example" ||
		gotClient["client_type"] != oauth.ClientTypeConfidential ||
		gotClient["status"] != oauth.StatusPending {
		t.Fatalf("owned client detail mismatch: %#v", gotClient)
	}

	ownedList, err := svc.ListClients(ctx, ownerActor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(ownedList) != 1 || ownedList[0]["client_id"] != clientID || ownedList[0]["name"] != "Managed app" {
		t.Fatalf("owned list mismatch: %#v", ownedList)
	}
	if _, err := svc.ListClients(ctx, permission.Actor{}, 10); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("list without owned permission error mismatch: %#v", err)
	}
	if _, err := svc.ListClientsForAdmin(ctx, adminActor, "weird", 10); !isHTTPError(err, 400, "invalid status") {
		t.Fatalf("admin list invalid status error mismatch: %#v", err)
	}
	pendingList, err := svc.ListClientsForAdmin(ctx, adminActor, oauth.StatusPending, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(pendingList) != 1 || pendingList[0]["client_id"] != clientID || pendingList[0]["status"] != oauth.StatusPending {
		t.Fatalf("pending admin list mismatch: %#v", pendingList)
	}
	allList, err := svc.ListClientsForAdmin(ctx, adminActor, "all", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(allList) != 1 || allList[0]["client_id"] != clientID || allList[0]["status"] != oauth.StatusPending {
		t.Fatalf("all admin list mismatch: %#v", allList)
	}

	updated, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
		Name:            "Managed app updated",
		Description:     "Updated description",
		RedirectURI:     "https://managed.example/new-callback",
		WebsiteURL:      "https://managed.example/docs",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "Managed app updated" || updated["status"] != oauth.StatusPending ||
		updated["redirect_uri"] != "https://managed.example/new-callback" {
		t.Fatalf("owner update should preserve pending status and update fields: %#v", updated)
	}
	submitted, err := svc.SubmitClientForReview(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if submitted["status"] != oauth.StatusPending {
		t.Fatalf("submitted client should be pending: %#v", submitted)
	}
	if _, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusPending); !isHTTPError(err, 400, "invalid status") {
		t.Fatalf("review pending status error mismatch: %#v", err)
	}
	reviewed, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed["status"] != oauth.StatusActive || reviewed["client_id"] != clientID {
		t.Fatalf("reviewed client mismatch: %#v", reviewed)
	}
	rotated, err := svc.RotateClientSecret(ctx, ownerActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated["client_secret"] == "" || rotated["client_secret"] == firstSecret || rotated["status"] != oauth.StatusActive {
		t.Fatalf("rotated secret mismatch: %#v", rotated)
	}
	if err := svc.DeleteClient(ctx, otherActor, clientID); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("other delete error mismatch: %#v", err)
	}
	if err := svc.DeleteClient(ctx, ownerActor, clientID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetClient(ctx, ownerActor, clientID); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("deleted client get error mismatch: %#v", err)
	}
}

func TestServiceClientManagementRejectsUnauthorizedMissingAndInvalidStateExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-owner-reject@test.com", "Password123", "OAuthOwnerReject", false)
	admin := testutil.CreateUser(t, db, "oauth-admin-reject@test.com", "Password123", "OAuthAdminReject", true, true)
	ownerActor, err := db.Permissions.ActorForUser(ctx, owner.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)

	if _, err := svc.CreateClient(ctx, permission.Actor{}, oauth.ClientInput{}); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("create without permission mismatch: %#v", err)
	}
	if _, err := svc.ListClientsForAdmin(ctx, permission.Actor{}, "all", 10); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("admin list without permission mismatch: %#v", err)
	}
	if _, err := svc.GetClient(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("get missing client mismatch: %#v", err)
	}
	if _, err := svc.UpdateClient(ctx, ownerActor, "missing-client", oauth.ClientInput{
		Name:            "Missing",
		RedirectURI:     "https://missing.example/callback",
		PermissionCodes: []string{"account.read.self"},
	}, "active"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("update missing client mismatch: %#v", err)
	}
	if _, err := svc.SubmitClientForReview(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("submit missing client mismatch: %#v", err)
	}
	if _, err := svc.ReviewClient(ctx, permission.Actor{}, "missing-client", oauth.StatusActive); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("review without permission mismatch: %#v", err)
	}
	if _, err := svc.ReviewClient(ctx, adminActor, "missing-client", oauth.StatusActive); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("review missing client mismatch: %#v", err)
	}
	if _, err := svc.RotateClientSecret(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("rotate missing client mismatch: %#v", err)
	}
	if err := svc.DeleteClient(ctx, ownerActor, "missing-client"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("delete missing client mismatch: %#v", err)
	}
	if _, err := svc.ClientPermissions(ctx, adminActor, "missing-client"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("client permissions missing mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, permission.Actor{}, "missing-client", "account.read.self", "deny"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("set permission deny without revoke permission mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, "missing-client", "account.read.self", "allow"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("set permission missing client mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, "missing-client", "account.read.self"); !isHTTPError(err, 404, "oauth client not found") {
		t.Fatalf("clear permission missing client mismatch: %#v", err)
	}

	created, err := svc.CreateClient(ctx, ownerActor, oauth.ClientInput{
		Name:            "Reject state app",
		RedirectURI:     "https://reject-state.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	if _, err := svc.UpdateClient(ctx, adminActor, clientID, oauth.ClientInput{
		Name:            "Reject state app updated",
		RedirectURI:     "https://reject-state.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	}, "archived"); !isHTTPError(err, 400, "invalid status") {
		t.Fatalf("update invalid status mismatch: %#v", err)
	}
}

func TestServicePublicClientSecretAndInputValidationPathsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-inputs@test.com", "Password123", "OAuthInputs", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	cases := []struct {
		name   string
		input  oauth.ClientInput
		status int
		detail string
	}{
		{name: "empty name", input: oauth.ClientInput{Name: "", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "invalid name"},
		{name: "bad redirect", input: oauth.ClientInput{Name: "Bad redirect", RedirectURI: "ftp://app.example/callback", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "invalid redirect_uri"},
		{name: "bad website", input: oauth.ClientInput{Name: "Bad website", RedirectURI: "https://app.example/callback", WebsiteURL: "://bad", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "invalid website_url"},
		{name: "bad type", input: oauth.ClientInput{Name: "Bad type", RedirectURI: "https://app.example/callback", ClientType: "native", PermissionCodes: []string{"account.read.self"}}, status: 400, detail: "invalid client_type"},
		{name: "bad scope", input: oauth.ClientInput{Name: "Bad scope", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"permission.catalog.system"}}, status: 400, detail: "invalid scope"},
		{name: "missing actor scope", input: oauth.ClientInput{Name: "Missing actor scope", RedirectURI: "https://app.example/callback", PermissionCodes: []string{"account.ban.any"}}, status: 403, detail: "permission denied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.CreateClient(ctx, actor, tc.input)
			assertHTTPError(t, err, tc.status, tc.detail)
		})
	}
	publicClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Public no secret",
		RedirectURI:     "https://public.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := publicClient["client_id"].(string)
	if publicClient["client_secret"] != nil {
		t.Fatalf("public client should not expose a secret: %#v", publicClient)
	}
	if _, err := svc.RotateClientSecret(ctx, actor, clientID); !isHTTPError(err, 400, "public clients do not have secrets") {
		t.Fatalf("rotate public secret error mismatch: %#v", err)
	}
}

func TestServiceClientPermissionAndGrantManagementExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-client-perms@test.com", "Password123", "OAuthClientPerms", false)
	admin := testutil.CreateUser(t, db, "oauth-client-perms-admin@test.com", "Password123", "OAuthClientPermsAdmin", true, true)
	userActor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	adminActor, err := db.Permissions.ActorForUser(ctx, admin.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	created, err := svc.CreateClient(ctx, userActor, oauth.ClientInput{
		Name:            "Permission app",
		RedirectURI:     "https://perms.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	if _, err := svc.ClientPermissions(ctx, userActor, clientID); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permissions without admin read error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server", "allow"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permission set without grant error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "permission.catalog.system", "allow"); !isHTTPError(err, 400, "invalid permission") {
		t.Fatalf("client system permission set error mismatch: %#v", err)
	}
	if err := svc.SetClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server", "allow"); err != nil {
		t.Fatal(err)
	}
	perms, err := svc.ClientPermissions(ctx, adminActor, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if perms["subject_id"] != permissiondb.SubjectIDForClient(clientID) {
		t.Fatalf("client permission subject mismatch: %#v", perms)
	}
	effective := stringSetFromStrings(perms["effective_permissions"].([]string))
	if !effective["minecraft_session.hasjoined.server"] {
		t.Fatalf("effective client permissions missing override: %#v", perms)
	}
	overrides := perms["overrides"].([]map[string]any)
	if len(overrides) != 1 || overrides[0]["permission_code"] != "minecraft_session.hasjoined.server" || overrides[0]["effect"] != "allow" {
		t.Fatalf("client overrides mismatch: %#v", overrides)
	}
	if err := svc.ClearClientPermissionOverride(ctx, userActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("client permission clear without revoke error mismatch: %#v", err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ClearClientPermissionOverride(ctx, adminActor, clientID, "minecraft_session.hasjoined.server"); !isHTTPError(err, 404, "permission override not found") {
		t.Fatalf("client permission clear missing error mismatch: %#v", err)
	}

	activateOAuthClient(t, db, clientID)
	verifier := "grant-verifier-abcdefghijklmnopqrstuvwxyz"
	approved, err := svc.ApproveAuthorization(ctx, userActor, oauth.AuthorizationRequest{
		ResponseType:        "code",
		ClientID:            clientID,
		RedirectURI:         "https://perms.example/callback",
		Scope:               "account.read.self",
		CodeChallenge:       pkceChallenge(verifier),
		CodeChallengeMethod: "S256",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:    "authorization_code",
		ClientID:     clientID,
		ClientSecret: created["client_secret"].(string),
		Code:         approved["code"].(string),
		RedirectURI:  "https://perms.example/callback",
		CodeVerifier: verifier,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.RefreshToken == "" {
		t.Fatalf("token should include refresh token: %#v", token)
	}
	grants, err := svc.ListGrants(ctx, userActor, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 1 || grants[0]["client_id"] != clientID || grants[0]["status"] != "active" {
		t.Fatalf("grant list mismatch: %#v", grants)
	}
	grantPermissions := grants[0]["permissions"].([]string)
	if len(grantPermissions) != 1 || grantPermissions[0] != "account.read.self" {
		t.Fatalf("grant permissions mismatch: %#v", grantPermissions)
	}
	if err := svc.RevokeGrant(ctx, userActor, grants[0]["id"].(string)); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeGrant(ctx, userActor, grants[0]["id"].(string)); !isHTTPError(err, 404, "oauth grant not found") {
		t.Fatalf("second grant revoke error mismatch: %#v", err)
	}
	if _, err := svc.ListGrants(ctx, permission.Actor{}, 10); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("list grants without permission mismatch: %#v", err)
	}
	if err := svc.RevokeGrant(ctx, permission.Actor{}, grants[0]["id"].(string)); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("revoke grant without permission mismatch: %#v", err)
	}
}

func TestServiceDeviceCodeFlowIssuesDelegatedTokenExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-flow@test.com", "Password123", "OAuthDeviceFlow", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Device app",
		RedirectURI:     "https://device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.read.self",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.DeviceCode == "" || started.UserCode == "" || started.ExpiresIn != 600 || started.Interval != 5 ||
		started.Scope != "account.read.self" || len(started.Permissions) != 1 || started.Permissions[0] != "account.read.self" {
		t.Fatalf("device authorization response mismatch: %#v", started)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "authorization_pending")

	details, err := svc.DeviceAuthorizationDetails(ctx, actor, started.UserCode)
	if err != nil {
		t.Fatal(err)
	}
	if details.Status != "pending" || details.Client["client_id"] != clientID || len(details.Scopes) != 1 {
		t.Fatalf("device details mismatch: %#v", details)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: true}); err != nil {
		t.Fatal(err)
	}
	token, err := svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if token.AccessToken == "" || token.RefreshToken == "" || token.Scope != "account.read.self" ||
		len(token.Permissions) != 1 || token.Permissions[0] != "account.read.self" {
		t.Fatalf("device token response mismatch: %#v", token)
	}
	delegated, ok, err := svc.ActorForBearer(ctx, token.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || delegated.UserID != user.ID || !delegated.Has(permission.MustDefinitionByCode("account.read.self")) {
		t.Fatalf("device delegated actor mismatch: ok=%v actor=%#v", ok, delegated)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "invalid_grant")
}

func TestServiceDeviceCodeFlowRejectsDeniedAndUnauthorizedScopesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-deny@test.com", "Password123", "OAuthDeviceDeny", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Denied device app",
		RedirectURI:     "https://device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	otherClient, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Other device app",
		RedirectURI:     "https://other-device.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherClientID := otherClient["client_id"].(string)
	activateOAuthClient(t, db, otherClientID)
	_, err = svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.update.self",
	})
	assertHTTPError(t, err, 400, "scope exceeds client permission limit")
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
		ClientID: clientID,
		Scope:    "account.read.self",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   otherClientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "invalid device_code")
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: false}); err != nil {
		t.Fatal(err)
	}
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: started.DeviceCode,
	})
	assertHTTPError(t, err, 400, "access_denied")
}

func TestServiceDeviceAuthorizationRejectsMissingExpiredAndRepeatedDecisionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-device-errors@test.com", "Password123", "OAuthDeviceErrors", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := newOAuthService(db)
	clientRes, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Device errors app",
		RedirectURI:     "https://device-errors.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	activateOAuthClient(t, db, clientID)
	started, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{ClientID: clientID, Scope: "account.read.self"})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := svc.DeviceAuthorizationDetails(ctx, permission.Actor{}, started.UserCode); !isHTTPError(err, 403, "permission denied") {
		t.Fatalf("anonymous device detail error mismatch: %#v", err)
	}
	if _, err := svc.DeviceAuthorizationDetails(ctx, actor, "missing-code"); !isHTTPError(err, 404, "device code not found") {
		t.Fatalf("missing device detail error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: "missing-code", Approve: true}); !isHTTPError(err, 404, "device code not found") {
		t.Fatalf("missing device decision error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: true}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: started.UserCode, Approve: false}); !isHTTPError(err, 400, "device code is not pending") {
		t.Fatalf("repeated device decision error mismatch: %#v", err)
	}

	expired := modelDeviceCode(t, db, clientID, "expired-device-hash", "expired-user-code", []string{"account.read.self"}, -time.Minute)
	if _, err := svc.DeviceAuthorizationDetails(ctx, actor, expired); !isHTTPError(err, 404, "device code not found") {
		t.Fatalf("expired device detail error mismatch: %#v", err)
	}
	if err := svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: expired, Approve: true}); !isHTTPError(err, 404, "device code not found") {
		t.Fatalf("expired device decision error mismatch: %#v", err)
	}

	expiredDeviceToken := "expired-device-token"
	modelDeviceCode(t, db, clientID, util.HashRefreshToken(expiredDeviceToken), "expired-token-code", []string{"account.read.self"}, -time.Minute)
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   clientID,
		DeviceCode: expiredDeviceToken,
	})
	assertHTTPError(t, err, 400, "expired_token")

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
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "authorization_code", ClientID: clientID, ClientSecret: clientSecret, Code: "missing-code", RedirectURI: "https://token-errors.example/callback", CodeVerifier: "valid-verifier-abcdefghijklmnopqrstuvwxyz"})
	assertHTTPError(t, err, 400, "invalid authorization code")
	_, err = svc.IssueToken(ctx, oauth.TokenRequest{GrantType: "refresh_token", ClientID: clientID, ClientSecret: clientSecret, RefreshToken: "missing-refresh"})
	assertHTTPError(t, err, 400, "invalid refresh_token")
	for _, tc := range []struct {
		name        string
		redirectURI string
		verifier    string
		detail      string
	}{
		{name: "wrong redirect", redirectURI: "https://token-errors.example/wrong", verifier: "token-verifier-abcdefghijklmnopqrstuvwxyz", detail: "invalid authorization code"},
		{name: "wrong verifier", redirectURI: "https://token-errors.example/callback", verifier: "wrong-verifier-abcdefghijklmnopqrstuvwxyz", detail: "invalid code_verifier"},
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
				RedirectURI:  tc.redirectURI,
				CodeVerifier: tc.verifier,
			})
			assertHTTPError(t, err, 400, tc.detail)
			_, err = svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "authorization_code",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Code:         approved["code"].(string),
				RedirectURI:  "https://token-errors.example/callback",
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
		RedirectURI:  "https://token-errors.example/callback",
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

func TestServiceOAuthClosedDatabasePropagatesExactDependencyErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-db-closed@test.com", "Password123", "OAuthDBClosed", false)
	admin := testutil.CreateUser(t, db, "oauth-db-closed-admin@test.com", "Password123", "OAuthDBClosedAdmin", true, true)
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
		Name:            "Closed database app",
		RedirectURI:     "https://closed-db.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := clientRes["client_id"].(string)
	clientSecret := clientRes["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	rawUserAccess := "closed-db-user-access"
	if err := svc.Redis.SetOAuthAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawUserAccess),
		ClientID:      clientID,
		UserID:        user.ID,
		GrantID:       "closed-db-grant",
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("account.read.self").ID)},
		ExpiresAt:     database.NowMS() + int64(time.Hour/time.Millisecond),
		CreatedAt:     database.NowMS(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}
	rawClientAccess := "closed-db-client-access"
	if err := svc.Redis.SetOAuthAccessToken(ctx, redisstore.OAuthAccessToken{
		TokenHash:     util.HashRefreshToken(rawClientAccess),
		ClientID:      clientID,
		PermissionIDs: []int64{int64(permission.MustDefinitionByCode("minecraft_session.hasjoined.server").ID)},
		ExpiresAt:     database.NowMS() + int64(time.Hour/time.Millisecond),
		CreatedAt:     database.NowMS(),
	}, time.Hour); err != nil {
		t.Fatal(err)
	}

	db.Close()

	checks := []struct {
		name string
		call func() error
	}{
		{name: "create client", call: func() error {
			_, err := svc.CreateClient(ctx, actor, oauth.ClientInput{
				Name:            "Closed database create",
				RedirectURI:     "https://closed-db.example/create",
				ClientType:      oauth.ClientTypePublic,
				PermissionCodes: []string{"account.read.self"},
			})
			return err
		}},
		{name: "list owned clients", call: func() error {
			_, err := svc.ListClients(ctx, actor, 10)
			return err
		}},
		{name: "list admin clients", call: func() error {
			_, err := svc.ListClientsForAdmin(ctx, adminActor, oauth.StatusActive, 10)
			return err
		}},
		{name: "get client", call: func() error {
			_, err := svc.GetClient(ctx, actor, clientID)
			return err
		}},
		{name: "update client", call: func() error {
			_, err := svc.UpdateClient(ctx, actor, clientID, oauth.ClientInput{
				Name:            "Closed database update",
				RedirectURI:     "https://closed-db.example/update",
				ClientType:      oauth.ClientTypePublic,
				PermissionCodes: []string{"account.read.self"},
			}, "")
			return err
		}},
		{name: "submit client", call: func() error {
			_, err := svc.SubmitClientForReview(ctx, actor, clientID)
			return err
		}},
		{name: "review client", call: func() error {
			_, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive)
			return err
		}},
		{name: "rotate secret", call: func() error {
			_, err := svc.RotateClientSecret(ctx, actor, clientID)
			return err
		}},
		{name: "delete client", call: func() error {
			return svc.DeleteClient(ctx, actor, clientID)
		}},
		{name: "client permissions", call: func() error {
			_, err := svc.ClientPermissions(ctx, adminActor, clientID)
			return err
		}},
		{name: "list grants", call: func() error {
			_, err := svc.ListGrants(ctx, actor, 10)
			return err
		}},
		{name: "authorization details", call: func() error {
			_, err := svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://closed-db.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge("closed-db-verifier"),
				CodeChallengeMethod: "S256",
			})
			return err
		}},
		{name: "start device authorization", call: func() error {
			_, err := svc.StartDeviceAuthorization(ctx, oauth.DeviceAuthorizationRequest{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Scope:        "account.read.self",
			})
			return err
		}},
		{name: "device details", call: func() error {
			_, err := svc.DeviceAuthorizationDetails(ctx, actor, "ABCD-EFGH")
			return err
		}},
		{name: "device decision", call: func() error {
			return svc.DecideDeviceAuthorization(ctx, actor, oauth.DeviceDecisionRequest{UserCode: "ABCD-EFGH", Approve: true})
		}},
		{name: "revoke token", call: func() error {
			return svc.RevokeToken(ctx, clientID, clientSecret, "any-token")
		}},
		{name: "client credentials token", call: func() error {
			_, err := svc.IssueToken(ctx, oauth.TokenRequest{
				GrantType:    "client_credentials",
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Scope:        "minecraft_session.hasjoined.server",
			})
			return err
		}},
		{name: "user bearer actor", call: func() error {
			_, ok, err := svc.ActorForBearer(ctx, rawUserAccess)
			if ok {
				t.Fatal("closed database must not authenticate user bearer")
			}
			return err
		}},
		{name: "client bearer actor", call: func() error {
			_, ok, err := svc.ActorForBearer(ctx, rawClientAccess)
			if ok {
				t.Fatal("closed database must not authenticate client bearer")
			}
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			assertClosedPoolError(t, tc.call())
		})
	}
}

func TestServiceOAuthPermissionCodeDependencyErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	owner := testutil.CreateUser(t, db, "oauth-code-dependency@test.com", "Password123", "OAuthCodeDependency", false)
	admin := testutil.CreateUser(t, db, "oauth-code-dependency-admin@test.com", "Password123", "OAuthCodeDependencyAdmin", true, true)
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
		Name:            "Permission code dependency app",
		RedirectURI:     "https://code-dependency.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	if ok, err := db.OAuth.UpdateClientStatus(ctx, clientID, oauth.StatusActive, database.NowMS()); err != nil || !ok {
		t.Fatalf("activate client before dependency drop: ok=%v err=%v", ok, err)
	}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE delegated_client_permissions CASCADE`); err != nil {
		t.Fatal(err)
	}

	checks := []struct {
		name string
		call func() error
	}{
		{name: "get client", call: func() error {
			_, err := svc.GetClient(ctx, ownerActor, clientID)
			return err
		}},
		{name: "list owned clients", call: func() error {
			_, err := svc.ListClients(ctx, ownerActor, 10)
			return err
		}},
		{name: "list admin clients", call: func() error {
			_, err := svc.ListClientsForAdmin(ctx, adminActor, "all", 10)
			return err
		}},
		{name: "update client", call: func() error {
			_, err := svc.UpdateClient(ctx, ownerActor, clientID, oauth.ClientInput{
				Name:            "Permission code dependency app updated",
				RedirectURI:     "https://code-dependency.example/updated",
				ClientType:      oauth.ClientTypeConfidential,
				PermissionCodes: []string{"account.read.self"},
			}, oauth.StatusActive)
			return err
		}},
		{name: "submit client", call: func() error {
			_, err := svc.SubmitClientForReview(ctx, ownerActor, clientID)
			return err
		}},
		{name: "review client", call: func() error {
			_, err := svc.ReviewClient(ctx, adminActor, clientID, oauth.StatusActive)
			return err
		}},
		{name: "rotate secret", call: func() error {
			_, err := svc.RotateClientSecret(ctx, ownerActor, clientID)
			return err
		}},
		{name: "client permissions", call: func() error {
			_, err := svc.ClientPermissions(ctx, adminActor, clientID)
			return err
		}},
		{name: "authorization details", call: func() error {
			_, err := svc.AuthorizationDetails(ctx, ownerActor, oauth.AuthorizationRequest{
				ResponseType:        "code",
				ClientID:            clientID,
				RedirectURI:         "https://code-dependency.example/callback",
				Scope:               "account.read.self",
				CodeChallenge:       pkceChallenge("code-dependency-verifier"),
				CodeChallengeMethod: "S256",
			})
			return err
		}},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			assertPgCode(t, tc.call(), "42P01")
		})
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

func modelDeviceCode(t *testing.T, db *database.DB, clientID, deviceHash, rawUserCode string, codes []string, ttlOffset time.Duration) string {
	t.Helper()
	now := database.NowMS()
	userCode := strings.ToUpper(rawUserCode)
	record := model.OAuthDeviceCode{
		DeviceCodeHash: deviceHash,
		UserCodeHash:   util.HashRefreshToken(userCode),
		ClientID:       clientID,
		Status:         "pending",
		ExpiresAt:      now + int64(ttlOffset/time.Millisecond),
		CreatedAt:      now,
	}
	if err := db.OAuth.CreateDeviceCode(context.Background(), record, permissionIDsFromCodesForTest(codes)); err != nil {
		t.Fatal(err)
	}
	return userCode
}

func permissionIDsFromCodesForTest(codes []string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	return ids
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func withAuthReq(req oauth.AuthorizationRequest, mutate func(*oauth.AuthorizationRequest)) oauth.AuthorizationRequest {
	mutate(&req)
	return req
}

func newOAuthService(db *database.DB) oauth.Service {
	return oauth.Service{DB: db, Redis: redisstore.NewMemoryStore()}
}

func grantClientPermission(t *testing.T, db *database.DB, clientID, code string) {
	t.Helper()
	def := permission.MustDefinitionByCode(code)
	if err := db.Permissions.SetPermissionOverrideForSubject(context.Background(), permissiondb.SubjectIDForClient(clientID), def, "allow", ""); err != nil {
		t.Fatal(err)
	}
}

func activateOAuthClient(t *testing.T, db *database.DB, clientID string) {
	t.Helper()
	if ok, err := db.OAuth.UpdateClientStatus(context.Background(), clientID, oauth.StatusActive, database.NowMS()); err != nil || !ok {
		t.Fatalf("activate oauth client: ok=%v err=%v", ok, err)
	}
}

func assertHTTPError(t *testing.T, err error, status int, detail string) {
	t.Helper()
	if !isHTTPError(err, status, detail) {
		t.Fatalf("HTTP error mismatch: err=%#v want status=%d detail=%q", err, status, detail)
	}
}

func assertClosedPoolError(t *testing.T, err error) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("closed database error mismatch: %v", err)
	}
}

func assertPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func isHTTPError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}

func stringSetFromStrings(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}
