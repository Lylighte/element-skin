package account_test

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountServiceBanUserPersistsInvalidatesCacheAndSendsExactNotice(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-ban@test.com", "Password123", "AdminAccountBan", true)
	target := testutil.CreateUser(t, db, "target-account-ban@test.com", "Password123", "TargetAccountBan", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.ban.any")
	bannedUntil := database.NowMS() + int64(6*time.Hour/time.Millisecond)
	reason := "server join abuse"
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	before := database.NowMS()

	gotUntil, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "  " + reason + "  "})
	if err != nil {
		t.Fatal(err)
	}
	after := database.NowMS()
	if gotUntil != bannedUntil {
		t.Fatalf("banned until mismatch: got=%d want=%d", gotUntil, bannedUntil)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("ban should persist exact timestamp: user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("ban should invalidate auth cache exactly, got %v", err)
	}

	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, noticesvc.CurrentUser{ID: target.ID}, noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]model.NoticeView)
	if page["page_size"] != 1 || page["has_next"] != false || len(items) != 1 {
		t.Fatalf("target notice page mismatch: page=%#v items=%#v", page, items)
	}
	notice := items[0]
	wantContent := "你的账号已被管理员封禁。\n\n封禁截止时间：" + strconv.FormatInt(bannedUntil, 10) + "\n\n原因：\n\n" + reason
	if notice.Type != noticesvc.TypeSystem || notice.Title != "账号已被封禁" ||
		notice.Summary != "你的账号已被管理员封禁，详情请查看通知。" ||
		notice.ContentMarkdown != wantContent ||
		notice.DisplayMode != noticesvc.DisplayDetail || notice.Level != noticesvc.LevelDanger ||
		notice.Audience != noticesvc.AudienceTargeted || !notice.Enabled || notice.Pinned ||
		!notice.Dismissible || notice.Read || notice.CreatedBy != nil {
		t.Fatalf("ban notice content mismatch: %#v", notice)
	}
	if notice.EndsAt == nil || *notice.EndsAt < before+int64(30*24*time.Hour/time.Millisecond) ||
		*notice.EndsAt > after+int64(30*24*time.Hour/time.Millisecond) {
		t.Fatalf("ban notice end time mismatch: ends_at=%v before=%d after=%d", notice.EndsAt, before, after)
	}

	other := testutil.CreateUser(t, db, "other-account-ban@test.com", "Password123", "OtherAccountBan", false)
	otherPage, err := noticesvc.Service{DB: db}.ListForUser(ctx, noticesvc.CurrentUser{ID: other.ID}, noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if otherPage["page_size"] != 0 || len(otherPage["items"].([]model.NoticeView)) != 0 {
		t.Fatalf("ban notice should be targeted only: %#v", otherPage)
	}
}

func TestAccountServiceBanUserRejectsInvalidInputsWithoutMutation(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-ban-invalid@test.com", "Password123", "AdminAccountBanInvalid", true)
	target := testutil.CreateUser(t, db, "target-account-ban-invalid@test.com", "Password123", "TargetAccountBanInvalid", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "account.ban.any")
	future := database.NowMS() + int64(time.Hour/time.Millisecond)

	if _, err := svc.BanUser(ctx, permission.Actor{}, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: "reason"}); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("ban without permission error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: 1, Reason: "reason"}); !httpErrorIs(err, http.StatusBadRequest, "banned_until is required") {
		t.Fatalf("expired ban timestamp error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: " \t\n "}); !httpErrorIs(err, http.StatusBadRequest, "reason is required") {
		t.Fatalf("missing ban reason error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: strings.Repeat("中", 501)}); !httpErrorIs(err, http.StatusBadRequest, "reason too long") {
		t.Fatalf("long ban reason error mismatch: %#v", err)
	}
	if banned, err := db.Users.IsBanned(ctx, target.ID); err != nil || banned {
		t.Fatalf("invalid ban inputs must not change target state: banned=%v err=%v", banned, err)
	}
	page, err := noticesvc.Service{DB: db}.ListForManagement(ctx, actorWithPermissions(adminUser.ID, "notice.read.any"), noticesvc.ListParams{Type: noticesvc.TypeSystem, Status: noticesvc.StatusAll, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page["page_size"] != 0 || len(page["items"].([]model.Notice)) != 0 {
		t.Fatalf("invalid ban inputs must not create notices: %#v", page)
	}
}

func TestAccountServiceProtectsProtectedSubjectAndUnbansExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	plainAdmin := testutil.CreateUser(t, db, "plain-account-protect@test.com", "Password123", "PlainAccountProtect", true)
	protectedAdmin := testutil.CreateUser(t, db, "protected-account-protect@test.com", "Password123", "ProtectedAccountProtect", true, true)
	target := testutil.CreateUser(t, db, "target-account-unban@test.com", "Password123", "TargetAccountUnban", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	plainActor := actorWithPermissions(plainAdmin.ID, "account.ban.any")
	future := database.NowMS() + int64(time.Hour/time.Millisecond)
	if _, err := svc.BanUser(ctx, plainActor, protectedAdmin.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: "protected"}); !httpErrorIs(err, http.StatusForbidden, "cannot modify protected subject") {
		t.Fatalf("protected admin ban error mismatch: %#v", err)
	}
	if banned, err := db.Users.IsBanned(ctx, protectedAdmin.ID); err != nil || banned {
		t.Fatalf("protected admin ban must not mutate user: banned=%v err=%v", banned, err)
	}

	unbanActor := actorWithPermissions(plainAdmin.ID, "account.unban.any")
	if err := db.Users.Ban(ctx, target.ID, future); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.UnbanUser(ctx, unbanActor, target.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || updated.BannedUntil != nil {
		t.Fatalf("unban should clear banned_until exactly: user=%#v err=%v", updated, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("unban should invalidate auth cache exactly, got %v", err)
	}

	if err := svc.UnbanUser(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("unban without permission error mismatch: %#v", err)
	}
	if err := svc.UnbanUser(ctx, unbanActor, "missing-account-unban"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("missing unban user error mismatch: %#v", err)
	}
}

func TestAccountServicePropagatesClosedDatabaseErrors(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-closed@test.com", "Password123", "AdminAccountClosed", true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(
		adminUser.ID,
		"user.read.any",
		"account.read.any",
		"permission.grant.any",
		"permission.revoke.any",
		"permission_protected.manage.any",
		"account.delete.any",
		"account.update.any",
		"account.ban.any",
		"account.unban.any",
	)
	db.Close()

	if result, err := svc.ListUsers(ctx, actor, "", 10, ""); result != nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("ListUsers closed db = result=%#v err=%v; want nil closed pool", result, err)
	}
	if result, err := svc.UserDetail(ctx, actor, adminUser.ID); result != nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UserDetail closed db = result=%#v err=%v; want nil closed pool", result, err)
	}
	if err := svc.GrantUserRole(ctx, actor, adminUser.ID, permission.RoleAdmin); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("GrantUserRole closed db error mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, adminUser.ID, permission.RoleAdmin); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("RevokeUserRole closed db error mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, "target-account-closed"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("TransferProtectedSubject closed db error mismatch: %#v", err)
	}
	if err := svc.DeleteUser(ctx, actor, "target-account-closed"); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("DeleteUser closed db error mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: "target-account-closed", NewPassword: "ChangedPassword123"}); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("ResetPassword closed db error mismatch: %#v", err)
	}
	if _, err := svc.BanUser(ctx, actor, adminUser.ID, accountsvc.BanUserInput{BannedUntil: database.NowMS() + int64(time.Hour/time.Millisecond), Reason: "closed"}); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("BanUser closed db error mismatch: %#v", err)
	}
	if err := svc.UnbanUser(ctx, actor, adminUser.ID); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UnbanUser closed db error mismatch: %#v", err)
	}
}

func TestAccountServiceRoleMutationDependencyErrorsAndMissingUsers(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "admin-account-role-errors@test.com", "Password123", "AdminRoleErrors", true)
	target := testutil.CreateUser(t, db, "target-account-role-errors@test.com", "Password123", "TargetRoleErrors", false)
	cache := &accountFailStore{Store: redisstore.NewMemoryStore(), failInvalidate: true}
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(admin.ID, "permission.grant.any", "permission.revoke.any")

	if err := svc.GrantUserRole(ctx, actor, "missing-role-user", permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("grant missing user mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, "missing-role-user", permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("revoke missing user mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, ""); !httpErrorIs(err, http.StatusBadRequest, "role_id required") {
		t.Fatalf("revoke empty role mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, permission.Actor{}, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("revoke without permission mismatch: %#v", err)
	}
	err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin)
	if err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("grant cache failure mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("grant should persist role before cache failure is returned: has=%v err=%v", hasRole, err)
	}
}

func TestAccountServiceDeleteResetBanUnbanDependencyFailuresKeepExactState(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "admin-account-dependency@test.com", "Password123", "AdminDependency", true)
	target := testutil.CreateUser(t, db, "target-account-dependency@test.com", "Password123", "TargetDependency", false)
	actor := actorWithPermissions(admin.ID, "account.delete.any", "account.update.any", "account.ban.any", "account.unban.any")

	yggFail := &accountFailStore{Store: redisstore.NewMemoryStore(), failYggDelete: true}
	svc := accountsvc.AccountService{DB: db, Redis: yggFail}
	if err := svc.DeleteUser(ctx, actor, target.ID); err == nil || err.Error() != "ygg token deletion failed" {
		t.Fatalf("delete ygg failure mismatch: %#v", err)
	}
	if user, err := db.Users.GetByID(ctx, target.ID); err != nil || user == nil {
		t.Fatalf("delete ygg failure must keep target user: user=%#v err=%v", user, err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "ChangedPassword123"}); err == nil || err.Error() != "ygg token deletion failed" {
		t.Fatalf("reset ygg failure mismatch: %#v", err)
	}
	if unchanged, err := db.Users.GetByID(ctx, target.ID); err != nil || unchanged == nil || !util.VerifyPassword("Password123", unchanged.Password) {
		t.Fatalf("reset ygg failure must preserve password: user=%#v err=%v", unchanged, err)
	}

	cacheFail := &accountFailStore{Store: redisstore.NewMemoryStore(), failInvalidate: true}
	svc = accountsvc.AccountService{DB: db, Redis: cacheFail}
	bannedUntil := database.NowMS() + int64(time.Hour/time.Millisecond)
	if _, err := svc.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "cache failure"}); err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("ban cache failure mismatch: %#v", err)
	}
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("ban cache failure should preserve ban timestamp: user=%#v err=%v", updated, err)
	}
	if err := svc.UnbanUser(ctx, actor, target.ID); err == nil || err.Error() != "auth cache invalidation failed" {
		t.Fatalf("unban cache failure mismatch: %#v", err)
	}
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil != nil {
		t.Fatalf("unban cache failure should still clear ban timestamp: user=%#v err=%v", updated, err)
	}

	noticeFail := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE notices CASCADE`); err != nil {
		t.Fatal(err)
	}
	nextBan := database.NowMS() + int64(2*time.Hour/time.Millisecond)
	gotUntil, err := noticeFail.BanUser(ctx, actor, target.ID, accountsvc.BanUserInput{BannedUntil: nextBan, Reason: "notice failure"})
	if gotUntil != 0 {
		t.Fatalf("ban notice failure returned banned_until=%d want 0", gotUntil)
	}
	assertAccountPgCode(t, err, "42P01")
	if updated, err := db.Users.GetByID(ctx, target.ID); err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != nextBan {
		t.Fatalf("ban notice failure should preserve ban timestamp: user=%#v err=%v", updated, err)
	}
}

func TestAccountServiceProtectedManagerCanBanProtectedUser(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	manager := testutil.CreateUser(t, db, "manager-account-protected@test.com", "Password123", "ManagerAccountProtected", true, true)
	protected := testutil.CreateUser(t, db, "protected-account-ban@test.com", "Password123", "ProtectedAccountBan", true, true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(manager.ID, "account.ban.any", "permission_protected.manage.any")
	bannedUntil := database.NowMS() + int64(time.Hour/time.Millisecond)

	if _, err := svc.BanUser(ctx, actor, protected.ID, accountsvc.BanUserInput{BannedUntil: bannedUntil, Reason: "protected managed"}); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, protected.ID)
	if err != nil || updated == nil || updated.BannedUntil == nil || *updated.BannedUntil != bannedUntil {
		t.Fatalf("protected manager ban mismatch: user=%#v err=%v", updated, err)
	}
}

func TestAccountServiceGrantAndRevokeRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-role@test.com", "Password123", "AdminAccountRole", true)
	target := testutil.CreateUser(t, db, "target-account-role@test.com", "Password123", "TargetAccountRole", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || !hasRole {
		t.Fatalf("grant role should persist exact role: has=%v err=%v", hasRole, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("grant role should invalidate auth cache exactly, got %v", err)
	}

	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("revoke role should remove exact role: has=%v err=%v", hasRole, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("revoke role should invalidate auth cache exactly, got %v", err)
	}

	if err := svc.GrantUserRole(ctx, permission.Actor{}, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("grant without permission mismatch: %#v", err)
	}
	if err := svc.GrantUserRole(ctx, actor, target.ID, ""); !httpErrorIs(err, http.StatusBadRequest, "role_id required") {
		t.Fatalf("grant empty role mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); !httpErrorIs(err, http.StatusNotFound, "role assignment not found") {
		t.Fatalf("revoke missing role mismatch: %#v", err)
	}
}

func TestAccountServiceRevokeRoleReconcilesOAuthDependentsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-role-oauth@test.com", "Password123", "AdminRoleOAuth", true)
	target := testutil.CreateUser(t, db, "target-account-role-oauth@test.com", "Password123", "TargetRoleOAuth", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	if err := svc.GrantUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}
	client := createAccountOAuthClient(t, db, target.ID, "account-role-reconcile-client", "invite.create.any")
	grant := model.OAuthGrant{
		ID:        "account-role-reconcile-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 2100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, accountOAuthPermissionIDs("invite.create.any")); err != nil {
		t.Fatal(err)
	}
	unaffectedClient := createAccountOAuthClient(t, db, target.ID, "account-role-unaffected-client", "account.read.self")
	unaffectedGrant := model.OAuthGrant{
		ID:        "account-role-unaffected-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  unaffectedClient.ID,
		Status:    "active",
		CreatedAt: 2200,
	}
	if err := db.OAuth.CreateGrant(ctx, unaffectedGrant, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	before := database.NowMS()

	if err := svc.RevokeUserRole(ctx, actor, target.ID, permission.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleAdmin); err != nil || hasRole {
		t.Fatalf("role revoke should persist before OAuth assertions: has=%v err=%v", hasRole, err)
	}
	grants, err := db.OAuth.ListGrantsByUser(ctx, target.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 {
		t.Fatalf("grant count after role revoke = %d want 2: %#v", len(grants), grants)
	}
	revokedGrant := accountOAuthGrantByID(grants, grant.ID)
	if revokedGrant == nil || revokedGrant.Status != "revoked" ||
		revokedGrant.RevokedAt == nil || *revokedGrant.RevokedAt < before {
		t.Fatalf("oauth grant should be revoked exactly after role revoke: %#v", grants)
	}
	keptGrant := accountOAuthGrantByID(grants, unaffectedGrant.ID)
	if keptGrant == nil || keptGrant.Status != "active" || keptGrant.RevokedAt != nil {
		t.Fatalf("unaffected oauth grant should remain active exactly: %#v", grants)
	}
	gotClient, err := db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotClient == nil || gotClient.Status != "disabled" || gotClient.UpdatedAt < before {
		t.Fatalf("oauth client should be disabled exactly after owner role revoke: %#v", gotClient)
	}
	gotUnaffectedClient, err := db.OAuth.GetClient(ctx, unaffectedClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotUnaffectedClient == nil || gotUnaffectedClient.Status != "active" || gotUnaffectedClient.UpdatedAt != unaffectedClient.UpdatedAt {
		t.Fatalf("unaffected oauth client should remain active exactly: %#v", gotUnaffectedClient)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("role revoke should invalidate auth cache exactly, got %v", err)
	}
}

func TestAccountServiceListUsersAndUserDetailAttachRolesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-list@test.com", "Password123", "AdminAccountList", true)
	target := testutil.CreateUser(t, db, "target-account-list@test.com", "Password123", "TargetAccountList", false)
	other := testutil.CreateUser(t, db, "other-account-list@test.com", "Password123", "OtherAccountList", true)
	if err := db.Permissions.GrantRole(ctx, target.ID, permission.RoleAdmin, ""); err != nil {
		t.Fatal(err)
	}
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "user.read.any", "account.read.any")

	page, err := svc.ListUsers(ctx, actor, "", 10, "target-account-list")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if page["page_size"] != 1 || page["has_next"] != false || page["next_cursor"] != "" || len(items) != 1 {
		t.Fatalf("list users page mismatch: %#v", page)
	}
	roles := items[0]["roles"].([]string)
	if items[0]["id"] != target.ID || items[0]["email"] != target.Email || items[0]["display_name"] != target.DisplayName ||
		items[0]["protected"] != false ||
		!stringSliceSetEquals(roles, []string{permission.RoleUser, permission.RoleAdmin}) {
		t.Fatalf("list users item mismatch: %#v", items[0])
	}

	detail, err := svc.UserDetail(ctx, actor, other.ID)
	if err != nil {
		t.Fatal(err)
	}
	detailRoles := detail["roles"].([]string)
	if detail["id"] != other.ID || detail["email"] != other.Email || detail["display_name"] != other.DisplayName ||
		detail["protected"] != false ||
		!stringSliceSetEquals(detailRoles, []string{permission.RoleUser, permission.RoleAdmin}) {
		t.Fatalf("user detail mismatch: %#v", detail)
	}
}

func TestAccountServiceListAndDetailRejectInvalidAccessExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-list-invalid@test.com", "Password123", "AdminAccountListInvalid", true)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	actor := actorWithPermissions(adminUser.ID, "user.read.any", "account.read.any")

	if _, err := svc.ListUsers(ctx, permission.Actor{}, "", 10, ""); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("ListUsers without permission mismatch: %#v", err)
	}
	if _, err := svc.ListUsers(ctx, actor, "bad-cursor", 10, ""); !httpErrorIs(err, http.StatusBadRequest, "Invalid cursor") {
		t.Fatalf("ListUsers bad cursor mismatch: %#v", err)
	}
	if _, err := svc.ListUsers(ctx, actor, util.EncodeCursor(map[string]any{"wrong": "field"}), 10, ""); !httpErrorIs(err, http.StatusBadRequest, "Invalid cursor") {
		t.Fatalf("ListUsers cursor missing last_id mismatch: %#v", err)
	}
	if _, err := svc.UserDetail(ctx, permission.Actor{}, adminUser.ID); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("UserDetail without permission mismatch: %#v", err)
	}
	if _, err := svc.UserDetail(ctx, actor, "missing-account-detail"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("UserDetail missing user mismatch: %#v", err)
	}
}

func TestAccountServiceProtectsProtectedRoleMutationsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-protected-role@test.com", "Password123", "AdminAccountProtectedRole", true)
	target := testutil.CreateUser(t, db, "target-account-protected-role@test.com", "Password123", "TargetAccountProtectedRole", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	plainActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	managerActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any", "permission_protected.manage.any")

	if err := svc.GrantUserRole(ctx, plainActor, target.ID, permission.RoleSystemMaintenance); !httpErrorIs(err, http.StatusForbidden, "protected role management required") {
		t.Fatalf("protected role generic grant mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || hasRole {
		t.Fatalf("rejected protected role generic grant must not mutate target: has=%v err=%v", hasRole, err)
	}
	if err := svc.GrantUserRole(ctx, managerActor, target.ID, permission.RoleSystemMaintenance); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || !hasRole {
		t.Fatalf("manager protected role grant mismatch: has=%v err=%v", hasRole, err)
	}
	if err := svc.RevokeUserRole(ctx, plainActor, target.ID, permission.RoleSystemMaintenance); !httpErrorIs(err, http.StatusForbidden, "protected role management required") {
		t.Fatalf("protected role generic revoke mismatch: %#v", err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSystemMaintenance); err != nil || !hasRole {
		t.Fatalf("rejected protected role generic revoke must preserve target: has=%v err=%v", hasRole, err)
	}
}

func TestAccountServiceTransfersProtectedSubjectExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	actorUser := testutil.CreateUser(t, db, "actor-account-transfer@test.com", "Password123", "ActorAccountTransfer", true, true)
	target := testutil.CreateUser(t, db, "target-account-transfer@test.com", "Password123", "TargetAccountTransfer", false)
	stale := testutil.CreateUser(t, db, "stale-account-transfer@test.com", "Password123", "StaleAccountTransfer", true, true)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(actorUser.ID, "permission_protected.manage.any")
	for _, userID := range []string{actorUser.ID, target.ID, stale.ID} {
		if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: userID}, time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.TransferProtectedSubject(ctx, permission.Actor{}, target.ID); !httpErrorIs(err, http.StatusForbidden, "protected subject management required") {
		t.Fatalf("transfer without protected permission mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, actorUser.ID); !httpErrorIs(err, http.StatusForbidden, "cannot transfer protected subject to yourself") {
		t.Fatalf("self transfer mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, "missing-account-transfer"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("missing transfer target mismatch: %#v", err)
	}
	if err := svc.TransferProtectedSubject(ctx, actor, target.ID); err != nil {
		t.Fatal(err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, actorUser.ID); err != nil || protected {
		t.Fatalf("actor protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, target.ID); err != nil || !protected {
		t.Fatalf("target protected flag after transfer = %v, %v; want true, nil", protected, err)
	}
	if protected, err := db.Permissions.UserIsProtected(ctx, stale.ID); err != nil || protected {
		t.Fatalf("stale protected flag after transfer = %v, %v; want false, nil", protected, err)
	}
	for _, userID := range []string{actorUser.ID, target.ID, stale.ID} {
		if _, err := cache.GetAuthUser(ctx, userID); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("transfer should invalidate auth cache for %s exactly, got %v", userID, err)
		}
	}
}

func TestAccountServiceDeleteUserCascadesAndInvalidatesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-delete@test.com", "Password123", "AdminAccountDelete", true)
	target := testutil.CreateUser(t, db, "target-account-delete@test.com", "Password123", "TargetAccountDelete", false)
	other := testutil.CreateUser(t, db, "other-account-delete@test.com", "Password123", "OtherAccountDelete", false)
	unaffected := testutil.CreateUser(t, db, "unaffected-account-delete@test.com", "Password123", "UnaffectedAccountDelete", false)
	profile := testutil.CreateProfile(t, db, target.ID, "account_delete_profile", "AccountDeleteProfile")
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.delete.any")
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "account_delete_ygg", UserID: target.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}
	ownedClient := createAccountOAuthClient(t, db, target.ID, "account-delete-owned-client", "account.read.self")
	otherClient := createAccountOAuthClient(t, db, other.ID, "account-delete-other-client", "account.read.self")
	targetGrantToOther := model.OAuthGrant{
		ID:        "account-delete-target-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  otherClient.ID,
		Status:    "active",
		CreatedAt: 3100,
	}
	otherGrantToTargetApp := model.OAuthGrant{
		ID:        "account-delete-other-grant",
		UserID:    other.ID,
		SubjectID: permissiondb.SubjectIDForUser(other.ID),
		ClientID:  ownedClient.ID,
		Status:    "active",
		CreatedAt: 3200,
	}
	if err := db.OAuth.CreateGrant(ctx, targetGrantToOther, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateGrant(ctx, otherGrantToTargetApp, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	unaffectedClient := createAccountOAuthClient(t, db, unaffected.ID, "account-delete-unaffected-client", "account.read.self")
	unaffectedGrant := model.OAuthGrant{
		ID:        "account-delete-unaffected-grant",
		UserID:    unaffected.ID,
		SubjectID: permissiondb.SubjectIDForUser(unaffected.ID),
		ClientID:  unaffectedClient.ID,
		Status:    "active",
		CreatedAt: 3250,
	}
	if err := db.OAuth.CreateGrant(ctx, unaffectedGrant, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateRefreshToken(ctx, model.OAuthToken{
		TokenHash: "account-delete-refresh",
		ClientID:  otherClient.ID,
		UserID:    target.ID,
		GrantID:   targetGrantToOther.ID,
		ExpiresAt: 9000,
		CreatedAt: 3300,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.OAuth.CreateAuthorizationCode(ctx, model.OAuthAuthorizationCode{
		CodeHash:            "account-delete-code",
		ClientID:            otherClient.ID,
		UserID:              target.ID,
		GrantID:             targetGrantToOther.ID,
		RedirectURI:         otherClient.RedirectURI,
		CodeChallenge:       "challenge",
		CodeChallengeMethod: "S256",
		ExpiresAt:           9000,
		CreatedAt:           3400,
	}, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	userID := target.ID
	subjectID := permissiondb.SubjectIDForUser(target.ID)
	if err := db.OAuth.CreateDeviceCode(ctx, model.OAuthDeviceCode{
		DeviceCodeHash: "account-delete-device",
		UserCodeHash:   "account-delete-user-code",
		ClientID:       otherClient.ID,
		UserID:         &userID,
		SubjectID:      &subjectID,
		Status:         "approved",
		ExpiresAt:      9000,
		CreatedAt:      3500,
	}, accountOAuthPermissionIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}

	if err := svc.DeleteUser(ctx, actor, target.ID); err != nil {
		t.Fatal(err)
	}
	if user, err := db.Users.GetByID(ctx, target.ID); err != nil || user != nil {
		t.Fatalf("delete should remove user row exactly: user=%#v err=%v", user, err)
	}
	if gotProfile, err := db.Profiles.GetByID(ctx, profile.ID); err != nil || gotProfile != nil {
		t.Fatalf("delete should cascade profile row exactly: profile=%#v err=%v", gotProfile, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "account_delete_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("delete should revoke ygg tokens exactly, got %v", err)
	}
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, ownedClient.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForClient(ownedClient.ID), 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, otherClient.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, targetGrantToOther.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, otherGrantToTargetApp.ID, 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_clients WHERE id=$1`, unaffectedClient.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM permission_subjects WHERE id=$1`, permissiondb.SubjectIDForClient(unaffectedClient.ID), 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM delegated_permission_grants WHERE id=$1`, unaffectedGrant.ID, 1)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_refresh_tokens WHERE token_hash=$1`, "account-delete-refresh", 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_authorization_codes WHERE code_hash=$1`, "account-delete-code", 0)
	assertAccountRowCount(t, db, `SELECT COUNT(*) FROM oauth_device_codes WHERE device_code_hash=$1`, "account-delete-device", 0)

	if err := svc.DeleteUser(ctx, actor, adminUser.ID); !httpErrorIs(err, http.StatusForbidden, "cannot delete yourself") {
		t.Fatalf("self delete error mismatch: %#v", err)
	}
	if err := svc.DeleteUser(ctx, actor, "missing-account-delete"); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("delete missing user mismatch: %#v", err)
	}
}

func TestAccountServiceResetPasswordRevokesTokensAndInvalidatesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-reset@test.com", "Password123", "AdminAccountReset", true)
	target := testutil.CreateUser(t, db, "target-account-reset@test.com", "Password123", "TargetAccountReset", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "account.update.any")
	refreshHash := "account_reset_refresh"
	if err := db.Tokens.AddRefresh(ctx, refreshHash, target.ID, database.NowMS()+int64(time.Hour/time.Millisecond), database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetYggToken(ctx, model.Token{AccessToken: "account_reset_ygg", UserID: target.ID, CreatedAt: database.NowMS()}, time.Hour); err != nil {
		t.Fatal(err)
	}

	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "ChangedPassword123"}); err != nil {
		t.Fatal(err)
	}
	updated, err := db.Users.GetByID(ctx, target.ID)
	if err != nil || updated == nil || !util.VerifyPassword("ChangedPassword123", updated.Password) || util.VerifyPassword("Password123", updated.Password) {
		t.Fatalf("reset should persist exact new password hash: user=%#v err=%v", updated, err)
	}
	if refresh, err := db.Tokens.GetRefresh(ctx, refreshHash); err != nil || refresh != nil {
		t.Fatalf("reset should revoke refresh token exactly: refresh=%#v err=%v", refresh, err)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset should invalidate auth cache exactly, got %v", err)
	}
	if _, err := cache.GetYggToken(ctx, "account_reset_ygg"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("reset should revoke ygg tokens exactly, got %v", err)
	}

	if err := svc.ResetPassword(ctx, permission.Actor{}, accountsvc.ResetPasswordInput{UserID: target.ID, NewPassword: "NextPassword123"}); !httpErrorIs(err, http.StatusForbidden, "permission denied") {
		t.Fatalf("reset without permission mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: target.ID}); !httpErrorIs(err, http.StatusBadRequest, "user_id and new_password required") {
		t.Fatalf("reset missing password mismatch: %#v", err)
	}
	if err := svc.ResetPassword(ctx, actor, accountsvc.ResetPasswordInput{UserID: "missing-account-reset", NewPassword: "NextPassword123"}); !httpErrorIs(err, http.StatusNotFound, "user not found") {
		t.Fatalf("reset missing user mismatch: %#v", err)
	}
}

func TestAccountServiceDeleteUserRecountsSharedTexturesAndDeletesUploadedTextures(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-delete-texture@test.com", "Password123", "AdminDeleteTexture", true)
	owner := testutil.CreateUser(t, db, "owner-account-delete-texture@test.com", "Password123", "OwnerDeleteTexture", false)
	target := testutil.CreateUser(t, db, "target-account-delete-texture@test.com", "Password123", "TargetDeleteTexture", false)
	other := testutil.CreateUser(t, db, "other-account-delete-texture@test.com", "Password123", "OtherDeleteTexture", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}

	if err := db.Textures.AddToLibrary(ctx, owner.ID, "delete_user_shared_skin", "skin", "Delete User Shared", true, "default"); err != nil {
		t.Fatal(err)
	}
	if err := db.Textures.AddToLibrary(ctx, target.ID, "delete_user_uploaded_skin", "skin", "Delete User Uploaded", true, "slim"); err != nil {
		t.Fatal(err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, target.ID, "delete_user_shared_skin", "skin"); err != nil || !added {
		t.Fatalf("seed target shared texture: added=%v err=%v", added, err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_user_shared_skin", "skin"); err != nil || !added {
		t.Fatalf("seed other shared texture: added=%v err=%v", added, err)
	}
	if added, err := db.Textures.AddToWardrobe(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || !added {
		t.Fatalf("seed other uploaded texture: added=%v err=%v", added, err)
	}

	if err := svc.DeleteUser(ctx, actorWithPermissions(adminUser.ID, "account.delete.any"), target.ID); err != nil {
		t.Fatal(err)
	}
	public, err := db.Textures.ListPublic(ctx, texture.PublicListOptions{Limit: 10, TextureType: "skin", Query: "delete_user_shared_skin", Sort: texture.PublicLibrarySortMostUsed})
	if err != nil {
		t.Fatal(err)
	}
	items := public["items"].([]map[string]any)
	if len(items) != 1 || items[0]["hash"] != "delete_user_shared_skin" || items[0]["usage_count"] != int64(2) {
		t.Fatalf("shared texture usage after delete = %#v; want one row with usage_count=2", public)
	}
	if exists, err := db.Textures.Exists(ctx, "delete_user_uploaded_skin", "skin"); err != nil || exists {
		t.Fatalf("deleting uploader should remove uploaded public texture: exists=%v err=%v", exists, err)
	}
	if info, err := db.Textures.GetInfo(ctx, other.ID, "delete_user_uploaded_skin", "skin"); err != nil || info != nil {
		t.Fatalf("deleting uploader should remove other users' wardrobe copies: info=%#v err=%v", info, err)
	}
}

func actorWithPermissions(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   permissiondb.SubjectIDForUser(userID),
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointAdmin,
		Permissions: bits,
	}
}

func httpErrorIs(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}

func createAccountOAuthClient(t testing.TB, db *database.DB, ownerUserID, clientID string, codes ...string) model.OAuthClient {
	t.Helper()
	client := model.OAuthClient{
		ID:          clientID,
		OwnerUserID: ownerUserID,
		Name:        clientID,
		Description: "account service oauth fixture",
		RedirectURI: "https://" + clientID + ".example/callback",
		WebsiteURL:  "https://" + clientID + ".example",
		ClientType:  "confidential",
		SecretHash:  clientID + "-secret-hash",
		Status:      "active",
		CreatedAt:   2000,
		UpdatedAt:   2000,
	}
	if err := db.OAuth.CreateClient(context.Background(), client, accountOAuthPermissionIDs(codes...)); err != nil {
		t.Fatal(err)
	}
	return client
}

func accountOAuthPermissionIDs(codes ...string) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		ids = append(ids, int64(permission.MustDefinitionByCode(code).ID))
	}
	return ids
}

func assertAccountRowCount(t testing.TB, db *database.DB, query string, arg any, want int) {
	t.Helper()
	var got int
	if err := db.Pool.QueryRow(context.Background(), query, arg).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("row count mismatch for %q arg=%v: got=%d want=%d", query, arg, got, want)
	}
}

func accountOAuthGrantByID(grants []model.OAuthGrant, id string) *model.OAuthGrant {
	for i := range grants {
		if grants[i].ID == id {
			return &grants[i]
		}
	}
	return nil
}

type accountFailStore struct {
	redisstore.Store
	failInvalidate bool
	failYggDelete  bool
}

func (s *accountFailStore) InvalidateAuthUser(ctx context.Context, userID string) error {
	if s.failInvalidate {
		return errors.New("auth cache invalidation failed")
	}
	return s.Store.InvalidateAuthUser(ctx, userID)
}

func (s *accountFailStore) DeleteYggTokensByUser(ctx context.Context, userID string) error {
	if s.failYggDelete {
		return errors.New("ygg token deletion failed")
	}
	return s.Store.DeleteYggTokensByUser(ctx, userID)
}

func assertAccountPgCode(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("PostgreSQL error mismatch: got=%T %v want SQLSTATE %s", err, err, code)
	}
	if pgErr.Code != code {
		t.Fatalf("PostgreSQL SQLSTATE mismatch: got=%s want=%s message=%s", pgErr.Code, code, pgErr.Message)
	}
}

func stringSliceSetEquals(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[string]int{}
	for _, item := range got {
		seen[item]++
	}
	for _, item := range want {
		seen[item]--
		if seen[item] < 0 {
			return false
		}
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return true
}
