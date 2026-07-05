package permission

import (
	"context"

	core "element-skin/backend/internal/permission"
)

func (s Store) sessionPolicy(ctx context.Context, sessionKind, entrypoint string) (core.BitSet, error) {
	if cached, ok := s.cachedSessionPolicy(sessionKind, entrypoint); ok {
		return cached.Clone(), nil
	}
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT spp.permission_id
		FROM session_permission_policies spp
		WHERE spp.session_kind=$1 AND spp.entrypoint=$2
		ORDER BY spp.permission_id
	`, sessionKind, entrypoint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var permissionID int64
		if err := rows.Scan(&permissionID); err != nil {
			return nil, err
		}
		setPermissionBit(policy, permissionID)
	}
	return policy, rows.Err()
}

func (s Store) delegationPolicy(ctx context.Context, subjectID, clientID, grantID string) (core.BitSet, error) {
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT gp.permission_id
		FROM delegated_permission_grants g
		JOIN delegated_clients c ON c.id=g.client_id
		JOIN delegated_grant_permissions gp ON gp.grant_id=g.id
		JOIN delegated_client_permissions cp ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
		WHERE g.id=$1
		  AND g.subject_id=$2
		  AND ($3='' OR g.client_id=$3)
		  AND g.status='active'
		  AND c.status='active'
		ORDER BY gp.permission_id
	`, grantID, subjectID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var permissionID int64
		if err := rows.Scan(&permissionID); err != nil {
			return nil, err
		}
		setPermissionBit(policy, permissionID)
	}
	return policy, rows.Err()
}
