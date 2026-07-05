package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

func seedProtectedUserSubject(ctx context.Context, tx pgx.Tx, now int64) error {
	subjectID, err := protectedSeedCandidate(ctx, tx)
	if err != nil {
		return err
	}
	if subjectID == "" {
		return nil
	}
	return assignProtectedManager(ctx, tx, subjectID, "", now)
}

func protectedSeedCandidate(ctx context.Context, tx pgx.Tx) (string, error) {
	var subjectID string
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM permission_subjects
		WHERE kind='user' AND user_id IS NOT NULL AND protected=TRUE
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`).Scan(&subjectID)
	if err == nil {
		return subjectID, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	manageProtected := core.MustDefinitionByCode("permission_protected.manage.any")
	err = tx.QueryRow(ctx, `
		SELECT spo.subject_id
		FROM subject_permission_overrides spo
		JOIN permission_subjects ps ON ps.id=spo.subject_id
		WHERE spo.permission_id=$1 AND spo.effect='allow' AND ps.kind='user' AND ps.user_id IS NOT NULL
		ORDER BY spo.created_at DESC, spo.subject_id DESC
		LIMIT 1
	`, int64(manageProtected.ID)).Scan(&subjectID)
	if err == nil {
		return subjectID, nil
	}
	if err != pgx.ErrNoRows {
		return "", err
	}

	err = tx.QueryRow(ctx, `
		SELECT ps.id
		FROM permission_subjects ps
		JOIN users u ON u.id=ps.user_id
		LEFT JOIN subject_roles admin_role ON admin_role.subject_id=ps.id AND admin_role.role_id=$1
		WHERE ps.kind='user' AND ps.user_id IS NOT NULL
		ORDER BY (admin_role.role_id IS NULL), u.created_at ASC, u.id ASC
		LIMIT 1
	`, core.RoleAdmin).Scan(&subjectID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return subjectID, err
}
