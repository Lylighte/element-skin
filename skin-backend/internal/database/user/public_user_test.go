package user_test

import (
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/model"
)

func TestPublicUserDoesNotExposePassword(t *testing.T) {
	u := model.User{
		ID:                "user-id",
		Email:             "user@test.com",
		Password:          "secret-hash",
		PreferredLanguage: "zh_CN",
		DisplayName:       "Public User",
	}

	body := user.PublicUser(u)
	if body["id"] != u.ID || body["email"] != u.Email || body["display_name"] != u.DisplayName || body["preferred_language"] != "zh_CN" {
		t.Fatalf("public user body mismatch: %#v", body)
	}
	if _, ok := body["password"]; ok {
		t.Fatalf("public user body must not expose password: %#v", body)
	}
	if _, ok := body["is_admin"]; ok {
		t.Fatalf("public user body must not expose legacy admin flag: %#v", body)
	}
}
