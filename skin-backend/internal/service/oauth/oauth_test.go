package oauth_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestServiceAuthorizationCodeFlowNarrowsActorExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-service@test.com", "Password123", "OAuthService", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := oauth.Service{DB: db}

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
}

func TestServiceRejectsInvalidAuthorizationRequestExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-invalid@test.com", "Password123", "OAuthInvalid", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	svc := oauth.Service{DB: db}
	_, err = svc.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Invalid app",
		RedirectURI:     "https://client.example/callback",
		ClientType:      oauth.ClientTypePublic,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.AuthorizationDetails(ctx, actor, oauth.AuthorizationRequest{ResponseType: "token"})
	var httpErr util.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status != 400 || httpErr.Detail != "response_type must be code" {
		t.Fatalf("invalid response_type error mismatch: err=%#v", err)
	}
}

func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
