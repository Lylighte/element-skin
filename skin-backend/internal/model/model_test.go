package model

import "testing"

func TestModelStructsPreserveExactFields(t *testing.T) {
	bannedUntil := int64(123)
	avatar := "avatar"
	user := User{ID: "uid", Email: "email", Password: "hash", PreferredLanguage: "zh_CN", DisplayName: "User", BannedUntil: &bannedUntil, AvatarHash: &avatar}
	if user.ID != "uid" || user.Email != "email" || user.Password != "hash" || user.PreferredLanguage != "zh_CN" || user.DisplayName != "User" ||
		user.BannedUntil == nil || *user.BannedUntil != 123 || user.AvatarHash == nil || *user.AvatarHash != "avatar" {
		t.Fatalf("User fields mismatch: %#v", user)
	}

	skin := "skin"
	cape := "cape"
	profile := Profile{ID: "pid", UserID: "uid", Name: "Role", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape}
	if profile.ID != "pid" || profile.UserID != "uid" || profile.Name != "Role" || profile.TextureModel != "slim" ||
		profile.SkinHash == nil || *profile.SkinHash != "skin" || profile.CapeHash == nil || *profile.CapeHash != "cape" {
		t.Fatalf("Profile fields mismatch: %#v", profile)
	}

	tokenProfile := "pid"
	token := Token{AccessToken: "access", ClientToken: "client", UserID: "uid", ProfileID: &tokenProfile, CreatedAt: 456}
	if token.AccessToken != "access" || token.ClientToken != "client" || token.UserID != "uid" || token.ProfileID == nil || *token.ProfileID != "pid" || token.CreatedAt != 456 {
		t.Fatalf("Token fields mismatch: %#v", token)
	}

	ip := "127.0.0.1"
	session := Session{ServerID: "server", AccessToken: "access", IP: &ip, CreatedAt: 789}
	if session.ServerID != "server" || session.AccessToken != "access" || session.IP == nil || *session.IP != "127.0.0.1" || session.CreatedAt != 789 {
		t.Fatalf("Session fields mismatch: %#v", session)
	}

	createdAt := int64(101)
	usedBy := "email"
	totalUses := 3
	invite := Invite{Code: "code", CreatedAt: &createdAt, UsedBy: &usedBy, TotalUses: &totalUses, UsedCount: 2, Note: "note"}
	if invite.Code != "code" || invite.CreatedAt == nil || *invite.CreatedAt != 101 || invite.UsedBy == nil || *invite.UsedBy != "email" ||
		invite.TotalUses == nil || *invite.TotalUses != 3 || invite.UsedCount != 2 || invite.Note != "note" {
		t.Fatalf("Invite fields mismatch: %#v", invite)
	}
}
