package user

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func (s Store) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	var cur, super bool
	if err := s.Pool.QueryRow(ctx, `SELECT is_admin,is_super_admin FROM users WHERE id=$1`, id).Scan(&cur, &super); err != nil {
		return false, err
	}
	if super {
		return true, nil
	}
	next := !cur
	_, err := s.Pool.Exec(ctx, `UPDATE users SET is_admin=$1 WHERE id=$2`, next, id)
	return next, err
}

func (s Store) TransferSuperAdmin(ctx context.Context, fromID, toID string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET is_super_admin=FALSE, is_admin=TRUE WHERE id=$1 AND is_super_admin=TRUE`, fromID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	tag, err = tx.Exec(ctx, `UPDATE users SET is_super_admin=TRUE, is_admin=TRUE WHERE id=$1`, toID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func (s Store) Ban(ctx context.Context, id string, until int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	return err
}

func (s Store) Unban(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	return err
}
