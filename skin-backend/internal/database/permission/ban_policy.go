package permission

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s Store) userBanned(ctx context.Context, userID string) (bool, error) {
	var bannedUntil *int64
	err := s.conn().QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, userID).Scan(&bannedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bannedUntil != nil && *bannedUntil > time.Now().UnixMilli(), nil
}
