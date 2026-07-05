package yggdrasil_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestYggdrasilRedisDependencyErrorsArePropagatedExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-redis-fail@test.com", "Password123", "YggRedisFail", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_redis_fail_profile", "YggRedisFailProfile")
	forced := errors.New("forced redis ygg failure")

	failingSet := redisstore.NewMemoryStore()
	failingSet.Err = forced
	yggSet := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: failingSet}
	if response, err := yggSet.Authenticate(ctx, user.Email, "Password123", "client", false); response != nil || !errors.Is(err, forced) {
		t.Fatalf("Authenticate redis failure response=%#v err=%v; want nil and forced error", response, err)
	}

	token := model.Token{
		AccessToken: "redis_fail_access",
		ClientToken: "redis_fail_client",
		UserID:      user.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   database.NowMS(),
	}
	for _, tc := range []struct {
		name string
		call func(yggdrasil.Yggdrasil) error
	}{
		{name: "refresh", call: func(y yggdrasil.Yggdrasil) error {
			_, err := y.Refresh(ctx, token.AccessToken, token.ClientToken, "", false)
			return err
		}},
		{name: "validate", call: func(y yggdrasil.Yggdrasil) error {
			return y.Validate(ctx, token.AccessToken, token.ClientToken)
		}},
		{name: "token", call: func(y yggdrasil.Yggdrasil) error {
			got, err := y.Token(ctx, token.AccessToken)
			if got != (model.Token{}) {
				t.Fatalf("Token redis failure returned token=%#v, want zero token", got)
			}
			return err
		}},
		{name: "invalidate", call: func(y yggdrasil.Yggdrasil) error {
			return y.Invalidate(ctx, token.AccessToken)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cache := redisstore.NewMemoryStore()
			if err := cache.SetYggToken(ctx, token, time.Minute); err != nil {
				t.Fatal(err)
			}
			cache.Err = forced
			ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}
			if err := tc.call(ygg); !errors.Is(err, forced) {
				t.Fatalf("%s redis error=%v; want forced error", tc.name, err)
			}
		})
	}
}

func TestYggdrasilMissingRedisTokensReturnExactProtocolErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: testutil.NewMemoryRedis()}

	if response, err := ygg.Refresh(ctx, "missing_refresh_access", "client", "", false); response != nil || !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("missing refresh response=%#v err=%v; want nil and exact invalid-token ygg error", response, err)
	}
	if err := ygg.Invalidate(ctx, "missing_invalidate_access"); err != nil {
		t.Fatalf("missing invalidate token should be a no-op, got %v", err)
	}
}

func TestYggdrasilRefreshPropagatesReplaceFailureAndKeepsOldToken(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-replace-fail@test.com", "Password123", "YggReplaceFail", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_replace_fail_profile", "YggReplaceFailProfile")
	forced := errors.New("replace token failed")
	old := model.Token{
		AccessToken: "replace_fail_old_access",
		ClientToken: "replace_fail_client",
		UserID:      user.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   database.NowMS(),
	}
	cache := &replaceFailStore{Store: testutil.NewMemoryRedis(), err: forced}
	if err := cache.Store.SetYggToken(ctx, old, time.Minute); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

	response, err := ygg.Refresh(ctx, old.AccessToken, old.ClientToken, "", false)
	if response != nil || !errors.Is(err, forced) {
		t.Fatalf("replace failure response=%#v err=%v; want nil and forced error", response, err)
	}
	if cache.replaceCalls != 1 || cache.oldAccess != old.AccessToken || cache.nextToken.AccessToken == "" || cache.nextToken.AccessToken == old.AccessToken {
		t.Fatalf("replace call mismatch: calls=%d old=%q next=%#v", cache.replaceCalls, cache.oldAccess, cache.nextToken)
	}
	got, err := cache.Store.GetYggToken(ctx, old.AccessToken)
	if err != nil || !sameToken(got, old) {
		t.Fatalf("replace failure must preserve old token: got=%#v err=%v want=%#v", got, err, old)
	}
}

func TestYggdrasilSignoutInvalidateAndTokenLimitUseRedisOnly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-signout@test.com", "Password123", "YggSignout", false)
	testutil.CreateProfile(t, db, user.ID, "ygg_signout_profile", "YggSignoutProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	var accesses []string
	for i := 0; i < 6; i++ {
		auth, err := ygg.Authenticate(ctx, user.Email, "Password123", "client", false)
		if err != nil {
			t.Fatal(err)
		}
		accesses = append(accesses, auth["accessToken"].(string))
	}
	if _, err := redis.GetYggToken(ctx, accesses[0]); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("oldest token should be trimmed from redis, got %v", err)
	}
	for _, access := range accesses[1:] {
		if _, err := redis.GetYggToken(ctx, access); err != nil {
			t.Fatalf("newer token %q should remain in redis: %v", access, err)
		}
	}

	if err := ygg.Invalidate(ctx, accesses[1]); err != nil {
		t.Fatal(err)
	}
	if _, err := redis.GetYggToken(ctx, accesses[1]); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidate should delete one redis token, got %v", err)
	}
	if err := ygg.Signout(ctx, user.Email, "Password123"); err != nil {
		t.Fatal(err)
	}
	for _, access := range accesses[2:] {
		if _, err := redis.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("signout should delete all remaining redis tokens, %q got %v", access, err)
		}
	}
}

func TestYggdrasilTokenReadsRedisOnly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-token@test.com", "Password123", "YggToken", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_token_profile", "YggTokenProfile")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

	token := model.Token{AccessToken: "redis_access", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: database.NowMS()}
	if err := redis.SetYggToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := ygg.Token(ctx, token.AccessToken)
	if err != nil || got.AccessToken != token.AccessToken || got.UserID != user.ID || got.ProfileID == nil || *got.ProfileID != profile.ID {
		t.Fatalf("Token should read redis token: %#v err=%v", got, err)
	}
	if _, err := ygg.Token(ctx, "missing_access"); !yggError(err, 401, "Unauthorized", "Invalid token") {
		t.Fatalf("missing redis token should be unauthorized ygg error, got %v", err)
	}
}

func TestYggdrasilAuthenticateRevokesNewTokenWhenLimitTrimFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-trim-fail@test.com", "Password123", "YggTrimFail", false)
	testutil.CreateProfile(t, db, user.ID, "ygg_trim_fail_profile", "YggTrimFailProfile")
	cache := &trimFailStore{Store: testutil.NewMemoryRedis()}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

	response, err := ygg.Authenticate(ctx, user.Email, "Password123", "trim-fail-client", false)
	if response != nil || err == nil || err.Error() != "token limit trim failed" {
		t.Fatalf("trim failure response=%#v err=%v, want nil and exact dependency error", response, err)
	}
	if cache.setCalls != 1 || cache.trimCalls != 1 || cache.lastToken.AccessToken == "" || cache.lastToken.UserID != user.ID {
		t.Fatalf("token operations mismatch: set=%d trim=%d token=%#v", cache.setCalls, cache.trimCalls, cache.lastToken)
	}
	if _, err := cache.Store.GetYggToken(ctx, cache.lastToken.AccessToken); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("trim failure must revoke newly-created token, got %v", err)
	}
}

func TestYggdrasilRefreshPreservesOldTokenWhenResponseUserLookupFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-refresh-user-fail@test.com", "Password123", "YggRefreshUserFail", false)
	cache := testutil.NewMemoryRedis()
	old := model.Token{
		AccessToken: "refresh_user_lookup_old_access",
		ClientToken: "refresh_user_lookup_client",
		UserID:      user.ID,
		CreatedAt:   database.NowMS(),
	}
	if err := cache.SetYggToken(ctx, old, time.Minute); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `ALTER TABLE users RENAME TO users_unavailable`); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}
	response, err := ygg.Refresh(ctx, old.AccessToken, old.ClientToken, "", true)
	var pgErr *pgconn.PgError
	if response != nil || !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
		t.Fatalf("Refresh response=%#v err=%#v; want nil and PostgreSQL 42P01", response, err)
	}
	got, err := cache.GetYggToken(ctx, old.AccessToken)
	if err != nil ||
		got.AccessToken != old.AccessToken ||
		got.ClientToken != old.ClientToken ||
		got.UserID != old.UserID ||
		got.ProfileID != nil ||
		got.CreatedAt != old.CreatedAt {
		t.Fatalf("failed refresh changed old token: token=%#v err=%v want=%#v", got, err, old)
	}
}
