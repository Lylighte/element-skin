package service

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/util"
)

type Yggdrasil struct {
	DB  *database.DB
	Cfg config.Config
}

func yggErr(status int, code, msg string) error {
	return util.HTTPError{Status: status, Detail: msg, YggError: code}
}
