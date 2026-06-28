package permission

import (
	"context"
	"errors"
	"time"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Querier interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

type Store struct {
	Pool *pgxpool.Pool
	q    Querier
}

func (s Store) conn() Querier {
	if s.q != nil {
		return s.q
	}
	return s.Pool
}

const firstSuperAdminRoleLockID int64 = 0x5045524D53555052

type EffectiveOptions struct {
	SessionKind       string
	Entrypoint        string
	DelegatedGrantID  string
	DelegatedClientID string
	ApplyBanPolicy    bool
}

type SubjectPermissionOverride struct {
	PermissionID   core.ID
	PermissionCode string
	Effect         string
	CreatedAt      int64
}

func SubjectIDForUser(userID string) string {
	return "user:" + userID
}

func (s Store) SeedDefaults(ctx context.Context) error {
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UnixMilli()
	if err := seedCatalog(ctx, tx, now); err != nil {
		return err
	}
	if err := seedRoles(ctx, tx, now); err != nil {
		return err
	}
	if err := seedSessionPolicies(ctx, tx, now); err != nil {
		return err
	}
	if err := seedUserSubjects(ctx, tx, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
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

func (s Store) EffectivePermissionsForUser(ctx context.Context, userID string, opts EffectiveOptions) (core.BitSet, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	subjectID := SubjectIDForUser(userID)
	permissions, err := s.effectivePermissionsForSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	if opts.SessionKind != "" || opts.Entrypoint != "" {
		policy, err := s.sessionPolicy(ctx, opts.SessionKind, opts.Entrypoint)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
	}
	if opts.DelegatedGrantID != "" {
		policy, err := s.delegationPolicy(ctx, userID, opts.DelegatedClientID, opts.DelegatedGrantID)
		if err != nil {
			return nil, err
		}
		permissions = permissions.And(policy)
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

func (s Store) GrantRole(ctx context.Context, userID, roleID, grantedBySubjectID string) error {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err := s.conn().Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (subject_id, role_id) DO UPDATE
		SET granted_by_subject_id=EXCLUDED.granted_by_subject_id
	`, SubjectIDForUser(userID), roleID, nullString(grantedBySubjectID), now)
	return err
}

func (s Store) RevokeRole(ctx context.Context, userID, roleID string) (bool, error) {
	tag, err := s.conn().Exec(ctx, `
		DELETE FROM subject_roles
		WHERE subject_id=$1 AND role_id=$2
	`, SubjectIDForUser(userID), roleID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) GrantInitialSuperAdminIfNone(ctx context.Context, userID string) (bool, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return false, err
	}
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, firstSuperAdminRoleLockID); err != nil {
		return false, err
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM subject_roles WHERE role_id=$1)`, core.RoleSuperAdmin).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		return false, tx.Commit(ctx)
	}
	now := time.Now().UnixMilli()
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,created_at)
		VALUES ($1,$2,$3), ($1,$4,$3)
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, SubjectIDForUser(userID), core.RoleUser, now, core.RoleSuperAdmin); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (s Store) RoleIDsForUser(ctx context.Context, userID string) ([]string, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.conn().Query(ctx, `
		SELECT role_id
		FROM subject_roles
		WHERE subject_id=$1
		ORDER BY role_id
	`, SubjectIDForUser(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var roleID string
		if err := rows.Scan(&roleID); err != nil {
			return nil, err
		}
		out = append(out, roleID)
	}
	return out, rows.Err()
}

func (s Store) UserHasRole(ctx context.Context, userID, roleID string) (bool, error) {
	var exists bool
	err := s.conn().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM subject_roles
			WHERE subject_id=$1 AND role_id=$2
		)
	`, SubjectIDForUser(userID), roleID).Scan(&exists)
	return exists, err
}

func (s Store) UserHasProtectedRole(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := s.conn().QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM subject_roles sr
			JOIN roles r ON r.id=sr.role_id
			WHERE sr.subject_id=$1 AND r.protected=TRUE
		)
	`, SubjectIDForUser(userID)).Scan(&exists)
	return exists, err
}

func (s Store) SetSubjectPermissionOverride(ctx context.Context, userID string, def core.Definition, effect string, grantedBySubjectID string) error {
	if effect != "allow" && effect != "deny" {
		return errors.New("permission override effect must be allow or deny")
	}
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	_, err := s.conn().Exec(ctx, `
		INSERT INTO subject_permission_overrides (subject_id,permission_id,effect,granted_by_subject_id,created_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (subject_id, permission_id) DO UPDATE
		SET effect=EXCLUDED.effect, granted_by_subject_id=EXCLUDED.granted_by_subject_id
	`, SubjectIDForUser(userID), int64(def.ID), effect, nullString(grantedBySubjectID), now)
	return err
}

func (s Store) SubjectPermissionOverridesForUser(ctx context.Context, userID string) ([]SubjectPermissionOverride, error) {
	if err := s.EnsureUserSubject(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.conn().Query(ctx, `
		SELECT p.id,p.code,spo.effect,spo.created_at
		FROM subject_permission_overrides spo
		JOIN permissions p ON p.id=spo.permission_id
		WHERE spo.subject_id=$1
		ORDER BY p.code
	`, SubjectIDForUser(userID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SubjectPermissionOverride
	for rows.Next() {
		var item SubjectPermissionOverride
		var permissionID int64
		if err := rows.Scan(&permissionID, &item.PermissionCode, &item.Effect, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.PermissionID = core.ID(permissionID)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ClearSubjectPermissionOverride(ctx context.Context, userID string, def core.Definition) (bool, error) {
	tag, err := s.conn().Exec(ctx, `
		DELETE FROM subject_permission_overrides
		WHERE subject_id=$1 AND permission_id=$2
	`, SubjectIDForUser(userID), int64(def.ID))
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func seedCatalog(ctx context.Context, tx pgx.Tx, now int64) error {
	for _, item := range core.Resources {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permission_resources (id,code,description,created_at)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (id) DO UPDATE
			SET code=EXCLUDED.code, description=EXCLUDED.description
		`, int(item.ID), item.Code, item.Description, now); err != nil {
			return err
		}
	}
	for _, item := range core.Actions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permission_actions (id,code,description,created_at)
			VALUES ($1,$2,$3,$4)
			ON CONFLICT (id) DO UPDATE
			SET code=EXCLUDED.code, description=EXCLUDED.description
		`, int(item.ID), item.Code, item.Description, now); err != nil {
			return err
		}
	}
	for _, item := range core.Scopes {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permission_scopes (id,code,resolver_key,description,created_at)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (id) DO UPDATE
			SET code=EXCLUDED.code, resolver_key=EXCLUDED.resolver_key, description=EXCLUDED.description
		`, int(item.ID), item.Code, item.ResolverKey, item.Description, now); err != nil {
			return err
		}
	}
	for _, def := range core.Definitions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO permissions (id,code,bit_index,resource_id,action_id,scope_id,description,created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (id) DO UPDATE
			SET code=EXCLUDED.code,
			    bit_index=EXCLUDED.bit_index,
			    resource_id=EXCLUDED.resource_id,
			    action_id=EXCLUDED.action_id,
			    scope_id=EXCLUDED.scope_id,
			    description=EXCLUDED.description
		`, int64(def.ID), def.Code, def.BitIndex, int(def.Resource.ID), int(def.Action.ID), int(def.Scope.ID), def.Description, now); err != nil {
			return err
		}
	}
	return nil
}

func seedRoles(ctx context.Context, tx pgx.Tx, now int64) error {
	for _, role := range core.Roles {
		if _, err := tx.Exec(ctx, `
			INSERT INTO roles (id,name,description,system_role,protected,created_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,$6,$6)
			ON CONFLICT (id) DO UPDATE
			SET name=EXCLUDED.name,
			    description=EXCLUDED.description,
			    system_role=EXCLUDED.system_role,
			    protected=EXCLUDED.protected,
			    updated_at=EXCLUDED.updated_at
		`, role.ID, role.Name, role.Description, role.SystemRole, role.Protected, now); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id=$1`, role.ID); err != nil {
			return err
		}
		for _, def := range role.Permissions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO role_permissions (role_id,permission_id,created_at)
				VALUES ($1,$2,$3)
				ON CONFLICT (role_id, permission_id) DO NOTHING
			`, role.ID, int64(def.ID), now); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedSessionPolicies(ctx context.Context, tx pgx.Tx, now int64) error {
	for _, policy := range core.SessionPolicies {
		if _, err := tx.Exec(ctx, `DELETE FROM session_permission_policies WHERE session_kind=$1 AND entrypoint=$2`, policy.SessionKind, policy.Entrypoint); err != nil {
			return err
		}
		for _, def := range policy.Permissions {
			if _, err := tx.Exec(ctx, `
				INSERT INTO session_permission_policies (session_kind,entrypoint,permission_id,created_at)
				VALUES ($1,$2,$3,$4)
				ON CONFLICT (session_kind, entrypoint, permission_id) DO NOTHING
			`, policy.SessionKind, policy.Entrypoint, int64(def.ID), now); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedUserSubjects(ctx context.Context, tx pgx.Tx, now int64) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO permission_subjects (id,user_id,kind,status,created_at,updated_at)
		SELECT 'user:' || id, id, 'user', 'active', $1, $1
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
	hasSuperAdmin, err := usersColumnExists(ctx, tx, "is_super_admin")
	if err != nil {
		return err
	}
	if hasSuperAdmin {
		if _, err := tx.Exec(ctx, `
			INSERT INTO subject_roles (subject_id,role_id,created_at)
			SELECT 'user:' || id, $1, $2
			FROM users
			WHERE is_super_admin=TRUE
			ON CONFLICT (subject_id, role_id) DO NOTHING
		`, core.RoleSuperAdmin, now); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO subject_roles (subject_id,role_id,created_at)
		SELECT ps.id, $1, $2
		FROM permission_subjects ps
		JOIN users u ON u.id=ps.user_id
		LEFT JOIN subject_roles admin_role ON admin_role.subject_id=ps.id AND admin_role.role_id=$3
		WHERE NOT EXISTS (SELECT 1 FROM subject_roles WHERE role_id=$1)
		ORDER BY (admin_role.role_id IS NULL), u.created_at ASC, u.id ASC
		LIMIT 1
		ON CONFLICT (subject_id, role_id) DO NOTHING
	`, core.RoleSuperAdmin, now, core.RoleAdmin); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		WITH ranked AS (
			SELECT sr.subject_id,
			       row_number() OVER (ORDER BY u.created_at ASC, u.id ASC) AS rn
			FROM subject_roles sr
			JOIN permission_subjects ps ON ps.id=sr.subject_id
			JOIN users u ON u.id=ps.user_id
			WHERE sr.role_id=$1
		)
		DELETE FROM subject_roles sr
		USING ranked
		WHERE sr.subject_id=ranked.subject_id
		  AND sr.role_id=$1
		  AND ranked.rn > 1
	`, core.RoleSuperAdmin)
	return err
}

func usersColumnExists(ctx context.Context, tx pgx.Tx, column string) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_schema='public'
			  AND table_name='users'
			  AND column_name=$1
		)
	`, column).Scan(&exists)
	return exists, err
}

func (s Store) effectivePermissionsForSubject(ctx context.Context, subjectID string) (core.BitSet, error) {
	permissions := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM subject_roles sr
		JOIN role_permissions rp ON rp.role_id=sr.role_id
		JOIN permissions p ON p.id=rp.permission_id
		WHERE sr.subject_id=$1
		ORDER BY p.bit_index
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		permissions.Set(bitIndex)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	rows, err = s.conn().Query(ctx, `
		SELECT p.bit_index, spo.effect
		FROM subject_permission_overrides spo
		JOIN permissions p ON p.id=spo.permission_id
		WHERE spo.subject_id=$1
		ORDER BY p.bit_index
	`, subjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	denied := core.NewBitSet(len(core.Definitions))
	for rows.Next() {
		var bitIndex int
		var effect string
		if err := rows.Scan(&bitIndex, &effect); err != nil {
			return nil, err
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
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM session_permission_policies spp
		JOIN permissions p ON p.id=spp.permission_id
		WHERE spp.session_kind=$1 AND spp.entrypoint=$2
		ORDER BY p.bit_index
	`, sessionKind, entrypoint)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		policy.Set(bitIndex)
	}
	return policy, rows.Err()
}

func (s Store) delegationPolicy(ctx context.Context, userID, clientID, grantID string) (core.BitSet, error) {
	policy := core.NewBitSet(len(core.Definitions))
	rows, err := s.conn().Query(ctx, `
		SELECT p.bit_index
		FROM delegated_permission_grants g
		JOIN delegated_clients c ON c.id=g.client_id
		JOIN delegated_grant_permissions gp ON gp.grant_id=g.id
		JOIN delegated_client_permissions cp ON cp.client_id=g.client_id AND cp.permission_id=gp.permission_id
		JOIN permissions p ON p.id=gp.permission_id
		WHERE g.id=$1
		  AND g.user_id=$2
		  AND ($3='' OR g.client_id=$3)
		  AND g.status='active'
		  AND c.status='active'
		ORDER BY p.bit_index
	`, grantID, userID, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var bitIndex int
		if err := rows.Scan(&bitIndex); err != nil {
			return nil, err
		}
		policy.Set(bitIndex)
	}
	return policy, rows.Err()
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

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
