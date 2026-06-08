package yggdrasil

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

type Yggdrasil struct {
	DB     *database.DB
	Cfg    config.Config
	Signer *Signer
}

func New(db *database.DB, cfg config.Config) (Yggdrasil, error) {
	signer, err := NewSigner(cfg)
	if err != nil {
		return Yggdrasil{}, err
	}
	return Yggdrasil{DB: db, Cfg: cfg, Signer: signer}, nil
}

func yggErr(status int, code, msg string) error {
	return util.HTTPError{Status: status, Detail: msg, YggError: code}
}

func (y Yggdrasil) signer() (*Signer, error) {
	if y.Signer != nil {
		return y.Signer, nil
	}
	return NewSigner(y.Cfg)
}

func (y Yggdrasil) publicTextureBaseURL() string {
	if y.Cfg.APIURL != "" {
		return y.Cfg.APIURL
	}
	return y.Cfg.SiteURL
}
