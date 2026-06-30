package permission

import (
	"context"
	"errors"
	"time"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

type EffectiveOptions struct {
	SessionKind       string
	Entrypoint        string
	DelegatedGrantID  string
	DelegatedClientID string
	ApplyBanPolicy    bool
}

var runtimePermissionBitIndexes = permissionBitIndexMap()

func (s Store) EffectivePermissionsForUser(ctx context.Context, userID string, opts EffectiveOptions) (core.BitSet, error) {
	subjectID := SubjectIDForUser(userID)

	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	permissions, err := s.EffectivePermissionsForSubject(ctx, subjectID, opts)
	if err != nil {
		return nil, err
	}
	if opts.ApplyBanPolicy {
		banned, err := s.userBanned(ctx, userID)
		if err != nil {
			return nil, err
		}
		if banned {
			join := core.MustDefinitionByCode("yggdrasil_server.join.bound_profile")
			permissions.Clear(join.BitIndex)
		}
	}
	return permissions, nil
}

func (s Store) EffectivePermissionsForClient(ctx context.Context, clientID string, opts EffectiveOptions) (core.BitSet, error) {
	if err := s.EnsureClientSubject(ctx, clientID); err != nil {
		return nil, err
	}
	return s.EffectivePermissionsForSubject(ctx, SubjectIDForClient(clientID), opts)
}

func (s Store) EffectivePermissionsForSubject(ctx context.Context, subjectID string, opts EffectiveOptions) (core.BitSet, error) {
	var permissions core.BitSet
	if s.Cache != nil {
		if cached, hit, err := s.Cache.GetEffective(ctx, subjectID); err != nil {
			return nil, err
		} else if hit {
			permissions = cached
		}
	}
	if permissions == nil {
		var err error
		permissions, err = s.effectivePermissionsForSubject(ctx, subjectID)
		if err != nil {
			return nil, err
		}
	}
	if opts.SessionKind != "" || opts.Entrypoint != "" {
		policy, err := s.sessionPolicy(ctx, opts.SessionKind, opts.Entrypoint)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
	}
	if opts.DelegatedGrantID != "" {
		policy, err := s.delegationPolicy(ctx, subjectID, opts.DelegatedClientID, opts.DelegatedGrantID)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
	}
	return permissions, nil
}

func (s Store) ActorForUser(ctx context.Context, userID string, opts EffectiveOptions) (core.Actor, error) {
	permissions, err := s.EffectivePermissionsForUser(ctx, userID, opts)
	if err != nil {
		return core.Actor{}, err
	}
	return core.Actor{
		SubjectID:         SubjectIDForUser(userID),
		UserID:            userID,
		SessionKind:       opts.SessionKind,
		Entrypoint:        opts.Entrypoint,
		DelegationID:      opts.DelegatedGrantID,
		DelegatedClientID: opts.DelegatedClientID,
		Permissions:       permissions,
	}, nil
}

func (s Store) ActorForClient(ctx context.Context, clientID string, opts EffectiveOptions) (core.Actor, error) {
	permissions, err := s.EffectivePermissionsForClient(ctx, clientID, opts)
	if err != nil {
		return core.Actor{}, err
	}
	return core.Actor{
		SubjectID:         SubjectIDForClient(clientID),
		SessionKind:       opts.SessionKind,
		Entrypoint:        opts.Entrypoint,
		DelegatedClientID: clientID,
		Permissions:       permissions,
	}, nil
}

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

func setPermissionBit(bits core.BitSet, permissionID int64) {
	if bitIndex, ok := bitIndexForPermissionID(permissionID); ok {
		bits.Set(bitIndex)
	}
}

func bitIndexForPermissionID(permissionID int64) (int, bool) {
	bitIndex, ok := runtimePermissionBitIndexes[permissionID]
	return bitIndex, ok
}

func permissionBitIndexMap() map[int64]int {
	out := make(map[int64]int, len(core.Definitions))
	for _, def := range core.Definitions {
		out[int64(def.ID)] = def.BitIndex
	}
	return out
}

func (s Store) userBanned(ctx context.Context, userID string) (bool, error) {
	var bannedUntil *int64
	err := s.conn().QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, userID).Scan(&bannedUntil)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return bannedUntil != nil && *bannedUntil > time.Now().UnixMilli(), nil
}
