package permission

import (
	"context"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
)

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
