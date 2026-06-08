package database

import "context"

func (db *DB) ListPublicLibrary(ctx context.Context, limit int, textureType, query string, lastCreated *int64, lastHash string) (map[string]any, error) {
	return db.textureStore().ListPublic(ctx, limit, textureType, query, lastCreated, lastHash)
}

func (db *DB) AddTextureToWardrobe(ctx context.Context, userID, hash string) (bool, error) {
	return db.textureStore().AddToWardrobe(ctx, userID, hash)
}
