package publicsite_test

import (
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	publicsvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSecurityPolicyToggleInvalidatesAndRebuildsPublicSettings(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := t.Context()
	cache := redisstore.NewMemoryStore()
	settings := settingssvc.Settings{DB: db, Redis: cache}
	svc := publicsvc.Service{DB: db, Redis: cache, Settings: settings, SiteURL: "http://test", APIURL: "http://test/api", CacheTTL: time.Minute}
	initial, err := svc.PublicSettings(ctx, permission.GuestActor())
	if err != nil || initial["enable_strong_password_check"] != false {
		t.Fatalf("initial public policy=%#v err=%v", initial, err)
	}
	for _, enabled := range []bool{true, false} {
		if err := settings.SaveGroupAndInvalidate(ctx, "security", map[string]any{"enable_strong_password_check": enabled}); err != nil {
			t.Fatal(err)
		}
		if _, err := cache.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("security update did not invalidate public cache: %v", err)
		}
		updated, err := svc.PublicSettings(ctx, permission.GuestActor())
		if err != nil || updated["enable_strong_password_check"] != enabled {
			t.Fatalf("updated public policy=%#v; want %t err=%v", updated, enabled, err)
		}
		cached, err := cache.GetPublicSettings(ctx)
		if err != nil || cached["enable_strong_password_check"] != enabled {
			t.Fatalf("cached policy=%#v; want %t err=%v", cached, enabled, err)
		}
	}
}
