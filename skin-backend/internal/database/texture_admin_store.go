package database

import (
	"context"

	"element-skin/backend/internal/database/texture"
)

var ErrNotFound = texture.ErrNotFound

func (db *DB) ListAllTextures(ctx context.Context, limit int, lastCreated *int64, lastHash, query, typeFilter string) (map[string]any, error) {
	return db.textureStore().ListAll(ctx, limit, lastCreated, lastHash, query, typeFilter)
}

func (db *DB) AdminUpdateTexturePublic(ctx context.Context, hash string, isPublic bool) error {
	return db.textureStore().AdminUpdatePublic(ctx, hash, isPublic)
}

func (db *DB) AdminUpdateTextureNote(ctx context.Context, hash, note string) error {
	return db.textureStore().AdminUpdateNote(ctx, hash, note)
}

func (db *DB) AdminUpdateTextureModel(ctx context.Context, hash, model string) error {
	return db.textureStore().AdminUpdateModel(ctx, hash, model)
}

func (db *DB) TextureExists(ctx context.Context, hash string) (bool, error) {
	return db.textureStore().Exists(ctx, hash)
}

func (db *DB) AdminDeleteTexture(ctx context.Context, hash, textureType, userID string, force bool) error {
	return db.textureStore().AdminDelete(ctx, hash, textureType, userID, force)
}
