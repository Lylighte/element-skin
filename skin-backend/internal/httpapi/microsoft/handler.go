package microsoft

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	importsvc "element-skin/backend/internal/service/imports"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Handler struct {
	cfg      config.Config
	settings settingssvc.Settings
	auth     shared.AuthFunc
	states   redisstore.Store
	imports  importsvc.ImportService
}

func New(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store) Handler {
	return NewWithHTTPClient(cfg, db, settings, auth, states, nil)
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store, client *http.Client) Handler {
	return Handler{
		cfg:      cfg,
		settings: settings,
		auth:     auth,
		states:   states,
		imports:  importsvc.ImportService{DB: db, TexturesDir: cfg.TexturesDir, HTTPClient: client},
	}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
