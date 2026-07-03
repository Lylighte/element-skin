package account_test

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountServiceMeReturnsCountsAndUpdateSelfPersistsExactFields(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "account-self@test.com", "Password123", "AccountSelf", false)
	actor := accountActor(t, db, user.ID)
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: user.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.UpdateSelf(ctx, actor, map[string]any{
		"email":              "updated-account@test.com",
		"display_name":       "UpdatedAccount",
		"preferred_language": "en_US",
		"avatar_hash":        "avatar_hash",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("UpdateSelf should invalidate auth cache exactly, got %v", err)
	}
	me, err := svc.Me(ctx, accountActor(t, db, user.ID))
	if err != nil {
		t.Fatal(err)
	}
	permissions := me["permissions"].([]string)
	if me["id"] != user.ID ||
		me["email"] != "updated-account@test.com" ||
		me["display_name"] != "UpdatedAccount" ||
		me["lang"] != "en_US" ||
		me["avatar_hash"].(*string) == nil ||
		*me["avatar_hash"].(*string) != "avatar_hash" ||
		!containsString(permissions, "account.read.self") ||
		me["profile_count"] != 0 ||
		me["texture_count"] != 0 {
		t.Fatalf("Me response mismatch: %#v", me)
	}
}

func TestAccountServiceMeReturnsDatabaseErrorsInsteadOfZeroCounts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		table string
	}{
		{name: "profile count", table: "profiles"},
		{name: "texture count", table: "user_textures"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, _ := testutil.NewTestAppTB(t)
			ctx := context.Background()
			svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
			user := testutil.CreateUser(t, db, tc.name+"@test.com", "Password123", "AccountMeFailure", false)
			if _, err := db.Pool.Exec(ctx, `ALTER TABLE `+tc.table+` RENAME TO unavailable_`+tc.table); err != nil {
				t.Fatal(err)
			}
			result, err := svc.Me(ctx, accountActor(t, db, user.ID))
			var pgErr *pgconn.PgError
			if result != nil || !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
				t.Fatalf("Me result=%#v err=%#v; want nil and PostgreSQL 42P01", result, err)
			}
		})
	}
}

func TestAccountServiceRejectsInvalidSelfUpdatesAndWrongPasswordExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "account-self-invalid@test.com", "Password123", "AccountSelfInvalid", false)
	other := testutil.CreateUser(t, db, "account-self-invalid-other@test.com", "Password123", "AccountSelfInvalidOther", false)
	actor := accountActor(t, db, user.ID)

	for _, tc := range []struct {
		name string
		body map[string]any
		want string
	}{
		{"invalid email", map[string]any{"email": "not-an-email"}, "Invalid email format"},
		{"duplicate display name", map[string]any{"display_name": other.DisplayName}, "Username already exists"},
		{"blank display name", map[string]any{"display_name": "   "}, "Username cannot be empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := svc.UpdateSelf(ctx, actor, tc.body)
			if !httpErrorIs(err, http.StatusBadRequest, tc.want) {
				t.Fatalf("UpdateSelf should reject %s exactly, got %#v", tc.name, err)
			}
		})
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || unchanged.Email != user.Email || unchanged.DisplayName != user.DisplayName {
		t.Fatalf("invalid updates should not mutate account: user=%#v err=%v", unchanged, err)
	}

	if err := svc.ChangePasswordSelf(ctx, actor, "WrongPassword", "NewPassword123"); !httpErrorIs(err, http.StatusForbidden, "旧密码错误") {
		t.Fatalf("wrong old password should reject exactly, got %#v", err)
	}
	afterWrongPassword, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || afterWrongPassword == nil || !util.VerifyPassword("Password123", afterWrongPassword.Password) {
		t.Fatalf("wrong old password should not change hash: user=%#v err=%v", afterWrongPassword, err)
	}

	missingPasswordActor := actorWithPermissions("missing-user", "account_password.update.self")
	if err := svc.ChangePasswordSelf(ctx, missingPasswordActor, "Password123", "NewPassword123"); !httpErrorIs(err, http.StatusNotFound, "用户不存在") {
		t.Fatalf("missing user password change should reject exactly, got %#v", err)
	}
	missingUpdateActor := actorWithPermissions("missing-user", "account.update.self")
	if err := svc.UpdateSelf(ctx, missingUpdateActor, map[string]any{"preferred_language": "en_US"}); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("missing user account update should reject exactly, got %#v", err)
	}
}

func TestAccountServiceSelfPermissionDenials(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	user := testutil.CreateUser(t, db, "account-self-perm@test.com", "Password123", "AccountSelfPerm", false)

	for _, tc := range []struct {
		name      string
		actorCode string
		call      func(permission.Actor) error
	}{
		{
			name:      "Me without account.read.self",
			actorCode: "account.update.self",
			call: func(a permission.Actor) error {
				result, err := svc.Me(ctx, a)
				if result != nil {
					t.Fatalf("Me denied call returned result=%#v", result)
				}
				return err
			},
		},
		{
			name:      "UpdateSelf without account.update.self",
			actorCode: "account.read.self",
			call: func(a permission.Actor) error {
				return svc.UpdateSelf(ctx, a, map[string]any{"preferred_language": "en_US"})
			},
		},
		{
			name:      "ChangePasswordSelf without account_password.update.self",
			actorCode: "account.update.self",
			call: func(a permission.Actor) error {
				return svc.ChangePasswordSelf(ctx, a, "Password123", "NewPassword123")
			},
		},
		{
			name:      "DeleteSelf without account.delete.self",
			actorCode: "account.read.self",
			call: func(a permission.Actor) error {
				return svc.DeleteSelf(ctx, a)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actor := actorWithPermissions(user.ID, tc.actorCode)
			if err := tc.call(actor); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
				t.Fatalf("expected 403 permission denied, got %#v", err)
			}
		})
	}
}

func TestConcurrentEmailUpdatesReturnExactBusinessConflict(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	first := testutil.CreateUser(t, db, "email-race-first@test.com", "Password123", "EmailRaceFirst", false)
	second := testutil.CreateUser(t, db, "email-race-second@test.com", "Password123", "EmailRaceSecond", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_user_email_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_user_email_update
		BEFORE UPDATE OF email ON users
		FOR EACH ROW EXECUTE FUNCTION delay_user_email_write();
	`); err != nil {
		t.Fatal(err)
	}

	const targetEmail = "email-race-target@test.com"
	results := runConcurrentSelfUpdates([]permission.Actor{
		accountActor(t, db, first.ID),
		accountActor(t, db, second.ID),
	}, func(actor permission.Actor) error {
		return svc.UpdateSelf(context.Background(), actor, map[string]any{"email": targetEmail})
	})
	assertOneSelfUpdateConflict(t, results, "Email already in use")
	var targetCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE email=$1),
			COUNT(*) FILTER (WHERE email IN ($2,$3))
		FROM users
		WHERE id = ANY($4)
	`, targetEmail, first.Email, second.Email, []string{first.ID, second.ID}).Scan(&targetCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent email state: target=%d original=%d; want 1 and 1", targetCount, originalCount)
	}
}

func TestConcurrentDisplayNameUpdatesKeepNameUnique(t *testing.T) {
	db, _ := testutil.NewTestAppWithMaxConnectionsTB(t, 8)
	ctx := context.Background()
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	first := testutil.CreateUser(t, db, "name-race-first@test.com", "Password123", "NameRaceFirst", false)
	second := testutil.CreateUser(t, db, "name-race-second@test.com", "Password123", "NameRaceSecond", false)
	if _, err := db.Pool.Exec(ctx, `
		CREATE FUNCTION delay_user_display_name_write() RETURNS trigger AS $$
		BEGIN
			PERFORM pg_sleep(0.2);
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		CREATE TRIGGER delay_user_display_name_update
		BEFORE UPDATE OF display_name ON users
		FOR EACH ROW EXECUTE FUNCTION delay_user_display_name_write();
	`); err != nil {
		t.Fatal(err)
	}

	const targetName = "SharedDisplayName"
	results := runConcurrentSelfUpdates([]permission.Actor{
		accountActor(t, db, first.ID),
		accountActor(t, db, second.ID),
	}, func(actor permission.Actor) error {
		return svc.UpdateSelf(context.Background(), actor, map[string]any{"display_name": targetName})
	})
	assertOneSelfUpdateConflict(t, results, "Username already exists")
	var targetCount, originalCount int
	if err := db.Pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE display_name=$1),
			COUNT(*) FILTER (WHERE display_name IN ($2,$3))
		FROM users
		WHERE id = ANY($4)
	`, targetName, first.DisplayName, second.DisplayName, []string{first.ID, second.ID}).Scan(&targetCount, &originalCount); err != nil {
		t.Fatal(err)
	}
	if targetCount != 1 || originalCount != 1 {
		t.Fatalf("concurrent display-name state: target=%d original=%d; want 1 and 1", targetCount, originalCount)
	}
}

func TestAccountServiceChangePasswordPreservesStateWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "change-password-ygg-fail@test.com", "Password123", "ChangePasswordYggFail", false)
	const refreshHash = "change_password_ygg_fail_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	err := svc.ChangePasswordSelf(ctx, accountActor(t, db, user.ID), "Password123", "NewPassword123")
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("ygg revocation failure should be returned exactly, got %v", err)
	}
	unchanged, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) || util.VerifyPassword("NewPassword123", unchanged.Password) {
		t.Fatalf("failed password change must preserve old hash: user=%#v err=%v", unchanged, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh == nil || refresh["user_id"] != user.ID {
		t.Fatalf("failed password change must preserve refresh token: refresh=%#v err=%v", refresh, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("password change should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

func TestAccountServiceChangePasswordSelfRevokesTokensAndInvalidatesCacheExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	user := testutil.CreateUser(t, db, "change-password-success@test.com", "Password123", "ChangePasswordSuccess", false)
	const refreshHash = "change_password_success_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, user.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: user.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "change_password_success_ygg", UserID: user.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.ChangePasswordSelf(ctx, actorWithPermissions(user.ID, "account_password.update.self"), "Password123", "NewPassword123"); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, user.ID)
	if err != nil || updated == nil || !util.VerifyPassword("NewPassword123", updated.Password) || util.VerifyPassword("Password123", updated.Password) {
		t.Fatalf("successful password change should persist exact hash: user=%#v err=%v", updated, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh != nil {
		t.Fatalf("successful password change should revoke refresh token: refresh=%#v err=%v", refresh, err)
	}
	if _, err := cache.GetAuthUser(ctx, user.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful password change should invalidate auth cache, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "change_password_success_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("successful password change should revoke ygg token, got %v", err)
	}
}

func TestAccountServiceDeleteSelfRejectsProtectedRoleAndDeletesPlainUserExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	protected := testutil.CreateUser(t, db, "self-delete-protected@test.com", "Password123", "SelfDeleteProtected", true, true)
	plain := testutil.CreateUser(t, db, "self-delete-plain@test.com", "Password123", "SelfDeletePlain", false)

	if err := svc.DeleteSelf(ctx, actorWithPermissions(protected.ID, "account.delete.self")); !httpErrorIs(err, http.StatusForbidden, "protected role holders cannot delete their own account") {
		t.Fatalf("protected self delete mismatch: %#v", err)
	}
	if got, err := db.Users.GetByID(ctx, protected.ID); err != nil || got == nil {
		t.Fatalf("protected self delete must keep user: user=%#v err=%v", got, err)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: plain.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "self_delete_ygg", UserID: plain.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSelf(ctx, actorWithPermissions(plain.ID, "account.delete.self")); err != nil {
		t.Fatal(err)
	}
	if got, err := db.Users.GetByID(ctx, plain.ID); err != nil || got != nil {
		t.Fatalf("DeleteSelf should remove plain user exactly: user=%#v err=%v", got, err)
	}
	if _, err := cache.GetAuthUser(ctx, plain.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("DeleteSelf should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "self_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("DeleteSelf should revoke ygg tokens exactly, got %v", err)
	}
}

func TestAccountServiceDeleteSelfPreservesUserWhenYggRevocationFails(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "self-delete-ygg-fail@test.com", "Password123", "SelfDeleteYggFail", false)
	cache := &deleteYggFailStore{Store: testutil.NewMemoryRedis()}
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	err := svc.DeleteSelf(ctx, actorWithPermissions(user.ID, "account.delete.self"))
	if err == nil || err.Error() != "ygg token revocation failed" {
		t.Fatalf("DeleteSelf ygg revocation failure should be returned exactly, got %v", err)
	}
	if got, err := db.Users.GetByID(ctx, user.ID); err != nil || got == nil || got.Email != user.Email {
		t.Fatalf("failed DeleteSelf must preserve user: user=%#v err=%v", got, err)
	}
	if cache.deleteCalls != 1 {
		t.Fatalf("DeleteSelf should attempt one ygg revocation, calls=%d", cache.deleteCalls)
	}
}

func accountActor(t testing.TB, db *database.DB, userID string) permission.Actor {
	t.Helper()
	actor, err := db.Permissions.ActorForUser(context.Background(), userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("create account actor: %v", err)
	}
	return actor
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func runConcurrentSelfUpdates(actors []permission.Actor, update func(permission.Actor) error) []error {
	start := make(chan struct{})
	results := make(chan error, len(actors))
	var wg sync.WaitGroup
	for _, actor := range actors {
		actor := actor
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- update(actor)
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	out := make([]error, 0, len(actors))
	for err := range results {
		out = append(out, err)
	}
	return out
}

func assertOneSelfUpdateConflict(t *testing.T, results []error, detail string) {
	t.Helper()
	successes := 0
	conflicts := 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case httpErrorIs(err, http.StatusBadRequest, detail):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent account result: %#v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent account updates: successes=%d conflicts=%d; want 1 and 1", successes, conflicts)
	}
}

type deleteYggFailStore struct {
	redisstore.Store
	deleteCalls int
}

func (s *deleteYggFailStore) DeleteYggTokensByUser(context.Context, string) error {
	s.deleteCalls++
	return errors.New("ygg token revocation failed")
}
