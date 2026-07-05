package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

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
