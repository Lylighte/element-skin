package profile_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/testutil"
)

func TestListByUserAndListAllPaginateExactFields(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := profile.Store{Pool: db.Pool}
	first := testutil.CreateUser(t, db, "domain-profile-list-first@test.com", "Password123", "DomainProfileListFirst", false)
	second := testutil.CreateUser(t, db, "domain-profile-list-second@test.com", "Password123", "DomainProfileListSecond", false)
	testutil.CreateProfile(t, db, first.ID, "domain_profile_list_a", "DomainProfileListA")
	testutil.CreateProfile(t, db, second.ID, "domain_profile_list_b", "DomainProfileListB")
	userPage, err := store.ListByUser(ctx, first.ID, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	userItems := userPage["items"].([]map[string]any)
	if len(userItems) != 1 || userItems[0]["id"] != "domain_profile_list_a" || userPage["has_next"] != false {
		t.Fatalf("user page mismatch: %#v", userPage)
	}
	adminPage, err := store.ListAll(ctx, 1, "", "DomainProfileList")
	if err != nil {
		t.Fatal(err)
	}
	adminItems := adminPage["items"].([]map[string]any)
	if len(adminItems) != 1 || adminItems[0]["owner_email"] != "domain-profile-list-first@test.com" || adminPage["has_next"] != true {
		t.Fatalf("admin page mismatch: %#v", adminPage)
	}
}
