package permission

import (
	"context"

	"github.com/jackc/pgx/v5"
)

func seedClientSubjects(ctx context.Context, tx pgx.Tx, now int64) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO permission_subjects (id,user_id,kind,status,protected,created_at,updated_at)
		SELECT 'client:' || id, NULL, 'client', 'active', FALSE, $1, $1
		FROM delegated_clients
		ON CONFLICT (id) DO UPDATE
		SET kind='client', updated_at=EXCLUDED.updated_at
	`, now)
	return err
}
