package database_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestUserStoresUpdateListBanInviteAndDeleteExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.CreateInvite(ctx, "invite_once", 1, "launch invite"); err != nil {
		t.Fatal(err)
	}
	hash := "hash"
	user := model.User{ID: "user_store_a", Email: "user-store-a@test.com", Password: hash, IsAdmin: false, DisplayName: "UserStoreA"}
	profile := model.Profile{ID: "user_profile_a", UserID: user.ID, Name: "UserProfileA", TextureModel: "default"}
	if err := db.CreateUserWithProfile(ctx, user, profile, "invite_once", user.Email); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateUserWithProfile(ctx, model.User{ID: "user_store_b", Email: "user-store-b@test.com", Password: hash, DisplayName: "UserStoreB"}, model.Profile{ID: "user_profile_b", UserID: "user_store_b", Name: "SearchByProfile", TextureModel: "slim"}, "invite_once", "second"); !errors.Is(err, database.ErrInviteExhausted) {
		t.Fatalf("exhausted invite should fail with ErrInviteExhausted, got %v", err)
	}

	invite, err := db.GetInvite(ctx, "invite_once")
	if err != nil {
		t.Fatal(err)
	}
	if invite == nil || invite.UsedCount != 1 || invite.UsedBy == nil || *invite.UsedBy != user.Email || invite.TotalUses == nil || *invite.TotalUses != 1 || invite.Note != "launch invite" {
		t.Fatalf("invite state did not update exactly: %#v", invite)
	}
	if count, err := db.CountUsers(ctx); err != nil || count != 1 {
		t.Fatalf("expected one created user, count=%d err=%v", count, err)
	}
	if taken, err := db.IsDisplayNameTaken(ctx, "UserStoreA", ""); err != nil || !taken {
		t.Fatalf("display name should be taken: taken=%v err=%v", taken, err)
	}
	if taken, err := db.IsDisplayNameTaken(ctx, "UserStoreA", user.ID); err != nil || taken {
		t.Fatalf("display name excluded by same user should be available: taken=%v err=%v", taken, err)
	}

	avatar := "avatar_hash"
	if err := db.UpdateUser(ctx, user.ID, map[string]any{
		"email":              "renamed@test.com",
		"display_name":       "RenamedUser",
		"preferred_language": "en_US",
		"avatar_hash":        avatar,
		"ignored":            "must-not-persist",
	}); err != nil {
		t.Fatal(err)
	}
	updated, err := db.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Email != "renamed@test.com" || updated.DisplayName != "RenamedUser" || updated.PreferredLanguage != "en_US" ||
		updated.AvatarHash == nil || *updated.AvatarHash != "avatar_hash" {
		t.Fatalf("user updates did not persist exactly: %#v", updated)
	}
	if nextAdmin, err := db.ToggleAdmin(ctx, user.ID); err != nil || !nextAdmin {
		t.Fatalf("toggle admin should turn user into admin: next=%v err=%v", nextAdmin, err)
	}
	banUntil := database.NowMS() + 60_000
	if err := db.BanUser(ctx, user.ID, banUntil); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.IsBanned(ctx, user.ID); err != nil || !banned {
		t.Fatalf("user should be banned: banned=%v err=%v", banned, err)
	}
	if err := db.UnbanUser(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	if banned, err := db.IsBanned(ctx, user.ID); err != nil || banned {
		t.Fatalf("user should be unbanned: banned=%v err=%v", banned, err)
	}

	list, err := db.ListUsers(ctx, 1, "", "UserProfileA")
	if err != nil {
		t.Fatal(err)
	}
	items := list["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != user.ID || items[0]["email"] != "renamed@test.com" ||
		items[0]["display_name"] != "RenamedUser" || items[0]["is_admin"] != true || items[0]["preferred_language"] != "en_US" ||
		list["has_next"] != false || list["next_key"] != nil {
		t.Fatalf("unexpected user list by profile query: %#v", list)
	}

	if err := db.AddToken(ctx, model.Token{AccessToken: "delete_user_token", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: database.NowMS()}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddRefreshToken(ctx, "delete_user_refresh", user.ID, database.NowMS()+60_000, database.NowMS()); err != nil {
		t.Fatal(err)
	}
	if err := db.AddTextureToLibrary(ctx, user.ID, "delete_user_texture", "skin", "delete texture", false, "default"); err != nil {
		t.Fatal(err)
	}
	deleted, err := db.DeleteUser(ctx, user.ID)
	if err != nil || !deleted {
		t.Fatalf("DeleteUser failed: deleted=%v err=%v", deleted, err)
	}
	if gone, err := db.GetUserByID(ctx, user.ID); err != nil || gone != nil {
		t.Fatalf("user should be deleted: user=%#v err=%v", gone, err)
	}
	if tok, err := db.GetToken(ctx, "delete_user_token"); err != nil || tok != nil {
		t.Fatalf("DeleteUser should delete ygg token: token=%#v err=%v", tok, err)
	}
	if refresh, err := db.GetRefreshToken(ctx, "delete_user_refresh"); err != nil || refresh != nil {
		t.Fatalf("DeleteUser should delete refresh token: refresh=%#v err=%v", refresh, err)
	}
	if texture, err := db.GetTextureInfo(ctx, user.ID, "delete_user_texture", "skin"); err != nil || texture != nil {
		t.Fatalf("DeleteUser should delete user texture row: texture=%#v err=%v", texture, err)
	}
}

func TestTokenSessionRefreshSettingAndVerificationStores(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "token-store@test.com", "Password123", "TokenStore", false)
	profile := testutil.CreateProfile(t, db, user.ID, "token_profile", "TokenProfile")

	if err := db.AddToken(ctx, model.Token{AccessToken: "old_access", ClientToken: "client", UserID: user.ID, ProfileID: &profile.ID, CreatedAt: 10}); err != nil {
		t.Fatal(err)
	}
	if err := db.AddToken(ctx, model.Token{AccessToken: "new_access", ClientToken: "client", UserID: user.ID, ProfileID: nil, CreatedAt: 20}); err != nil {
		t.Fatal(err)
	}
	if err := db.CleanupTokens(ctx, user.ID, 15, 1); err != nil {
		t.Fatal(err)
	}
	if old, err := db.GetToken(ctx, "old_access"); err != nil || old != nil {
		t.Fatalf("old token should be cleaned by cutoff: token=%#v err=%v", old, err)
	}
	if newer, err := db.GetToken(ctx, "new_access"); err != nil || newer == nil || newer.ClientToken != "client" || newer.ProfileID != nil {
		t.Fatalf("new token should remain exactly: token=%#v err=%v", newer, err)
	}

	ip := "127.0.0.1"
	if err := db.AddSession(ctx, model.Session{ServerID: "server", AccessToken: "new_access", IP: &ip, CreatedAt: 100}); err != nil {
		t.Fatal(err)
	}
	if err := db.ReplaceSession(ctx, model.Session{ServerID: "server", AccessToken: "replacement_access", IP: nil, CreatedAt: 200}); err != nil {
		t.Fatal(err)
	}
	session, err := db.GetSession(ctx, "server")
	if err != nil {
		t.Fatal(err)
	}
	if session == nil || session.AccessToken != "replacement_access" || session.IP != nil || session.CreatedAt != 200 {
		t.Fatalf("replacement session mismatch: %#v", session)
	}

	if err := db.AddRefreshToken(ctx, "refresh_a", user.ID, 1000, 100); err != nil {
		t.Fatal(err)
	}
	consumed, err := db.ConsumeRefreshToken(ctx, "refresh_a")
	if err != nil {
		t.Fatal(err)
	}
	if consumed["token_hash"] != "refresh_a" || consumed["user_id"] != user.ID || consumed["expires_at"] != int64(1000) || consumed["created_at"] != int64(100) {
		t.Fatalf("refresh consume returned wrong data: %#v", consumed)
	}
	if again, err := db.ConsumeRefreshToken(ctx, "refresh_a"); err != nil || again != nil {
		t.Fatalf("refresh token should be single-use: %#v err=%v", again, err)
	}

	if err := db.SetSetting(ctx, "bool_setting", true); err != nil {
		t.Fatal(err)
	}
	if err := db.SetSetting(ctx, "int_setting", "42"); err != nil {
		t.Fatal(err)
	}
	group, err := db.GetSettingsGroup(ctx, map[string]string{"bool_setting": "false", "missing_bool": "false", "int_setting": "0"})
	if err != nil {
		t.Fatal(err)
	}
	if group["bool_setting"] != true || group["missing_bool"] != false || group["int_setting"] != "42" {
		t.Fatalf("settings group coercion mismatch: %#v", group)
	}
	if n, err := db.SettingInt(ctx, "int_setting", 7); err != nil || n != 42 {
		t.Fatalf("SettingInt should parse stored value: n=%d err=%v", n, err)
	}
	if n, err := db.SettingInt(ctx, "missing_int", 7); err != nil || n != 7 {
		t.Fatalf("SettingInt should use fallback: n=%d err=%v", n, err)
	}

	if err := db.CreateVerificationCode(ctx, "verify@test.com", "123456", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, expiresAt, ok, err := db.GetVerificationCode(ctx, "verify@test.com", "register")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || code != "123456" || expiresAt <= database.NowMS() {
		t.Fatalf("verification code mismatch: code=%q expires=%d ok=%v", code, expiresAt, ok)
	}
	if err := db.CreateVerificationCode(ctx, "verify@test.com", "654321", "register", 300); err != nil {
		t.Fatal(err)
	}
	code, _, ok, err = db.GetVerificationCode(ctx, "verify@test.com", "register")
	if err != nil || !ok || code != "654321" {
		t.Fatalf("verification upsert should replace code: code=%q ok=%v err=%v", code, ok, err)
	}
	if err := db.DeleteVerificationCode(ctx, "verify@test.com", "register"); err != nil {
		t.Fatal(err)
	}
	if code, _, ok, err := db.GetVerificationCode(ctx, "verify@test.com", "register"); err != nil || ok || code != "" {
		t.Fatalf("verification code should be deleted: code=%q ok=%v err=%v", code, ok, err)
	}
}
