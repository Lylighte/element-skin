package auth

import (
	"errors"

	"element-skin/backend/internal/util"
)

func validEmail(s string) bool {
	return util.ValidEmail(s)
}

var ErrUnauthorized = errors.New("unauthorized")
