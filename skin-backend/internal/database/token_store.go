package database

import (
	"context"

	"element-skin/backend/internal/database/token"
	"element-skin/backend/internal/model"
)

func (db *DB) tokenStore() token.Store {
	return token.Store{Pool: db.Pool}
}

func (db *DB) AddToken(ctx context.Context, t model.Token) error {
	return db.tokenStore().Add(ctx, t)
}

func (db *DB) GetToken(ctx context.Context, access string) (*model.Token, error) {
	return db.tokenStore().Get(ctx, access)
}

func (db *DB) DeleteToken(ctx context.Context, access string) error {
	return db.tokenStore().Delete(ctx, access)
}

func (db *DB) DeleteTokensByUser(ctx context.Context, userID string) error {
	return db.tokenStore().DeleteByUser(ctx, userID)
}

func (db *DB) CleanupTokens(ctx context.Context, userID string, cutoff int64, keep int) error {
	return db.tokenStore().Cleanup(ctx, userID, cutoff, keep)
}

func (db *DB) AddSession(ctx context.Context, s model.Session) error {
	return db.tokenStore().AddSession(ctx, s)
}

func (db *DB) ReplaceSession(ctx context.Context, s model.Session) error {
	return db.tokenStore().ReplaceSession(ctx, s)
}

func (db *DB) GetSession(ctx context.Context, serverID string) (*model.Session, error) {
	return db.tokenStore().GetSession(ctx, serverID)
}

func (db *DB) AddRefreshToken(ctx context.Context, hash, userID string, expiresAt, createdAt int64) error {
	return db.tokenStore().AddRefresh(ctx, hash, userID, expiresAt, createdAt)
}

func (db *DB) ConsumeRefreshToken(ctx context.Context, hash string) (map[string]any, error) {
	return db.tokenStore().ConsumeRefresh(ctx, hash)
}

func (db *DB) DeleteRefreshToken(ctx context.Context, hash string) error {
	return db.tokenStore().DeleteRefresh(ctx, hash)
}

func (db *DB) DeleteRefreshTokensByUser(ctx context.Context, userID string) error {
	return db.tokenStore().DeleteRefreshByUser(ctx, userID)
}

func (db *DB) DeleteExpiredRefreshTokens(ctx context.Context, cutoff int64) error {
	return db.tokenStore().DeleteExpiredRefresh(ctx, cutoff)
}

func (db *DB) GetRefreshToken(ctx context.Context, hash string) (map[string]any, error) {
	return db.tokenStore().GetRefresh(ctx, hash)
}
