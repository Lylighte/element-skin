package admin

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	noticesvc "element-skin/backend/internal/service/notice"
	permissionssvc "element-skin/backend/internal/service/permissions"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	profiles profilesvc.Service
	notices  noticesvc.Service
	perms    permissionssvc.PermissionService
	accounts accountsvc.AccountService
	auth     shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	return Handler{cfg: cfg, db: db, redis: redis, settings: settings, profiles: profilesvc.Service{DB: db, Settings: settings}, notices: noticesvc.Service{DB: db}, perms: permissionssvc.PermissionService{DB: db, Redis: redis}, accounts: accountsvc.AccountService{DB: db, Redis: redis}, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
