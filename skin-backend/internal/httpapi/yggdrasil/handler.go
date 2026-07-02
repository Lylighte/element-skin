package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	profilesvc "element-skin/backend/internal/service/profile"
	settingssvc "element-skin/backend/internal/service/settings"
	texturesvc "element-skin/backend/internal/service/texture"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	profiles profilesvc.Service
	textures texturesvc.LibraryService
	uploads  texturesvc.UploadService
	ygg      yggpkg.Yggdrasil
}

func New(cfg config.Config, db *database.DB, redis redisstore.Store, settings settingssvc.Settings, ygg yggpkg.Yggdrasil) Handler {
	ygg.Redis = redis
	ygg.Settings = settings
	return Handler{
		cfg:      cfg,
		db:       db,
		redis:    redis,
		settings: settings,
		profiles: profilesvc.Service{DB: db, Settings: settings},
		textures: texturesvc.LibraryService{DB: db, Settings: settings},
		uploads:  texturesvc.UploadService{DB: db, TexturesDir: cfg.TexturesDir},
		ygg:      ygg,
	}
}
