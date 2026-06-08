package httpapi

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

type Router struct {
	cfg  config.Config
	db   *database.DB
	site service.Site
	ygg  service.Yggdrasil
	mux  *http.ServeMux
}

var MicrosoftImportStates = util.NewInMemoryStateStore()

type ctxKey string

const userIDKey ctxKey = "user_id"
const adminKey ctxKey = "admin"

func NewRouter(cfg config.Config, db *database.DB, site service.Site, ygg service.Yggdrasil) http.Handler {
	r := &Router{cfg: cfg, db: db, site: site, ygg: ygg, mux: http.NewServeMux()}
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
