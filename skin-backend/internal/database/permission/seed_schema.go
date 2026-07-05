package permission

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func usersColumnExists(ctx context.Context, tx pgx.Tx, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema='public'
			  AND table_name='users'
			  AND column_name=$1
		)
	`, column).Scan(&exists)
	return exists, err
}
