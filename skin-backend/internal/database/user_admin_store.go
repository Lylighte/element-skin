package database

import "context"

func (db *DB) ToggleAdmin(ctx context.Context, id string) (bool, error) {
	return db.userStore().ToggleAdmin(ctx, id)
}

func (db *DB) BanUser(ctx context.Context, id string, until int64) error {
	return db.userStore().Ban(ctx, id, until)
}

func (db *DB) UnbanUser(ctx context.Context, id string) error {
	return db.userStore().Unban(ctx, id)
}
