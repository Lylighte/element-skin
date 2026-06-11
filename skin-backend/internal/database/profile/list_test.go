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

func TestProfileListsAdvanceCursorsWithoutRepeatingRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := profile.Store{Pool: db.Pool}
	first := testutil.CreateUser(t, db, "profile-cursor-first@test.com", "Password123", "ProfileCursorFirst", false)
	second := testutil.CreateUser(t, db, "profile-cursor-second@test.com", "Password123", "ProfileCursorSecond", false)
	for _, item := range []struct {
		userID string
		id     string
		name   string
	}{
		{first.ID, "profile_cursor_a", "CursorNeedleA"},
		{first.ID, "profile_cursor_b", "CursorNeedleB"},
		{first.ID, "profile_cursor_c", "CursorOtherC"},
		{second.ID, "profile_cursor_d", "CursorNeedleD"},
	} {
		testutil.CreateProfile(t, db, item.userID, item.id, item.name)
	}

	firstPage, err := store.ListByUser(ctx, first.ID, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	firstItems := firstPage["items"].([]map[string]any)
	firstNext := firstPage["next_key"].(map[string]any)
	if len(firstItems) != 2 ||
		firstItems[0]["id"] != "profile_cursor_a" ||
		firstItems[1]["id"] != "profile_cursor_b" ||
		firstPage["has_next"] != true ||
		firstPage["page_size"] != 2 ||
		firstNext["last_id"] != "profile_cursor_b" {
		t.Fatalf("first user page mismatch: %#v", firstPage)
	}
	secondPage, err := store.ListByUser(ctx, first.ID, 2, firstNext["last_id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	secondItems := secondPage["items"].([]map[string]any)
	if len(secondItems) != 1 ||
		secondItems[0]["id"] != "profile_cursor_c" ||
		secondPage["has_next"] != false ||
		secondPage["next_key"] != nil ||
		secondPage["page_size"] != 1 {
		t.Fatalf("second user page mismatch: %#v", secondPage)
	}

	allFirst, err := store.ListAll(ctx, 2, "", "")
	if err != nil {
		t.Fatal(err)
	}
	allFirstItems := allFirst["items"].([]map[string]any)
	allNext := allFirst["next_key"].(map[string]any)
	if len(allFirstItems) != 2 ||
		allFirstItems[0]["id"] != "profile_cursor_a" ||
		allFirstItems[1]["id"] != "profile_cursor_b" ||
		allNext["last_id"] != "profile_cursor_b" {
		t.Fatalf("first admin page mismatch: %#v", allFirst)
	}
	allSecond, err := store.ListAll(ctx, 2, allNext["last_id"].(string), "")
	if err != nil {
		t.Fatal(err)
	}
	allSecondItems := allSecond["items"].([]map[string]any)
	if len(allSecondItems) != 2 ||
		allSecondItems[0]["id"] != "profile_cursor_c" ||
		allSecondItems[1]["id"] != "profile_cursor_d" ||
		allSecondItems[1]["owner_email"] != second.Email ||
		allSecondItems[1]["owner_display_name"] != second.DisplayName ||
		allSecond["has_next"] != false {
		t.Fatalf("second admin page mismatch: %#v", allSecond)
	}

	searchFirst, err := store.ListAll(ctx, 1, "", "Needle")
	if err != nil {
		t.Fatal(err)
	}
	searchFirstItems := searchFirst["items"].([]map[string]any)
	searchNext := searchFirst["next_key"].(map[string]any)
	if len(searchFirstItems) != 1 ||
		searchFirstItems[0]["id"] != "profile_cursor_a" ||
		searchNext["last_id"] != "profile_cursor_a" {
		t.Fatalf("first searched page mismatch: %#v", searchFirst)
	}
	searchSecond, err := store.ListAll(ctx, 2, searchNext["last_id"].(string), "Needle")
	if err != nil {
		t.Fatal(err)
	}
	searchSecondItems := searchSecond["items"].([]map[string]any)
	if len(searchSecondItems) != 2 ||
		searchSecondItems[0]["id"] != "profile_cursor_b" ||
		searchSecondItems[1]["id"] != "profile_cursor_d" ||
		searchSecond["has_next"] != false ||
		searchSecond["next_key"] != nil {
		t.Fatalf("second searched page mismatch: %#v", searchSecond)
	}
}
