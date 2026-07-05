package permission

import (
	"context"

	core "element-skin/backend/internal/permission"
)

type EffectiveOptions struct {
	SessionKind       string
	Entrypoint        string
	DelegatedGrantID  string
	DelegatedClientID string
	ApplyBanPolicy    bool
}

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
