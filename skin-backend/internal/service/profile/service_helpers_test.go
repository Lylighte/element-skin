package profile_test

import (
	"strings"

	"element-skin/backend/internal/database"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/testutil"
)

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}

func newProfileService(db *database.DB) profilesvc.Service {
	redis := testutil.NewMemoryRedis()
	return profilesvc.Service{DB: db, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}
