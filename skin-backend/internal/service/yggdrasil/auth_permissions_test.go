package yggdrasil_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilSessionPermissionsRejectExactOperations(t *testing.T) {
	for _, tc := range []struct {
		name       string
		permission string
		call       func(context.Context, yggdrasil.Yggdrasil, *testing.T, string, string) error
	}{
		{
			name:       "authenticate",
			permission: "yggdrasil_session.create.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				_, err := ygg.Authenticate(ctx, email, password, "client", false)
				return err
			},
		},
		{
			name:       "refresh",
			permission: "yggdrasil_session.refresh.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				_, err := ygg.Refresh(ctx, auth["accessToken"].(string), auth["clientToken"].(string), "", false)
				return err
			},
		},
		{
			name:       "validate",
			permission: "yggdrasil_session.validate.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				return ygg.Validate(ctx, auth["accessToken"].(string), auth["clientToken"].(string))
			},
		},
		{
			name:       "invalidate",
			permission: "yggdrasil_session.invalidate.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				auth := mustYggAuth(t, ctx, ygg, email, password)
				access := auth["accessToken"].(string)
				err := ygg.Invalidate(ctx, access)
				if _, tokenErr := ygg.Redis.GetYggToken(ctx, access); tokenErr != nil {
					t.Fatalf("denied invalidate must keep the token: %v", tokenErr)
				}
				return err
			},
		},
		{
			name:       "signout",
			permission: "yggdrasil_session.signout.owned",
			call: func(ctx context.Context, ygg yggdrasil.Yggdrasil, t *testing.T, email, password string) error {
				mustYggAuth(t, ctx, ygg, email, password)
				return ygg.Signout(ctx, email, password)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			ctx := context.Background()
			user := testutil.CreateUser(t, db, "ygg-"+tc.name+"-deny@test.com", "Password123", "YggPermissionDeny", false)
			testutil.CreateProfile(t, db, user.ID, "ygg_"+tc.name+"_deny_profile", "YggDeny"+tc.name)
			redis := testutil.NewMemoryRedis()
			ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig(), Redis: redis}

			def := permission.MustDefinitionByCode(tc.permission)
			if tc.name != "authenticate" {
				auth := mustYggAuth(t, ctx, ygg, user.Email, "Password123")
				if err := redis.DeleteYggToken(ctx, auth["accessToken"].(string)); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "deny", ""); err != nil {
				t.Fatal(err)
			}
			err := tc.call(ctx, ygg, t, user.Email, "Password123")
			if !yggError(err, 403, "ForbiddenOperationException", "Permission denied.") {
				t.Fatalf("%s without %s should be denied with exact ygg permission error, got %v", tc.name, tc.permission, err)
			}
		})
	}
}
