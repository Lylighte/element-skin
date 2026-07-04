package user

import "context"

func (s Store) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	return err
}

func (s Store) UpdatePasswordAndRevokeRefresh(ctx context.Context, id, hash string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
