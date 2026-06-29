package permission

import (
	"context"
	"time"

	core "element-skin/backend/internal/permission"
)

func SubjectIDForUser(userID string) string {
	return "user:" + userID
}

func (s Store) EnsureUserSubject(ctx context.Context, userID string) error {
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(ctx, `
		INSERT INTO permission_subjects (id,user_id,kind,status,created_at,updated_at)
		VALUES ($1,$2,'user','active',$3,$3)
		ON CONFLICT (id) DO UPDATE
		SET user_id=EXCLUDED.user_id, kind='user', updated_at=EXCLUDED.updated_at
	`, SubjectIDForUser(userID), userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,created_at)
		VALUES ($1,$2,$3)
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, SubjectIDForUser(userID), core.RoleUser, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
