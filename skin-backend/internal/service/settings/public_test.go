package settings_test

import (
	"context"
	"errors"
	"testing"

	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsPublicUsesSavedValuesAndPrimaryFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if err := settings.SaveGroup(ctx, "site", map[string]any{
		"site_name":      "Exact Skin",
		"allow_register": false,
		"require_invite": true,
		"site_url":       "skin.example.com/root/",
		"api_url":        "https://api.example.com/skinapi/",
	}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "fallback", map[string]any{
		"fallbacks": []any{map[string]any{
			"priority":     1,
			"session_url":  "https://session.example",
			"account_url":  "https://account.example",
			"services_url": "https://services.example",
		}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := settings.SaveGroup(ctx, "easter_eggs", map[string]any{
		"easter_eggs_enabled": []any{"april-fools", "children-day", "mid-autumn"},
	}); err != nil {
		t.Fatal(err)
	}

	public, err := settings.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
	if err != nil {
		t.Fatal(err)
	}
	status := public["mojang_status_urls"].(map[string]any)
	easterEggs := public["easter_eggs"].(map[string]any)
	enabled := easterEggs["enabled"].([]string)
	if public["site_name"] != "Exact Skin" || public["allow_register"] != false || public["require_invite"] != true ||
		public["site_url"] != "https://skin.example.com/root" || public["api_url"] != "https://api.example.com/skinapi" ||
		status["session"] != "https://session.example" || status["account"] != "https://account.example" || status["services"] != "https://services.example" ||
		len(enabled) != 3 || enabled[0] != "april-fools" || enabled[1] != "children-day" || enabled[2] != "mid-autumn" {
		t.Fatalf("unexpected public settings: %#v", public)
	}
}

func TestSettingsPublicPropagatesDatabaseErrors(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	db.Close()
	if out, err := settings.Public(context.Background(), "http://cfg-site.local/", "http://cfg-api.local/"); err == nil || out != nil {
		t.Fatalf("closed database should fail instead of returning partial public settings: out=%#v err=%v", out, err)
	}
}

func TestSettingsPublicPropagatesEachSettingReadErrorExactly(t *testing.T) {
	keys := []string{
		"site_name",
		"allow_register",
		"require_invite",
		"site_url",
		"api_url",
		"site_subtitle",
		"enable_skin_library",
		"email_verify_enabled",
		"footer_text",
		"filing_icp",
		"filing_icp_link",
		"filing_mps",
		"filing_mps_link",
	}
	for index, key := range keys {
		t.Run(key, func(t *testing.T) {
			db, _ := testutil.NewTestApp(t)
			ctx := context.Background()
			errSentinel := errors.New("public setting read failed at " + key)
			cache := &indexedSettingFailureStore{Store: testutil.NewMemoryRedis(), failAt: index + 1, err: errSentinel}
			svc := settings.Settings{DB: db, Redis: cache}

			out, err := svc.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
			if out != nil || !errors.Is(err, errSentinel) {
				t.Fatalf("Public() at %s returned out=%#v err=%v want nil exact sentinel", key, out, errSentinel)
			}
			if cache.calls != index+1 || cache.keys[index] != key {
				t.Fatalf("Public() setting access trace=%#v calls=%d want failure at %s", cache.keys, cache.calls, key)
			}
		})
	}
}

func TestSettingsPublicPropagatesEasterEggStoreErrorExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE enabled_easter_eggs`); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
	if out != nil || err == nil {
		t.Fatalf("Public() easter egg store failure out=%#v err=%v, want nil and database error", out, err)
	}
}

func TestSettingsPublicPropagatesFallbackEndpointErrorExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	svc := settings.Settings{DB: db, Redis: testutil.NewMemoryRedis()}
	if _, err := db.Pool.Exec(ctx, `DROP TABLE fallback_endpoints CASCADE`); err != nil {
		t.Fatal(err)
	}

	out, err := svc.Public(ctx, "http://cfg-site.local/", "http://cfg-api.local/")
	if out != nil || err == nil {
		t.Fatalf("Public() fallback dependency error out=%#v err=%v want nil error", out, err)
	}
}

type indexedSettingFailureStore struct {
	redisstore.Store
	failAt int
	calls  int
	keys   []string
	err    error
}

func (s *indexedSettingFailureStore) GetSetting(ctx context.Context, key string) (string, error) {
	s.calls++
	s.keys = append(s.keys, key)
	if s.calls == s.failAt {
		return "", s.err
	}
	return s.Store.GetSetting(ctx, key)
}
