package remote

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
)

type Handler struct {
	cfg        config.Config
	db         *database.DB
	auth       shared.AuthFunc
	httpClient *http.Client
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	return Handler{cfg: cfg, db: db, auth: auth}
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, auth shared.AuthFunc, client *http.Client) Handler {
	return Handler{cfg: cfg, db: db, auth: auth, httpClient: client}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
