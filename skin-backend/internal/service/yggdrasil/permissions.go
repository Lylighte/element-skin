package yggdrasil

import (
	"context"

	"element-skin/backend/internal/permission"
)

func (y Yggdrasil) requireYggPermission(ctx context.Context, userID string, def permission.Definition) error {
	actor, err := y.actorForUser(ctx, userID, false)
	if err != nil {
		return err
	}
	if !actor.Has(def) {
		return yggErr(403, "ForbiddenOperationException", "Permission denied.")
	}
	return nil
}
