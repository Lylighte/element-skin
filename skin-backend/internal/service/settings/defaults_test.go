package settings_test

import (
	"testing"

	"element-skin/backend/internal/service/settings"
)

func TestSettingDefaultsAndGroupsExposeCurrentFrontendGroups(t *testing.T) {
	for _, key := range []string{"site_name", "allow_register", "smtp_port", "fallback_strategy", "easter_eggs_enabled"} {
		if _, ok := settings.SettingDefaults[key]; !ok {
			t.Fatalf("missing setting default %q", key)
		}
	}
	if got := settings.SettingDefaults["easter_eggs_enabled"]; got != `[]` {
		t.Fatalf("easter eggs should default to disabled, got %s", got)
	}
}
