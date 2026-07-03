package oauth

import (
	"context"

	"element-skin/backend/internal/model"
)

func (s Store) CreateGrant(ctx context.Context, grant model.OAuthGrant, permissionIDs []int64) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `
		INSERT INTO delegated_permission_grants (id, user_id, subject_id, client_id, status, created_at, revoked_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, grant.ID, grant.UserID, grant.SubjectID, grant.ClientID, grant.Status, grant.CreatedAt, grant.RevokedAt); err != nil {
		return err
	}
	if err := insertGrantPermissions(ctx, tx, grant.ID, permissionIDs, grant.CreatedAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) RevokeGrant(ctx context.Context, grantID, userID string, revokedAt int64) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE delegated_permission_grants
		SET status='revoked', revoked_at=$3
		WHERE id=$1 AND ($2='' OR user_id=$2) AND status='active'
	`, grantID, userID, revokedAt)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) ListGrantsByUser(ctx context.Context, userID string, limit int) ([]model.OAuthGrant, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT id, user_id, subject_id, client_id, status, created_at, revoked_at
		FROM delegated_permission_grants
		WHERE user_id=$1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []model.OAuthGrant
	for rows.Next() {
		var grant model.OAuthGrant
		if err := rows.Scan(&grant.ID, &grant.UserID, &grant.SubjectID, &grant.ClientID, &grant.Status, &grant.CreatedAt, &grant.RevokedAt); err != nil {
			return nil, err
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func (s Store) GrantPermissionIDs(ctx context.Context, grantID string) ([]int64, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT permission_id
		FROM delegated_grant_permissions
		WHERE grant_id=$1
		ORDER BY permission_id
	`, grantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanInt64Rows(rows)
}
