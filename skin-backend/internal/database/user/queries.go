package user

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) GetByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, userSelectSQL+` WHERE email=$1`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) GetByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scan(s.Pool.QueryRow(ctx, userSelectSQL+` WHERE id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &u, err
}

func (s Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s Store) IsBanned(ctx context.Context, id string) (bool, error) {
	var until *int64
	err := s.Pool.QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, id).Scan(&until)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if until == nil {
		return false, nil
	}
	return *until > time.Now().UnixMilli(), err
}
