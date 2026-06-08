package site_test

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
)

func newSiteService(db *database.DB, cfg config.Config) site.Site {
	redis := testutil.NewMemoryRedis()
	return site.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}
