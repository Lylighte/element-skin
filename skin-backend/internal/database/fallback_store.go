package database

import (
	"context"

	"element-skin/backend/internal/database/fallback"
)

type FallbackEndpoint = fallback.Endpoint

func (db *DB) fallbackStore() fallback.Store {
	return fallback.Store{Pool: db.Pool}
}

func (db *DB) ListFallbackEndpoints(ctx context.Context) ([]map[string]any, error) {
	return db.fallbackStore().ListEndpoints(ctx)
}

func (db *DB) SaveFallbackEndpoints(ctx context.Context, endpoints []FallbackEndpoint) error {
	return db.fallbackStore().SaveEndpoints(ctx, endpoints)
}

func (db *DB) CollectFallbackSkinDomains(ctx context.Context) ([]string, error) {
	return db.fallbackStore().CollectSkinDomains(ctx)
}

func (db *DB) GetPrimaryFallbackEndpoint(ctx context.Context) (map[string]any, error) {
	return db.fallbackStore().PrimaryEndpoint(ctx)
}

func (db *DB) AddWhitelistUser(ctx context.Context, username string, endpointID int) error {
	return db.fallbackStore().AddWhitelistUser(ctx, username, endpointID)
}

func (db *DB) IsUserInWhitelist(ctx context.Context, username string, endpointID int) (bool, error) {
	return db.fallbackStore().IsUserInWhitelist(ctx, username, endpointID)
}

func (db *DB) ListWhitelistUsers(ctx context.Context, endpointID int) ([]map[string]any, error) {
	return db.fallbackStore().ListWhitelistUsers(ctx, endpointID)
}

func (db *DB) RemoveWhitelistUser(ctx context.Context, username string, endpointID int) error {
	return db.fallbackStore().RemoveWhitelistUser(ctx, username, endpointID)
}
