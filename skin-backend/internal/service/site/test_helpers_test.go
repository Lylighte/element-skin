package site_test

import (
	"errors"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/service/site"
	"element-skin/backend/internal/testutil"
	"element-skin/backend/internal/util"
)

func newSiteService(db *database.DB, cfg config.Config) site.Site {
	redis := testutil.NewMemoryRedis()
	return site.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settingssvc.Settings{DB: db, Redis: redis}}
}

func httpError(err error, status int, detail string) bool {
	var httpErr util.HTTPError
	return errors.As(err, &httpErr) && httpErr.Status == status && httpErr.Detail == detail
}

func closedPoolError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "closed pool")
}
