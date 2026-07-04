package remote

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	importsvc "element-skin/backend/internal/service/imports"
)

type Handler struct {
	auth    shared.AuthFunc
	imports importsvc.RemoteYggService
}

func New(cfg config.Config, db *database.DB, auth shared.AuthFunc) Handler {
	return NewWithHTTPClient(cfg, db, auth, nil)
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, auth shared.AuthFunc, client *http.Client) Handler {
	return Handler{
		auth:    auth,
		imports: importsvc.RemoteYggService{DB: db, TexturesDir: cfg.TexturesDir, HTTPClient: client},
	}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
