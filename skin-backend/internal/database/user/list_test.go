package user_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/testutil"
)

func TestListSearchesUserAndProfileFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := user.Store{Pool: db.Pool}
	u := testutil.CreateUser(t, db, "domain-user-list@test.com", "Password123", "DomainUserList", false)
	testutil.CreateProfile(t, db, u.ID, "domain_user_list_profile", "DomainSearchProfile")
	page, err := store.List(ctx, 1, "", "DomainSearchProfile")
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != u.ID || items[0]["email"] != "domain-user-list@test.com" || page["has_next"] != false {
		t.Fatalf("list mismatch: %#v", page)
	}
}
