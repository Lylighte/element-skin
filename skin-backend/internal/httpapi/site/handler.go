package site

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	sitepkg "element-skin/backend/internal/service/site"
)

type Handler struct {
	cfg   config.Config
	db    *database.DB
	redis redisstore.Store
	site  sitepkg.Site
	auth  shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	if svc.Redis == nil {
		svc.Redis = redis
	}
	return NewWithRedis(cfg, db, redis, svc, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, svc sitepkg.Site, auth shared.AuthFunc) Handler {
	if svc.Redis == nil {
		svc.Redis = redis
	}
	return Handler{cfg: cfg, db: db, redis: redis, site: svc, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, false)
}
