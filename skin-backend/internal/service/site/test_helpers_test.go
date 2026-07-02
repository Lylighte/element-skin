package site_test

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type deleteYggFailStore struct {
	redisstore.Store
	deleteCalls int
}

func (s *deleteYggFailStore) DeleteYggTokensByUser(context.Context, string) error {
	s.deleteCalls++
	return errors.New("ygg token revocation failed")
}
