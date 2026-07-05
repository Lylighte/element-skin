package yggdrasil_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilProfileStoreDependencyErrorsAreExact(t *testing.T) {
	t.Run("authenticate profile lookup after email credentials", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "ygg-profile-db-auth@test.com", "Password123", "YggProfileDBAuth", false)
		if _, err := db.Pool.Exec(ctx, `ALTER TABLE profiles RENAME TO profiles_unavailable`); err != nil {
			t.Fatal(err)
		}
		ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: testutil.NewMemoryRedis()}

		response, err := ygg.Authenticate(ctx, user.Email, "Password123", "client", false)
		assertPgCode(t, err, "42P01")
		if response != nil {
			t.Fatalf("profile lookup failure response=%#v; want nil", response)
		}
	})

	t.Run("authenticate username profile lookup", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		if _, err := db.Pool.Exec(ctx, `ALTER TABLE profiles RENAME TO profiles_unavailable`); err != nil {
			t.Fatal(err)
		}
		ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: testutil.NewMemoryRedis()}

		response, err := ygg.Authenticate(ctx, "missing_profile_name", "Password123", "client", false)
		assertPgCode(t, err, "42P01")
		if response != nil {
			t.Fatalf("username profile lookup failure response=%#v; want nil", response)
		}
	})

	t.Run("refresh bound token ownership", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "ygg-profile-db-refresh@test.com", "Password123", "YggProfileDBRefresh", false)
		profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_db_refresh", "YggProfileDBRefreshProfile")
		old := model.Token{
			AccessToken: "profile_db_refresh_access",
			ClientToken: "profile_db_refresh_client",
			UserID:      user.ID,
			ProfileID:   &profile.ID,
			CreatedAt:   database.NowMS(),
		}
		cache := testutil.NewMemoryRedis()
		if err := cache.SetYggToken(ctx, old, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `ALTER TABLE profiles RENAME TO profiles_unavailable`); err != nil {
			t.Fatal(err)
		}
		ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

		response, err := ygg.Refresh(ctx, old.AccessToken, old.ClientToken, "", false)
		assertPgCode(t, err, "42P01")
		if response != nil {
			t.Fatalf("refresh profile dependency failure response=%#v; want nil", response)
		}
		got, tokenErr := cache.GetYggToken(ctx, old.AccessToken)
		if tokenErr != nil || !sameToken(got, old) {
			t.Fatalf("profile dependency failure must preserve old token: got=%#v err=%v want=%#v", got, tokenErr, old)
		}
	})

	t.Run("validate bound token ownership", func(t *testing.T) {
		db, _ := testutil.NewTestApp(t)
		ctx := context.Background()
		user := testutil.CreateUser(t, db, "ygg-profile-db-validate@test.com", "Password123", "YggProfileDBValidate", false)
		profile := testutil.CreateProfile(t, db, user.ID, "ygg_profile_db_validate", "YggProfileDBValidateProfile")
		token := model.Token{
			AccessToken: "profile_db_validate_access",
			ClientToken: "profile_db_validate_client",
			UserID:      user.ID,
			ProfileID:   &profile.ID,
			CreatedAt:   database.NowMS(),
		}
		cache := testutil.NewMemoryRedis()
		if err := cache.SetYggToken(ctx, token, time.Minute); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Pool.Exec(ctx, `ALTER TABLE profiles RENAME TO profiles_unavailable`); err != nil {
			t.Fatal(err)
		}
		ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: cache}

		assertPgCode(t, ygg.Validate(ctx, token.AccessToken, token.ClientToken), "42P01")
		got, tokenErr := cache.GetYggToken(ctx, token.AccessToken)
		if tokenErr != nil || !sameToken(got, token) {
			t.Fatalf("validate profile dependency failure must preserve token: got=%#v err=%v want=%#v", got, tokenErr, token)
		}
	})
}

func TestYggdrasilRejectsBoundTokenAfterProfileIDIsReassigned(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	originalOwner := testutil.CreateUser(t, db, "ygg-stale-owner@test.com", "Password123", "YggStaleOwner", false)
	newOwner := testutil.CreateUser(t, db, "ygg-stale-new-owner@test.com", "Password123", "YggStaleNewOwner", false)
	profile := testutil.CreateProfile(t, db, originalOwner.ID, "ygg_reassigned_profile", "YggOriginalRole")
	redis := testutil.NewMemoryRedis()
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}
	token := model.Token{
		AccessToken: "stale_reassigned_access",
		ClientToken: "stale_reassigned_client",
		UserID:      originalOwner.ID,
		ProfileID:   &profile.ID,
		CreatedAt:   database.NowMS(),
	}
	if err := redis.SetYggToken(ctx, token, time.Minute); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.Profiles.DeleteCascade(ctx, profile.ID); err != nil || !ok {
		t.Fatalf("delete original profile: ok=%v err=%v", ok, err)
	}
	if err := db.Profiles.Create(ctx, model.Profile{
		ID:           profile.ID,
		UserID:       newOwner.ID,
		Name:         "YggReassignedRole",
		TextureModel: "default",
	}); err != nil {
		t.Fatal(err)
	}

	if err := ygg.Validate(ctx, token.AccessToken, token.ClientToken); !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("validate must reject a token whose profile ID now belongs to another user, got %v", err)
	}
	if _, err := ygg.Token(ctx, token.AccessToken); !yggError(err, 401, "Unauthorized", "Invalid token") {
		t.Fatalf("token lookup must reject reassigned profile ownership, got %v", err)
	}
	if _, err := ygg.Refresh(ctx, token.AccessToken, token.ClientToken, "", false); !yggError(err, 403, "ForbiddenOperationException", "Invalid token.") {
		t.Fatalf("refresh must reject reassigned profile ownership, got %v", err)
	}
}
