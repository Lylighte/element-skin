package fallback

import (
	"errors"

	"github.com/jackc/pgx/v5"
)

func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}
