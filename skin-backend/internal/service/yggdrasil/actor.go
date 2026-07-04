package yggdrasil

import (
	"context"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
)

func (y Yggdrasil) ActorForToken(ctx context.Context, token model.Token, applyBanPolicy bool) (permission.Actor, error) {
	actor, err := y.actorForUser(ctx, token.UserID, applyBanPolicy)
	if err != nil {
		return permission.Actor{}, err
	}
	if token.ProfileID != nil {
		actor.BoundProfileID = *token.ProfileID
	}
	return actor, nil
}

func (y Yggdrasil) actorForUser(ctx context.Context, userID string, applyBanPolicy bool) (permission.Actor, error) {
	return y.DB.Permissions.ActorForUser(ctx, userID, permissiondb.EffectiveOptions{
		SessionKind:    permission.SessionKindYggdrasil,
		Entrypoint:     permission.EntrypointYggdrasil,
		ApplyBanPolicy: applyBanPolicy,
	})
}
