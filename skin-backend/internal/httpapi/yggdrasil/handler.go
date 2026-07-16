package yggdrasil

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	fallbacksvc "element-skin/backend/internal/service/fallback"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Handler struct {
	profiles profilesvc.Service
	textures texturesvc.LibraryService
	uploads  texturesvc.UploadService
	fallback fallbacksvc.Fallback
	ygg      yggpkg.Yggdrasil
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, settings settingssvc.Settings, ygg yggpkg.Yggdrasil) Handler {
	return NewWithHTTPClient(cfg, db, redis, settings, ygg, nil)
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, redis redisstore.Store, settings settingssvc.Settings, ygg yggpkg.Yggdrasil, client *http.Client) Handler {
	ygg.Redis = redis
	ygg.Settings = settings
	return Handler{
		profiles: profilesvc.Service{DB: db, Settings: settings},
		textures: texturesvc.LibraryService{DB: db, Settings: settings},
		uploads:  texturesvc.UploadService{DB: db, TexturesDir: cfg.TexturesDir},
		fallback: fallbacksvc.Fallback{DB: db, Redis: redis, Settings: settings, Client: client},
		ygg:      ygg,
	}
}
