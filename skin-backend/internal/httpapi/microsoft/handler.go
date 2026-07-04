package microsoft

import (
	"net/http"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Handler struct {
	cfg        config.Config
	db         *database.DB
	settings   settingssvc.Settings
	auth       shared.AuthFunc
	states     redisstore.Store
	httpClient *http.Client
}

func New(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store) Handler {
	return Handler{cfg: cfg, db: db, settings: settings, auth: auth, states: states}
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store, client *http.Client) Handler {
	return Handler{cfg: cfg, db: db, settings: settings, auth: auth, states: states, httpClient: client}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
