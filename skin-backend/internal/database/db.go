package database

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/database/invite"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/setting"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/database/token"
	"element-skin/backend/internal/database/user"
	"element-skin/backend/internal/database/verification"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	Pool          *pgxpool.Pool
	Users         user.Store
	Profiles      profile.Store
	Textures      texture.Store
	Tokens        token.Store
	Settings      setting.Store
	Invites       invite.Store
	Fallbacks     fallback.Store
	Verifications verification.Store
}

func Open(ctx context.Context, cfg config.Config) (*DB, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseDSN)
	if err != nil {
		return nil, err
	}
	pcfg.MaxConns = cfg.MaxConnections
	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	db := New(pool)
	if err := db.Init(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return db, nil
}

func New(pool *pgxpool.Pool) *DB {
	return &DB{
		Pool:          pool,
		Users:         user.Store{Pool: pool},
		Profiles:      profile.Store{Pool: pool},
		Textures:      texture.Store{Pool: pool},
		Tokens:        token.Store{Pool: pool},
		Settings:      setting.Store{Pool: pool},
		Invites:       invite.Store{Pool: pool},
		Fallbacks:     fallback.Store{Pool: pool},
		Verifications: verification.Store{Pool: pool},
	}
}

func (db *DB) Close() {
	if db != nil && db.Pool != nil {
		db.Pool.Close()
	}
}

func (db *DB) Init(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, InitSQL)
	return err
}

func (db *DB) ResetPublicSchema(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO public;`)
	if err != nil {
		return err
	}
	return db.Init(ctx)
}

func NowMS() int64 { return time.Now().UnixMilli() }

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
