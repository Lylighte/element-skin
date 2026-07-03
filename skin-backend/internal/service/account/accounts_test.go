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

func TestAccountServiceProtectsSuperAdminAndUnbansExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	plainAdmin := testutil.CreateUser(t, db, "plain-account-protect@test.com", "Password123", "PlainAccountProtect", true)
	protectedAdmin := testutil.CreateUser(t, db, "protected-account-protect@test.com", "Password123", "ProtectedAccountProtect", true, true)
	target := testutil.CreateUser(t, db, "target-account-unban@test.com", "Password123", "TargetAccountUnban", false)
	cache := redisstore.NewMemoryStore()
	svc := accountsvc.AccountService{DB: db, Redis: cache}

	plainActor := actorWithPermissions(plainAdmin.ID, "account.ban.any")
	future := database.NowMS() + int64(time.Hour/time.Millisecond)
	if _, err := svc.BanUser(ctx, plainActor, protectedAdmin.ID, accountsvc.BanUserInput{BannedUntil: future, Reason: "protected"}); !httpErrorIs(err, http.StatusForbidden, "cannot modify super admin") {
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
	actor := actorWithPermissions(adminUser.ID, "account.ban.any", "account.unban.any")
	db.Close()

	if _, err := svc.BanUser(ctx, actor, adminUser.ID, accountsvc.BanUserInput{BannedUntil: database.NowMS() + int64(time.Hour/time.Millisecond), Reason: "closed"}); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("BanUser closed db error mismatch: %#v", err)
	}
	if err := svc.UnbanUser(ctx, actor, adminUser.ID); err == nil || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("UnbanUser closed db error mismatch: %#v", err)
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

func TestAccountServiceProtectsProtectedRoleMutationsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-protected-role@test.com", "Password123", "AdminAccountProtectedRole", true)
	target := testutil.CreateUser(t, db, "target-account-protected-role@test.com", "Password123", "TargetAccountProtectedRole", false)
	svc := accountsvc.AccountService{DB: db, Redis: redisstore.NewMemoryStore()}
	plainActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any")
	managerActor := actorWithPermissions(adminUser.ID, "permission.grant.any", "permission.revoke.any", "permission_protected.manage.any")

	if err := svc.GrantUserRole(ctx, plainActor, target.ID, permission.RoleSuperAdmin); !httpErrorIs(err, http.StatusForbidden, "protected role management required") {
		t.Fatalf("plain protected role grant mismatch: %#v", err)
	}
	if err := svc.GrantUserRole(ctx, managerActor, adminUser.ID, permission.RoleSuperAdmin); !httpErrorIs(err, http.StatusForbidden, "cannot grant protected role to yourself") {
		t.Fatalf("self protected role grant mismatch: %#v", err)
	}
	if err := svc.GrantUserRole(ctx, managerActor, target.ID, permission.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if err := svc.RevokeUserRole(ctx, plainActor, target.ID, permission.RoleSuperAdmin); !httpErrorIs(err, http.StatusForbidden, "protected role management required") {
		t.Fatalf("plain protected role revoke mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, managerActor, adminUser.ID, permission.RoleSuperAdmin); !httpErrorIs(err, http.StatusForbidden, "cannot revoke protected role from yourself") {
		t.Fatalf("self protected role revoke mismatch: %#v", err)
	}
	if err := svc.RevokeUserRole(ctx, managerActor, target.ID, permission.RoleSuperAdmin); err != nil {
		t.Fatal(err)
	}
	if hasRole, err := db.Permissions.UserHasRole(ctx, target.ID, permission.RoleSuperAdmin); err != nil || hasRole {
		t.Fatalf("manager revoke protected role mismatch: has=%v err=%v", hasRole, err)
	}
}

func TestAccountServiceDeleteUserCascadesAndInvalidatesExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-account-delete@test.com", "Password123", "AdminAccountDelete", true)
	target := testutil.CreateUser(t, db, "target-account-delete@test.com", "Password123", "TargetAccountDelete", false)
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
