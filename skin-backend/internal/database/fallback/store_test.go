package fallback_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/testutil"
)

func TestStoreEndpointsDomainsAndWhitelist(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := fallback.Store{Pool: db.Pool}
	if err := store.SaveEndpoints(ctx, []fallback.Endpoint{
		{Priority: 2, SessionURL: "https://session-two", AccountURL: "https://account-two", ServicesURL: "https://services-two", CacheTTL: 120, SkinDomains: "skins.example, cdn.example", EnableWhitelist: true, Note: "second"},
		{Priority: 1, SessionURL: "https://session-one", AccountURL: "https://account-one", ServicesURL: "https://services-one", CacheTTL: 60, SkinDomains: "cdn.example, textures.example", EnableProfile: true, Note: "first"},
	}); err != nil {
		t.Fatal(err)
	}
	endpoints, err := store.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(endpoints) != 2 || endpoints[0]["priority"] != 1 || endpoints[0]["session_url"] != "https://session-one" || endpoints[1]["enable_whitelist"] != true {
		t.Fatalf("endpoint list mismatch: %#v", endpoints)
	}
	primary, err := store.PrimaryEndpoint(ctx)
	if err != nil || primary["services_url"] != "https://services-one" {
		t.Fatalf("primary mismatch: primary=%#v err=%v", primary, err)
	}
	domains, err := store.CollectSkinDomains(ctx)
	if err != nil || len(domains) != 3 || domains[0] != "cdn.example" || domains[1] != "textures.example" || domains[2] != "skins.example" {
		t.Fatalf("domains mismatch: domains=%#v err=%v", domains, err)
	}
	endpointID := endpoints[1]["id"].(int)
	if ok, err := store.IsUserInWhitelist(ctx, "Alex", endpointID); err != nil || ok {
		t.Fatalf("user should not be whitelisted: ok=%v err=%v", ok, err)
	}
	if err := store.AddWhitelistUser(ctx, "Alex", endpointID); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.IsUserInWhitelist(ctx, "Alex", endpointID); err != nil || !ok {
		t.Fatalf("user should be whitelisted: ok=%v err=%v", ok, err)
	}
	users, err := store.ListWhitelistUsers(ctx, endpointID)
	if err != nil || len(users) != 1 || users[0]["username"] != "Alex" {
		t.Fatalf("whitelist list mismatch: users=%#v err=%v", users, err)
	}
	if err := store.RemoveWhitelistUser(ctx, "Alex", endpointID); err != nil {
		t.Fatal(err)
	}
}
