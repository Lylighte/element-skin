package database

import (
	"context"

	"element-skin/backend/internal/database/texture"
)

func (db *DB) textureStore() texture.Store {
	return texture.Store{Pool: db.Pool}
}

func (db *DB) AddTextureToLibrary(ctx context.Context, userID, hash, textureType, note string, isPublic bool, model string) error {
	return db.textureStore().AddToLibrary(ctx, userID, hash, textureType, note, isPublic, model)
}

func (db *DB) CountTexturesForUser(ctx context.Context, userID string) (int, error) {
	return db.textureStore().CountForUser(ctx, userID)
}

func (db *DB) VerifyTextureOwnership(ctx context.Context, userID, hash, textureType string) (bool, error) {
	return db.textureStore().VerifyOwnership(ctx, userID, hash, textureType)
}

func (db *DB) GetTextureInfo(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	return db.textureStore().GetInfo(ctx, userID, hash, textureType)
}

func (db *DB) ListUserTextures(ctx context.Context, userID, textureType string, limit int, lastCreated *int64, lastHash string) (map[string]any, error) {
	return db.textureStore().ListForUser(ctx, userID, textureType, limit, lastCreated, lastHash)
}

func (db *DB) UpdateTextureNote(ctx context.Context, userID, hash, textureType, note string) error {
	return db.textureStore().UpdateNote(ctx, userID, hash, textureType, note)
}

func (db *DB) UpdateTextureModel(ctx context.Context, userID, hash, textureType, model string) error {
	return db.textureStore().UpdateModel(ctx, userID, hash, textureType, model)
}

func (db *DB) UpdateTexturePublic(ctx context.Context, userID, hash, textureType string, isPublic bool) error {
	return db.textureStore().UpdatePublic(ctx, userID, hash, textureType, isPublic)
}

func (db *DB) DeleteTextureFromLibrary(ctx context.Context, userID, hash, textureType string) (bool, error) {
	return db.textureStore().DeleteFromLibrary(ctx, userID, hash, textureType)
}
