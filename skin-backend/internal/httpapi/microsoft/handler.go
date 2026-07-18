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
	auth     shared.AuthFunc
	workflow importsvc.MicrosoftImportWorkflow
}

func New(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store) Handler {
	return NewWithHTTPClient(cfg, db, settings, auth, states, nil)
}

func NewWithHTTPClient(cfg config.Config, db *database.DB, settings settingssvc.Settings, auth shared.AuthFunc, states redisstore.Store, client *http.Client) Handler {
	profiles := importsvc.ImportService{DB: db, TexturesDir: cfg.TexturesDir, HTTPClient: client}
	return Handler{
		auth: auth,
		workflow: importsvc.MicrosoftImportWorkflow{
			APIURL:     cfg.APIURL,
			SiteURL:    cfg.SiteURL,
			Settings:   settings,
			States:     states,
			Profiles:   profiles,
			HTTPClient: client,
		},
	}
}

func (h Handler) Auth(next http.HandlerFunc) http.HandlerFunc {
	return h.auth(next)
}
