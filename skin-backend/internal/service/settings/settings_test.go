package settings_test

import (
	"testing"

	"element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func TestSettingsStoresDatabaseDependency(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	settings := settings.Settings{DB: db}
	if settings.DB != db {
		t.Fatalf("Settings should retain DB dependency")
	}
}
