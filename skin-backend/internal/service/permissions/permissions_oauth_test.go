package permissions_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	noticesvc "element-skin/backend/internal/service/notice"
	permissionssvc "element-skin/backend/internal/service/permissions"
	"element-skin/backend/internal/testutil"
)

func TestPermissionServiceSetOverrideReconcilesOAuthDependentsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	adminUser := testutil.CreateUser(t, db, "admin-perms-oauth@test.com", "Password123", "AdminPermsOAuth", true)
	target := testutil.CreateUser(t, db, "target-perms-oauth@test.com", "Password123", "TargetPermsOAuth", false)
	cache := redisstore.NewMemoryStore()
	svc := permissionssvc.PermissionService{DB: db, Redis: cache}
	actor := actorWithPermissions(adminUser.ID, "permission.revoke.any")
	client := model.OAuthClient{
		ID:          "permission-reconcile-client",
		OwnerUserID: target.ID,
		Name:        "Permission reconcile client",
		Description: "client disabled when owner loses requested permission",
		RedirectURI: "https://permission-reconcile.example/callback",
		WebsiteURL:  "https://permission-reconcile.example",
		ClientType:  "confidential",
		SecretHash:  "secret-hash",
		Status:      "active",
		CreatedAt:   1000,
		UpdatedAt:   1000,
	}
	clientPermissionIDs := permissionTestIDs("account.read.self", "profile.update.owned", "minecraft_session.hasjoined.server")
	if err := db.OAuth.CreateClient(ctx, client, clientPermissionIDs); err != nil {
		t.Fatal(err)
	}
	grant := model.OAuthGrant{
		ID:        "permission-reconcile-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  client.ID,
		Status:    "active",
		CreatedAt: 1100,
	}
	if err := db.OAuth.CreateGrant(ctx, grant, permissionTestIDs("profile.update.owned")); err != nil {
		t.Fatal(err)
	}
	unaffectedClient := model.OAuthClient{
		ID:          "permission-reconcile-unaffected-client",
		OwnerUserID: target.ID,
		Name:        "Permission reconcile unaffected client",
		Description: "client kept active when owner keeps requested permission",
		RedirectURI: "https://permission-reconcile-unaffected.example/callback",
		WebsiteURL:  "https://permission-reconcile-unaffected.example",
		ClientType:  "confidential",
		SecretHash:  "unaffected-secret-hash",
		Status:      "active",
		CreatedAt:   1200,
		UpdatedAt:   1200,
	}
	if err := db.OAuth.CreateClient(ctx, unaffectedClient, permissionTestIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	unaffectedGrant := model.OAuthGrant{
		ID:        "permission-reconcile-unaffected-grant",
		UserID:    target.ID,
		SubjectID: permissiondb.SubjectIDForUser(target.ID),
		ClientID:  unaffectedClient.ID,
		Status:    "active",
		CreatedAt: 1300,
	}
	if err := db.OAuth.CreateGrant(ctx, unaffectedGrant, permissionTestIDs("account.read.self")); err != nil {
		t.Fatal(err)
	}
	if err := cache.SetAuthUser(ctx, redisstore.AuthUser{ID: target.ID}, 0); err != nil {
		t.Fatal(err)
	}
	before := database.NowMS()

	if err := svc.SetUserPermissionOverride(ctx, actor, target.ID, "profile.update.owned", "deny"); err != nil {
		t.Fatal(err)
	}

	grants, err := db.OAuth.ListGrantsByUser(ctx, target.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 2 {
		t.Fatalf("grant count after permission denial = %d want 2: %#v", len(grants), grants)
	}
	revokedGrant := oauthGrantByID(grants, grant.ID)
	if revokedGrant == nil || revokedGrant.Status != "revoked" ||
		revokedGrant.RevokedAt == nil || *revokedGrant.RevokedAt < before {
		t.Fatalf("oauth grant should be revoked exactly after permission denial: %#v", grants)
	}
	keptGrant := oauthGrantByID(grants, unaffectedGrant.ID)
	if keptGrant == nil || keptGrant.Status != "active" || keptGrant.RevokedAt != nil {
		t.Fatalf("unaffected oauth grant should remain active exactly: %#v", grants)
	}
	gotClient, err := db.OAuth.GetClient(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotClient == nil || gotClient.Status != "disabled" || gotClient.UpdatedAt < before {
		t.Fatalf("oauth client should be disabled exactly after owner permission denial: %#v", gotClient)
	}
	gotUnaffectedClient, err := db.OAuth.GetClient(ctx, unaffectedClient.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gotUnaffectedClient == nil || gotUnaffectedClient.Status != "active" || gotUnaffectedClient.UpdatedAt != unaffectedClient.UpdatedAt {
		t.Fatalf("unaffected oauth client should remain active exactly: %#v", gotUnaffectedClient)
	}
	gotClientPermissionIDs, err := db.OAuth.ClientPermissionIDs(ctx, client.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !int64SliceEqual(gotClientPermissionIDs, clientPermissionIDs) {
		t.Fatalf("client permission list must be preserved: got=%v want=%v", gotClientPermissionIDs, clientPermissionIDs)
	}
	if _, err := cache.GetAuthUser(ctx, target.ID); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("auth cache should still be invalidated exactly, got %v", err)
	}
	page, err := noticesvc.Service{DB: db}.ListForUser(ctx, noticesvc.CurrentUser{ID: target.ID}, noticesvc.ListParams{Type: noticesvc.TypeSystem, IncludeRead: true, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	notices := page["items"].([]model.NoticeView)
	if page["page_size"] != 3 || page["has_next"] != false || len(notices) != 3 {
		t.Fatalf("permission denial notice page mismatch: page=%#v items=%#v", page, notices)
	}
	overrideNotice := permissionNoticeByTitle(t, notices, "权限已更新：单项权限已调整")
	wantOverrideContent := "你的单项权限已被调整。\n\n权限：`profile.update.owned`\n\n说明：修改自己的角色\n\n结果：拒绝"
	if overrideNotice.Summary != "你的单项权限已被管理员调整，详情请查看通知。" ||
		overrideNotice.ContentMarkdown != wantOverrideContent ||
		overrideNotice.Type != noticesvc.TypeSystem || overrideNotice.Audience != noticesvc.AudienceTargeted ||
		overrideNotice.Level != noticesvc.LevelInfo || overrideNotice.CreatedBy != nil {
		t.Fatalf("permission override notice mismatch: %#v", overrideNotice)
	}
	grantDependencyNotice := permissionNoticeByTitle(t, notices, "第三方应用授权已自动撤销")
	wantGrantDependencyContent := "你的站点权限发生变化，以下第三方应用授权已自动撤销：\n\n" +
		"- Permission reconcile client（`permission-reconcile-client`）\n\n" +
		"这些授权包含你当前已不再拥有的权限，后续访问会失败。需要继续使用时，请在权限恢复后重新授权。"
	if grantDependencyNotice.Summary != "你的权限发生变化，1 个第三方应用授权已自动撤销。" ||
		grantDependencyNotice.ContentMarkdown != wantGrantDependencyContent ||
		grantDependencyNotice.Level != noticesvc.LevelWarning ||
		grantDependencyNotice.LinkText != "查看授权" || grantDependencyNotice.LinkURL != "/dashboard/oauth" ||
		strings.Contains(grantDependencyNotice.ContentMarkdown, unaffectedClient.ID) ||
		grantDependencyNotice.CreatedBy != nil {
		t.Fatalf("oauth grant dependency notice mismatch: %#v", grantDependencyNotice)
	}
	clientDependencyNotice := permissionNoticeByTitle(t, notices, "第三方应用已自动停用")
	wantClientDependencyContent := "你的站点权限发生变化，以下第三方应用已自动停用：\n\n" +
		"- Permission reconcile client（`permission-reconcile-client`）\n\n" +
		"这些应用申请了你当前已不再拥有的权限。请调整应用权限后重新提交审核。"
	if clientDependencyNotice.Summary != "你的权限发生变化，1 个你创建的第三方应用已自动停用。" ||
		clientDependencyNotice.ContentMarkdown != wantClientDependencyContent ||
		clientDependencyNotice.Level != noticesvc.LevelWarning ||
		clientDependencyNotice.LinkText != "查看应用" || clientDependencyNotice.LinkURL != "/dashboard/oauth" ||
		strings.Contains(clientDependencyNotice.ContentMarkdown, unaffectedClient.ID) ||
		clientDependencyNotice.CreatedBy != nil {
		t.Fatalf("oauth client dependency notice mismatch: %#v", clientDependencyNotice)
	}
}
