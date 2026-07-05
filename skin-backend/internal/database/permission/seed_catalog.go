package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

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
			INSERT INTO permissions (id,code,resource_id,action_id,scope_id,description,created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT (id) DO UPDATE
			SET code=EXCLUDED.code,
			    resource_id=EXCLUDED.resource_id,
			    action_id=EXCLUDED.action_id,
			    scope_id=EXCLUDED.scope_id,
			    description=EXCLUDED.description
		`, int64(def.ID), def.Code, int(def.Resource.ID), int(def.Action.ID), int(def.Scope.ID), def.Description, now); err != nil {
			return err
		}
	}
	return nil
}
