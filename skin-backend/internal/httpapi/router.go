package httpapi

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
	yggpkg "element-skin/backend/internal/service/yggdrasil"
)

type Router struct {
	cfg      config.Config
	db       *database.DB
	redis    redisstore.Store
	settings settingssvc.Settings
	ygg      yggpkg.Yggdrasil
	mux      *http.ServeMux
}

func NewRouter(cfg config.Config, db *database.DB, ygg yggpkg.Yggdrasil) http.Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewRouterWithRedis(cfg, db, redis, ygg)
}

func NewRouterWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, ygg yggpkg.Yggdrasil) http.Handler {
	settings := settingssvc.Settings{DB: db, Redis: redis}
	ygg.Redis = redis
	ygg.Settings = settings
	r := &Router{cfg: cfg, db: db, redis: redis, settings: settings, ygg: ygg, mux: http.NewServeMux()}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.cfg.APIURL != "" {
		w.Header().Set("X-Authlib-Injector-API-Location", r.cfg.APIURL)
	}
	r.mux.ServeHTTP(w, req)
}

func (r *Router) handle(pattern string, h http.HandlerFunc) {
	r.mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		h(w, req)
	})
}
