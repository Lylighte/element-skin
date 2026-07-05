package permission

import (
	"context"

	core "element-skin/backend/internal/permission"
)

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
