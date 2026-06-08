package database

import "context"

func (db *DB) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	var cur bool
	if err := db.Pool.QueryRow(ctx, `SELECT is_admin FROM users WHERE id=$1`, id).Scan(&cur); err != nil {
		return false, err
	}
	next := !cur
	_, err := db.Pool.Exec(ctx, `UPDATE users SET is_admin=$1 WHERE id=$2`, next, id)
	return next, err
}

func (db *DB) BanUser(ctx context.Context, id string, until int64) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, until, id)
	return err
}

func (db *DB) UnbanUser(ctx context.Context, id string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=NULL WHERE id=$1`, id)
	return err
}
