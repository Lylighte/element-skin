package auth_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	authsvc "element-skin/backend/internal/service/auth"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func TestActorForWebAccessTokenRejectsInvalidAndMissingUsersExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	svc := newAuthService(db, cfg)

	actor, authenticated, err := svc.ActorForWebAccessToken(t.Context(), "not-a-jwt")
	if err != nil || authenticated || actor.UserID != "" || !actor.Permissions.Empty() {
		t.Fatalf("invalid token should be rejected exactly: actor=%#v authenticated=%v err=%v", actor, authenticated, err)
	}

	token, err := util.CreateAccessToken(cfg.JWTSecret, "missing-user", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	actor, authenticated, err = svc.ActorForWebAccessToken(t.Context(), token)
	if err != nil || authenticated || actor.UserID != "" || !actor.Permissions.Empty() {
		t.Fatalf("missing user token should be rejected exactly: actor=%#v authenticated=%v err=%v", actor, authenticated, err)
	}
	if _, err := svc.Redis.GetAuthUser(t.Context(), "missing-user"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("missing user must not be cached, got err=%v", err)
	}
}

func TestActorForWebAccessTokenCachesIdentityAndRecomputesPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	redis := testutil.NewMemoryRedis()
	svc := authsvc.Service{
		DB:       db,
		Cfg:      cfg,
		Redis:    redis,
		Settings: settingssvc.Settings{DB: db, Redis: redis},
	}
	user := testutil.CreateUser(t, db, "actor-web@test.com", "Password123", "ActorWeb", false)
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	actor, authenticated, err := svc.ActorForWebAccessToken(t.Context(), token)
	if err != nil || !authenticated {
		t.Fatalf("cold web actor mismatch: actor=%#v authenticated=%v err=%v", actor, authenticated, err)
	}
	adminPermission := permission.MustDefinitionByCode("user.read.any")
	if actor.UserID != user.ID ||
		actor.SubjectID != "user:"+user.ID ||
		actor.SessionKind != permission.SessionKindWeb ||
		actor.Entrypoint != permission.EntrypointDashboard ||
		actor.Has(adminPermission) {
		t.Fatalf("cold actor should be user dashboard actor without admin permission: %#v", actor)
	}
	cached, err := redis.GetAuthUser(t.Context(), user.ID)
	if err != nil || cached.ID != user.ID || cached.BannedUntil != nil {
		t.Fatalf("cold actor should cache exact auth identity: cached=%#v err=%v", cached, err)
	}

	if err := db.Permissions.GrantRole(t.Context(), user.ID, permission.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	actor, authenticated, err = svc.ActorForWebAccessToken(t.Context(), token)
	if err != nil || !authenticated {
		t.Fatalf("cached web actor mismatch: actor=%#v authenticated=%v err=%v", actor, authenticated, err)
	}
	if !actor.Has(adminPermission) {
		t.Fatalf("cached auth identity must still recompute current permissions: %#v", actor.PermissionCodes())
	}
}

func TestActorForWebAccessTokenFailsClosedWhenCachePopulationFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := &setAuthUserFailStore{Store: testutil.NewMemoryRedis()}
	svc := authsvc.Service{
		DB:       db,
		Cfg:      cfg,
		Redis:    cache,
		Settings: settingssvc.Settings{DB: db, Redis: cache},
	}
	user := testutil.CreateUser(t, db, "actor-cache-fail@test.com", "Password123", "ActorCacheFail", false)
	token, err := util.CreateAccessToken(cfg.JWTSecret, user.ID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	actor, authenticated, err := svc.ActorForWebAccessToken(t.Context(), token)
	if err == nil || err.Error() != "cache write failed" || authenticated || actor.UserID != "" {
		t.Fatalf("cache write failure mismatch: actor=%#v authenticated=%v err=%v", actor, authenticated, err)
	}
	if cache.setCalls != 1 {
		t.Fatalf("cache population should be attempted exactly once, calls=%d", cache.setCalls)
	}
	if _, err := cache.Store.GetAuthUser(t.Context(), user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("failed cache population must not leave cached auth user, got err=%v", err)
	}
}

type setAuthUserFailStore struct {
	redisstore.Store
	setCalls int
}

func (s *setAuthUserFailStore) SetAuthUser(context.Context, redisstore.AuthUser, time.Duration) error {
	s.setCalls++
	return errors.New("cache write failed")
}
