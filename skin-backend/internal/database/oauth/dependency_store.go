package oauth

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type UserCleanupResult struct {
	DeletedGrants      int64
	DeletedDeviceCodes int64
	DeletedClients     int64
	DeletedSubjects    int64
}

func (s Store) RevokeInvalidGrantsForUser(ctx context.Context, userID string, allowedPermissionIDs []int64, revokedAt int64) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE delegated_permission_grants g
		SET status='revoked', revoked_at=$3
		WHERE g.user_id=$1
		  AND g.status='active'
		  AND EXISTS (
		      SELECT 1
		      FROM delegated_grant_permissions gp
		      LEFT JOIN delegated_client_permissions cp
		        ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
		      WHERE gp.grant_id=g.id
		        AND (NOT (gp.permission_id = ANY($2::bigint[])) OR cp.permission_id IS NULL)
		  )
	`, userID, allowedPermissionIDs, revokedAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) DisableInvalidClientsForOwner(ctx context.Context, ownerUserID string, allowedPermissionIDs []int64, exemptPermissionIDs []int64, updatedAt int64) (int64, error) {
	tag, err := s.Pool.Exec(ctx, `
		UPDATE delegated_clients c
		SET status='disabled', updated_at=$4
		WHERE c.owner_user_id=$1
		  AND c.status IN ('pending', 'active')
		  AND EXISTS (
		      SELECT 1
		      FROM delegated_client_permissions cp
		      WHERE cp.client_id=c.id
		        AND NOT (
		            cp.permission_id = ANY($2::bigint[])
		            OR cp.permission_id = ANY($3::bigint[])
		        )
		  )
	`, ownerUserID, allowedPermissionIDs, exemptPermissionIDs, updatedAt)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s Store) DeleteUserOAuthData(ctx context.Context, userID string) (UserCleanupResult, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return UserCleanupResult{}, err
	}
	defer tx.Rollback(ctx)

	clientIDs, err := userOAuthClientIDs(ctx, tx, userID)
	if err != nil {
		return UserCleanupResult{}, err
	}

	var result UserCleanupResult
	tag, err := tx.Exec(ctx, `DELETE FROM delegated_permission_grants WHERE user_id=$1`, userID)
	if err != nil {
		return UserCleanupResult{}, err
	}
	result.DeletedGrants = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `DELETE FROM oauth_device_codes WHERE user_id=$1`, userID)
	if err != nil {
		return UserCleanupResult{}, err
	}
	result.DeletedDeviceCodes = tag.RowsAffected()

	tag, err = tx.Exec(ctx, `DELETE FROM delegated_clients WHERE owner_user_id=$1`, userID)
	if err != nil {
		return UserCleanupResult{}, err
	}
	result.DeletedClients = tag.RowsAffected()

	if len(clientIDs) > 0 {
		subjectIDs := make([]string, 0, len(clientIDs))
		for _, clientID := range clientIDs {
			subjectIDs = append(subjectIDs, "client:"+clientID)
		}
		tag, err = tx.Exec(ctx, `DELETE FROM permission_subjects WHERE id = ANY($1::text[])`, subjectIDs)
		if err != nil {
			return UserCleanupResult{}, err
		}
		result.DeletedSubjects = tag.RowsAffected()
	}

	if err := tx.Commit(ctx); err != nil {
		return UserCleanupResult{}, err
	}
	return result, nil
}

func userOAuthClientIDs(ctx context.Context, tx pgx.Tx, userID string) ([]string, error) {
	rows, err := tx.Query(ctx, `SELECT id FROM delegated_clients WHERE owner_user_id=$1 ORDER BY id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
