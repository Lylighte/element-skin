package permission

import (
	"context"
	"time"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

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
