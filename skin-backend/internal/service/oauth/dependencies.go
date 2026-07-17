package oauth

import (
	"context"

	"element-skin/backend/internal/database"
	dboauth "element-skin/backend/internal/database/oauth"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

type PermissionDependencyResult struct {
	RevokedGrants   []dboauth.RevokedGrantDependency
	DisabledClients []dboauth.DisabledClientDependency
}

func (s Service) ReconcileUserPermissionDependents(ctx context.Context, userID string) (PermissionDependencyResult, error) {
	bits, err := s.DB.Permissions.EffectivePermissionsForUser(ctx, userID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		return PermissionDependencyResult{}, err
	}
	allowedIDs := permissionIDsFromBitSet(bits)
	now := database.NowMS()
	revoked, err := s.DB.OAuth.RevokeInvalidGrantsForUser(ctx, userID, allowedIDs, now)
	if err != nil {
		return PermissionDependencyResult{}, err
	}
	for _, item := range revoked {
		if err := s.invalidateGrantCredentials(ctx, item.GrantID, now); err != nil {
			return PermissionDependencyResult{}, err
		}
	}
	disabled, err := s.DB.OAuth.DisableInvalidClientsForOwner(ctx, userID, allowedIDs, serverPermissionIDs(), now)
	if err != nil {
		return PermissionDependencyResult{}, err
	}
	for _, item := range disabled {
		if err := s.revokeClientAuthorizations(ctx, item.ClientID, now); err != nil {
			return PermissionDependencyResult{}, err
		}
	}
	result := PermissionDependencyResult{RevokedGrants: revoked, DisabledClients: disabled}
	if err := s.notifyPermissionDependencyChanges(ctx, result); err != nil {
		return PermissionDependencyResult{}, err
	}
	return result, nil
}

func (s Service) DeleteUserOAuthData(ctx context.Context, userID string) (dboauth.UserCleanupResult, error) {
	clientIDs, err := s.DB.OAuth.ClientIDsByOwner(ctx, userID)
	if err != nil {
		return dboauth.UserCleanupResult{}, err
	}
	result, err := s.DB.OAuth.DeleteUserOAuthData(ctx, userID)
	if err != nil {
		return dboauth.UserCleanupResult{}, err
	}
	if err := s.Redis.DeleteOAuthAccessTokensByUser(ctx, userID); err != nil {
		return dboauth.UserCleanupResult{}, err
	}
	for _, clientID := range clientIDs {
		if err := s.Redis.DeleteOAuthAccessTokensByClient(ctx, clientID); err != nil {
			return dboauth.UserCleanupResult{}, err
		}
	}
	return result, nil
}

func permissionIDsFromBitSet(bits permission.BitSet) []int64 {
	ids := make([]int64, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if bits.Has(def.BitIndex) {
			ids = append(ids, int64(def.ID))
		}
	}
	return ids
}

func serverPermissionIDs() []int64 {
	ids := []int64{}
	for _, def := range permission.Definitions {
		if def.Scope.ID == permission.ScopeServer {
			ids = append(ids, int64(def.ID))
		}
	}
	return ids
}
