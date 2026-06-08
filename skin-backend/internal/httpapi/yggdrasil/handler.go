package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	site     sitepkg.Site
	ygg      yggpkg.Yggdrasil
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, settings settingssvc.Settings, ygg yggpkg.Yggdrasil) Handler {
	ygg.Redis = redis
	ygg.Settings = settings
	return Handler{cfg: cfg, db: db, redis: redis, settings: settings, site: sitepkg.Site{DB: db, Cfg: cfg, Redis: redis, Settings: settings}, ygg: ygg}
}
