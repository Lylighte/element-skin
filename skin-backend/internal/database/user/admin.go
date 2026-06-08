package user

import "context"

func (s Store) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	var cur bool
	if err := s.Pool.QueryRow(ctx, `SELECT is_admin FROM users WHERE id=$1`, id).Scan(&cur); err != nil {
		return false, err
	}
	next := !cur
	_, err := s.Pool.Exec(ctx, `UPDATE users SET is_admin=$1 WHERE id=$2`, next, id)
	return next, err
}

func (s Store) Ban(ctx context.Context, id string, until int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	return err
}

func (s Store) Unban(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	return err
}
