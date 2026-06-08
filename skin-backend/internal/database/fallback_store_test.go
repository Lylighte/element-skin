package database_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestFallbackStoreEndpointsDomainsAndWhitelistExactState(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.SaveFallbackEndpoints(ctx, []database.FallbackEndpoint{
		{
			Priority: 2, SessionURL: "https://session-two", AccountURL: "https://account-two", ServicesURL: "https://services-two",
			CacheTTL: 120, SkinDomains: "skins.example, cdn.example", EnableProfile: false, EnableHasJoined: true, EnableWhitelist: true, Note: "second",
		},
		{
			Priority: 1, SessionURL: "https://session-one", AccountURL: "https://account-one", ServicesURL: "https://services-one",
			CacheTTL: 60, SkinDomains: "cdn.example, textures.example", EnableProfile: true, EnableHasJoined: false, EnableWhitelist: false, Note: "first",
		},
	}); err != nil {
		t.Fatal(err)
	}

	endpoints, err := db.ListFallbackEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 || endpoints[0]["priority"] != 1 || endpoints[0]["session_url"] != "https://session-one" ||
		endpoints[0]["enable_profile"] != true || endpoints[0]["enable_hasjoined"] != false ||
		endpoints[1]["priority"] != 2 || endpoints[1]["note"] != "second" || endpoints[1]["enable_whitelist"] != true {
		t.Fatalf("fallback endpoints not listed in exact priority order: %#v", endpoints)
	}
	primary, err := db.GetPrimaryFallbackEndpoint(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if primary["session_url"] != "https://session-one" || primary["account_url"] != "https://account-one" || primary["services_url"] != "https://services-one" {
		t.Fatalf("primary endpoint mismatch: %#v", primary)
	}

	domains, err := db.CollectFallbackSkinDomains(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(domains) != 3 || domains[0] != "cdn.example" || domains[1] != "textures.example" || domains[2] != "skins.example" {
		t.Fatalf("domains should be deduplicated by endpoint order: %#v", domains)
	}

	endpointID := endpoints[1]["id"].(int)
	if ok, err := db.IsUserInWhitelist(ctx, "Steve", endpointID); err != nil || ok {
		t.Fatalf("Steve should not be whitelisted yet: ok=%v err=%v", ok, err)
	}
	if err := db.AddWhitelistUser(ctx, "Steve", endpointID); err != nil {
		t.Fatal(err)
	}
	if err := db.AddWhitelistUser(ctx, "Steve", endpointID); err != nil {
		t.Fatal(err)
	}
	if ok, err := db.IsUserInWhitelist(ctx, "Steve", endpointID); err != nil || !ok {
		t.Fatalf("Steve should be whitelisted: ok=%v err=%v", ok, err)
	}
	users, err := db.ListWhitelistUsers(ctx, endpointID)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0]["username"] != "Steve" || users[0]["created_at"].(int64) <= 0 {
		t.Fatalf("whitelist list mismatch: %#v", users)
	}
	if err := db.RemoveWhitelistUser(ctx, "Steve", endpointID); err != nil {
		t.Fatal(err)
	}
	if users, err := db.ListWhitelistUsers(ctx, endpointID); err != nil || len(users) != 0 {
		t.Fatalf("whitelist should be empty after removal: users=%#v err=%v", users, err)
	}
}
