package token

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Add(ctx context.Context, t model.Token) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO tokens (access_token,client_token,user_id,profile_id,created_at) VALUES ($1,$2,$3,$4,$5)`,
		t.AccessToken, t.ClientToken, t.UserID, t.ProfileID, t.CreatedAt)
	return err
}

func (s Store) Get(ctx context.Context, access string) (*model.Token, error) {
	var t model.Token
	err := s.Pool.QueryRow(ctx, `SELECT access_token,client_token,user_id,profile_id,created_at FROM tokens WHERE access_token=$1`, access).
		Scan(&t.AccessToken, &t.ClientToken, &t.UserID, &t.ProfileID, &t.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &t, err
}

func (s Store) Delete(ctx context.Context, access string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tokens WHERE access_token=$1`, access)
	return err
}

func (s Store) DeleteByUser(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1`, userID)
	return err
}

func (s Store) Cleanup(ctx context.Context, userID string, cutoff int64, keep int) error {
	if _, err := s.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1 AND created_at < $2`, userID, cutoff); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `DELETE FROM tokens WHERE user_id=$1 AND access_token NOT IN (SELECT access_token FROM tokens WHERE user_id=$1 ORDER BY created_at DESC LIMIT $2)`, userID, keep)
	return err
}

func (s Store) AddSession(ctx context.Context, sess model.Session) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO sessions (server_id,access_token,ip,created_at) VALUES ($1,$2,$3,$4)`, sess.ServerID, sess.AccessToken, sess.IP, sess.CreatedAt)
	return err
}

func (s Store) ReplaceSession(ctx context.Context, sess model.Session) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM sessions WHERE server_id=$1`, sess.ServerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO sessions (server_id,access_token,ip,created_at) VALUES ($1,$2,$3,$4)`, sess.ServerID, sess.AccessToken, sess.IP, sess.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) GetSession(ctx context.Context, serverID string) (*model.Session, error) {
	var sess model.Session
	err := s.Pool.QueryRow(ctx, `SELECT server_id,access_token,ip,created_at FROM sessions WHERE server_id=$1`, serverID).
		Scan(&sess.ServerID, &sess.AccessToken, &sess.IP, &sess.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &sess, err
}

func (s Store) AddRefresh(ctx context.Context, hash, userID string, expiresAt, createdAt int64) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO site_refresh_tokens (token_hash,user_id,expires_at,created_at) VALUES ($1,$2,$3,$4)`, hash, userID, expiresAt, createdAt)
	return err
}

func (s Store) ConsumeRefresh(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := s.Pool.QueryRow(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1 RETURNING token_hash,user_id,expires_at,created_at`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}

func (s Store) DeleteRefresh(ctx context.Context, hash string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE token_hash=$1`, hash)
	return err
}

func (s Store) DeleteRefreshByUser(ctx context.Context, userID string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, userID)
	return err
}

func (s Store) DeleteExpiredRefresh(ctx context.Context, cutoff int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE expires_at < $1`, cutoff)
	return err
}

func (s Store) GetRefresh(ctx context.Context, hash string) (map[string]any, error) {
	var tokenHash, userID string
	var expiresAt, createdAt int64
	err := s.Pool.QueryRow(ctx, `SELECT token_hash,user_id,expires_at,created_at FROM site_refresh_tokens WHERE token_hash=$1`, hash).
		Scan(&tokenHash, &userID, &expiresAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"token_hash": tokenHash, "user_id": userID, "expires_at": expiresAt, "created_at": createdAt}, nil
}
