package permission

import (
	"context"
	"sort"
	"time"

	core "element-skin/backend/internal/permission"
)

const (
	protectedSubjectLockID   int64  = 0x50524F5445535542
	obsoleteSuperAdminRoleID string = "super_admin"
)

func (s Store) GrantInitialProtectedManagerIfNone(ctx context.Context, userID string) (bool, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return false, err
	}
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, protectedSubjectLockID); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM permission_subjects
			WHERE protected=TRUE AND kind='user' AND user_id IS NOT NULL
		)
	`).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, tx.Commit(ctx)
	}
	now := time.Now().UnixMilli()
	if err := assignProtectedManager(ctx, tx, SubjectIDForUser(userID), "", now); err != nil {
		return false, err
	}
	if err := cleanupObsoleteSuperAdminRole(ctx, tx); err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	if s.Cache != nil {
		_ = s.Cache.DeleteEffective(ctx, SubjectIDForUser(userID))
	}
	return true, nil
}

func (s Store) TransferProtectedSubject(ctx context.Context, fromUserID, toUserID, grantedBySubjectID string) ([]string, error) {
	if err := s.EnsureUserSubject(ctx, fromUserID); err != nil {
		return nil, err
	}
	if err := s.EnsureUserSubject(ctx, toUserID); err != nil {
		return nil, err
	}
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, protectedSubjectLockID); err != nil {
		return nil, err
	}

	affected, err := protectedManagerUserIDs(ctx, tx)
	if err != nil {
		return nil, err
	}
	affected[fromUserID] = true
	affected[toUserID] = true

	now := time.Now().UnixMilli()
	if err := assignProtectedManager(ctx, tx, SubjectIDForUser(toUserID), grantedBySubjectID, now); err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, SubjectIDForUser(fromUserID), core.RoleAdmin, nullString(grantedBySubjectID), now); err != nil {
		return nil, err
	}
	if err := cleanupObsoleteSuperAdminRole(ctx, tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	userIDs := make([]string, 0, len(affected))
	for userID := range affected {
		userIDs = append(userIDs, userID)
	}
	sort.Strings(userIDs)
	if s.Cache != nil {
		for _, userID := range userIDs {
			_ = s.Cache.DeleteEffective(ctx, SubjectIDForUser(userID))
		}
	}
	return userIDs, nil
}

func assignProtectedManager(ctx context.Context, q Querier, targetSubjectID, grantedBySubjectID string, now int64) error {
	manageProtected := core.MustDefinitionByCode("permission_protected.manage.any")
	if _, err := q.Exec(ctx, `
		UPDATE permission_subjects
		SET protected=FALSE, updated_at=$1
		WHERE kind='user' AND user_id IS NOT NULL AND protected=TRUE
	`, now); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `
		UPDATE permission_subjects
		SET protected=TRUE, updated_at=$2
		WHERE id=$1
	`, targetSubjectID, now); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `
		DELETE FROM subject_permission_overrides spo
		USING permission_subjects ps
		WHERE ps.id=spo.subject_id
		  AND ps.kind='user'
		  AND ps.user_id IS NOT NULL
		  AND spo.permission_id=$1
	`, int64(manageProtected.ID)); err != nil {
		return err
	}
	if _, err := q.Exec(ctx, `
		INSERT INTO subject_permission_overrides (subject_id,permission_id,effect,granted_by_subject_id,created_at)
		VALUES ($1,$2,'allow',$3,$4)
		ON CONFLICT (subject_id, permission_id) DO UPDATE
		SET effect='allow', granted_by_subject_id=EXCLUDED.granted_by_subject_id
	`, targetSubjectID, int64(manageProtected.ID), nullString(grantedBySubjectID), now); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4), ($1,$5,$3,$4)
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, targetSubjectID, core.RoleUser, nullString(grantedBySubjectID), now, core.RoleAdmin)
	return err
}

func protectedManagerUserIDs(ctx context.Context, q Querier) (map[string]bool, error) {
	manageProtected := core.MustDefinitionByCode("permission_protected.manage.any")
	rows, err := q.Query(ctx, `
		SELECT DISTINCT ps.user_id
		FROM permission_subjects ps
		LEFT JOIN subject_permission_overrides spo ON spo.subject_id=ps.id
			AND spo.permission_id=$1
			AND spo.effect='allow'
		WHERE ps.kind='user'
		  AND ps.user_id IS NOT NULL
		  AND (ps.protected=TRUE OR spo.subject_id IS NOT NULL)
	`, int64(manageProtected.ID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		out[userID] = true
	}
	return out, rows.Err()
}

func cleanupObsoleteSuperAdminRole(ctx context.Context, q Querier) error {
	if _, err := q.Exec(ctx, `DELETE FROM subject_roles WHERE role_id=$1`, obsoleteSuperAdminRoleID); err != nil {
		return err
	}
	_, err := q.Exec(ctx, `DELETE FROM roles WHERE id=$1`, obsoleteSuperAdminRoleID)
	return err
}
