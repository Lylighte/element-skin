package user

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

const displayNameLockSeed int64 = 0x444953504C4159

var ErrDisplayNameConflict = errors.New("display name already exists")

func (s Store) IsDisplayNameTaken(ctx context.Context, name string, exclude string) (bool, error) {
	var one int
	var err error
	if exclude != "" {
		err = s.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1 AND id<>$2`, name, exclude).Scan(&one)
	} else {
		err = s.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1`, name).Scan(&one)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func lockDisplayName(ctx context.Context, tx pgx.Tx, name, excludeID string) error {
	if name == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,$2))`, name, displayNameLockSeed); err != nil {
		return err
	}
	var exists bool
	if err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM users WHERE display_name=$1 AND id<>$2)`,
		name, excludeID,
	).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return ErrDisplayNameConflict
	}
	return nil
}
