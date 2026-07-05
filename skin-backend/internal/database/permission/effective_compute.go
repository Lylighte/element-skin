package permission

import (
	"context"
	"time"

	core "element-skin/backend/internal/permission"
)

func (s Store) effectivePermissionsForSubject(ctx context.Context, subjectID string) (core.BitSet, error) {
	if s.Cache != nil {
		if cached, ok, err := s.Cache.GetEffective(ctx, subjectID); err != nil {
			return nil, err
		} else if ok {
			return cached, nil
		}
	}
	permissions, err := s.computeEffectivePermissions(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	if s.Cache != nil {
		_ = s.Cache.SetEffective(ctx, subjectID, permissions, 5*time.Minute)
	}
	return permissions, nil
}

func (s Store) computeEffectivePermissions(ctx context.Context, subjectID string) (core.BitSet, error) {
	permissions := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT rp.permission_id
		FROM subject_roles sr
		JOIN role_permissions rp ON rp.role_id=sr.role_id
		WHERE sr.subject_id=$1
		ORDER BY rp.permission_id
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var permissionID int64
		if err := rows.Scan(&permissionID); err != nil {
			return nil, err
		}
		setPermissionBit(permissions, permissionID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = s.conn().Query(ctx, `
		SELECT spo.permission_id, spo.effect
		FROM subject_permission_overrides spo
		WHERE spo.subject_id=$1
		ORDER BY spo.permission_id
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	denied := core.NewBitSet(len(core.Definitions))
	for rows.Next() {
		var permissionID int64
		var effect string
		if err := rows.Scan(&permissionID, &effect); err != nil {
			return nil, err
		}
		bitIndex, ok := bitIndexForPermissionID(permissionID)
		if !ok {
			continue
		}
		if effect == "allow" {
			permissions.Set(bitIndex)
		} else {
			denied.Set(bitIndex)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return permissions.AndNot(denied), nil
}
