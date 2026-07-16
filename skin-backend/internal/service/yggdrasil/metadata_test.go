package yggdrasil_test

import (
	"context"
	"strings"
	"testing"

	dbfallback "element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilMetadataUsesSiteSettingsAndSigningKey(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	if err := db.Settings.Set(ctx, "site_name", "Exact Ygg"); err != nil {
		t.Fatal(err)
	}
	if err := db.Fallbacks.SaveEndpoints(ctx, []dbfallback.Endpoint{
		{Priority: 1, SessionURL: "https://session.one", AccountURL: "https://account.one", ServicesURL: "https://services.one", CacheTTL: 60, SkinDomains: "cdn.example, skins.example"},
		{Priority: 2, SessionURL: "https://session.two", AccountURL: "https://account.two", ServicesURL: "https://services.two", CacheTTL: 60, SkinDomains: "skins.example, skin.example"},
	}); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: cfg, Settings: settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}}

	meta, err := ygg.Metadata(ctx, permission.GuestActor())
	if err != nil {
		t.Fatal(err)
	}
	metaBody := meta["meta"].(map[string]any)
	links := metaBody["links"].(map[string]any)
	domains := meta["skinDomains"].([]string)
	if metaBody["serverName"] != "Exact Ygg" || links["homepage"] != "https://skin.example/root/" || links["register"] != "https://skin.example/root/register/" ||
		len(domains) != 3 || domains[0] != "cdn.example" || domains[1] != "skins.example" || domains[2] != "skin.example" {
		t.Fatalf("metadata mismatch: %#v", meta)
	}
	if publicKey := meta["signaturePublickey"].(string); !strings.Contains(publicKey, "BEGIN PUBLIC KEY") {
		t.Fatalf("metadata should expose PEM public key, got %q", publicKey)
	}
	publicKeys := meta["signaturePublickeys"].([]string)
	if len(publicKeys) != 1 || publicKeys[0] != meta["signaturePublickey"] {
		t.Fatalf("metadata plural keys=%#v, want own singular key only without cached fallbacks", publicKeys)
	}
}
