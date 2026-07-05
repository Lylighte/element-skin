package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

func seedUserSubjects(ctx context.Context, tx pgx.Tx, now int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO permission_subjects (id,user_id,kind,status,protected,created_at,updated_at)
		SELECT 'user:' || id, id, 'user', 'active', FALSE, $1, $1
		FROM users
		ON CONFLICT (id) DO UPDATE
		SET user_id=EXCLUDED.user_id, updated_at=EXCLUDED.updated_at
	`, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,created_at)
		SELECT 'user:' || id, $1, $2
		FROM users
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, core.RoleUser, now); err != nil {
		return err
	}
	hasAdmin, err := usersColumnExists(ctx, tx, "is_admin")
	if err != nil {
		return err
	}
	if hasAdmin {
		if _, err := tx.Exec(ctx, `
			INSERT INTO subject_roles (subject_id,role_id,created_at)
			SELECT 'user:' || id, $1, $2
			FROM users
			WHERE is_admin=TRUE
			ON CONFLICT (subject_id, role_id) DO NOTHING
		`, core.RoleAdmin, now); err != nil {
			return err
		}
	}
	if err := seedProtectedUserSubject(ctx, tx, now); err != nil {
		return err
	}
	return nil
}
