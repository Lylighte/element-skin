package settings_test

import (
	"testing"

	"element-skin/backend/internal/service/settings"
)

func TestSettingDefaultsAndGroupsExposeCurrentFrontendGroups(t *testing.T) {
	for _, key := range []string{"site_name", "allow_register", "smtp_port", "fallback_strategy"} {
		if _, ok := settings.SettingDefaults[key]; !ok {
			t.Fatalf("missing setting default %q", key)
		}
	}
	if _, exists := settings.SettingDefaults["easter_eggs_enabled"]; exists {
		t.Fatal("structured easter egg state must not have a string setting default")
	}
}
