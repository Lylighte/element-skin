package yggdrasil_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilMetadataUsesSiteSettingsAndSigningKey(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example/root"
	cfg.FallbackDomains = []string{"cdn.example"}
	if err := db.Settings.Set(ctx, "site_name", "Exact Ygg"); err != nil {
		t.Fatal(err)
	}
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: cfg}

	meta, err := ygg.Metadata(ctx)
	if err != nil {
		t.Fatal(err)
	}
	metaBody := meta["meta"].(map[string]any)
	links := metaBody["links"].(map[string]any)
	domains := meta["skinDomains"].([]string)
	if metaBody["serverName"] != "Exact Ygg" || links["homepage"] != "https://skin.example/root/" || links["register"] != "https://skin.example/root/register/" ||
		len(domains) != 2 || domains[0] != "cdn.example" || domains[1] != "skin.example" {
		t.Fatalf("metadata mismatch: %#v", meta)
	}
	if publicKey := meta["signaturePublickey"].(string); !strings.Contains(publicKey, "BEGIN PUBLIC KEY") {
		t.Fatalf("metadata should expose PEM public key, got %q", publicKey)
	}
}
