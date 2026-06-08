package admin

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
)

type Handler struct {
	cfg   config.Config
	db    *database.DB
	redis redisstore.Store
	auth  shared.AuthFunc
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	redis := redisstore.Store(redisstore.NewMemoryStore())
	return NewWithRedis(cfg, db, redis, auth)
}

func NewWithRedis(cfg config.Config, db *database.DB, redis redisstore.Store, auth shared.AuthFunc) Handler {
	return Handler{cfg: cfg, db: db, redis: redis, auth: auth}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next, true)
}
