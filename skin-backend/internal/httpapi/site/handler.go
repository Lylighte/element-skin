package site

import (
	"net/http"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	accountsvc "element-skin/backend/internal/service/account"
	publicsitesvc "element-skin/backend/internal/service/publicsite"
	settingssvc "element-skin/backend/internal/service/settings"
	sitepkg "element-skin/backend/internal/service/site"
)

type Handler struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	site     sitepkg.Site
	accounts accountsvc.AccountService
	public   publicsitesvc.Service
	settings settingssvc.Settings
	auth     shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, svc, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	svc.Redis = redis
	svc.Settings = settings
	accounts := accountsvc.AccountService{DB: db, Redis: redis}
	public := publicsitesvc.Service{
		DB:       db,
		Redis:    redis,
		Settings: settings,
		SiteURL:  cfg.SiteURL,
		APIURL:   cfg.APIURL,
		CacheTTL: time.Duration(cfg.PublicCacheTTL) * time.Second,
	}
	return Handler{cfg: cfg, db: db, redis: redis, site: svc, accounts: accounts, public: public, settings: settings, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
