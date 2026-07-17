package oauth_test

import (
	"context"
	"errors"
	"testing"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

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

func TestServiceSecretRotationDoesNotChangeSecretWhenCredentialInvalidationFailsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "oauth-rotation-redis-fail@test.com", "Password123", "OAuthRotationRedisFail", false)
	actor, err := db.Permissions.ActorForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	healthy := newOAuthService(db)
	created, err := healthy.CreateClient(ctx, actor, oauth.ClientInput{
		Name:            "Rotation redis fail app",
		RedirectURI:     "https://rotation-redis-fail.example/callback",
		ClientType:      oauth.ClientTypeConfidential,
		PermissionCodes: []string{"account.read.self"},
	})
	if err != nil {
		t.Fatal(err)
	}
	clientID := created["client_id"].(string)
	originalSecret := created["client_secret"].(string)
	activateOAuthClient(t, db, clientID)
	forced := errors.New("oauth client access invalidation failed")
	failing := oauth.Service{
		DB: db,
		Redis: &oauthClientAccessDeleteFailStore{
			Store: healthy.Redis,
			err:   forced,
		},
	}
	if _, err := failing.RotateClientSecret(ctx, actor, clientID); !errors.Is(err, forced) {
		t.Fatalf("secret rotation invalidation error mismatch: got=%v want=%v", err, forced)
	}
	client, err := db.OAuth.GetClient(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if client == nil || client.SecretHash != util.HashRefreshToken(originalSecret) {
		t.Fatalf("failed secret rotation must preserve original secret hash: %#v", client)
	}
}
