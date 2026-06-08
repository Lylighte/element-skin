package service

import (
	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database"
)

// Site contains user-facing account, profile, and texture operations.
type Site struct {
	DB  *database.DB
	Cfg config.Config
}
