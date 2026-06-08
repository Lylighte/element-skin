package database

import "context"

func (db *DB) ListUsers(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	return db.userStore().List(ctx, limit, lastID, query)
}
