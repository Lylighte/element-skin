package database

import "context"

func (db *DB) ListProfilesByUser(ctx context.Context, userID string, limit int, lastID string) (map[string]any, error) {
	return db.profileStore().ListByUser(ctx, userID, limit, lastID)
}

func (db *DB) ListAllProfiles(ctx context.Context, limit int, lastID, query string) (map[string]any, error) {
	return db.profileStore().ListAll(ctx, limit, lastID, query)
}
