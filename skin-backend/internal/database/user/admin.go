package user

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s Store) Ban(ctx context.Context, id string, until int64) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}

func (s Store) Unban(ctx context.Context, id string) error {
	tag, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return err
}
