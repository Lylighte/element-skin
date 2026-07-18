package oauth

import (
	"context"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func (s Service) ClientPermissions(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("permission.read.any")); err != nil {
		return nil, forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return nil, err
	}
	if client == nil {
		return nil, notFound("oauth client not found")
	}
	subjectID := permissiondb.SubjectIDForClient(client.ID)
	effective, err := s.DB.Permissions.EffectivePermissionsForClient(ctx, client.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		return nil, err
	}
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForSubject(ctx, subjectID)
	if err != nil {
		return nil, err
	}
	clientScopes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	overrideItems := make([]map[string]any, 0, len(overrides))
	for _, item := range overrides {
		overrideItems = append(overrideItems, map[string]any{
			"permission_code": item.PermissionCode,
			"effect":          item.Effect,
			"created_at":      item.CreatedAt,
		})
	}
	return map[string]any{
		"subject_id":             subjectID,
		"client":                 publicClient(*client),
		"effective_permissions":  permissionCodesFromBitSet(effective),
		"overrides":              overrideItems,
		"client_allowed_scopes":  clientScopes,
		"session_allowed_scopes": clientCredentialsPolicyCodes(),
	}, nil
}

func (s Service) SetClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code, effect string) error {
	if effect == "allow" {
		if err := actor.Require(permission.MustDefinitionByCode("permission.grant.any")); err != nil {
			return forbidden()
		}
	} else {
		if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
			return forbidden()
		}
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth client not found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok || def.Scope.ID == permission.ScopeSystem {
		return badRequest("invalid permission")
	}
	if err := s.DB.Permissions.SetPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), def, effect, actor.SubjectID); err != nil {
		return err
	}
	return s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID)
}

func (s Service) ClearClientPermissionOverride(ctx context.Context, actor permission.Actor, clientID, code string) error {
	if err := actor.Require(permission.MustDefinitionByCode("permission.revoke.any")); err != nil {
		return forbidden()
	}
	client, err := s.DB.OAuth.GetClient(ctx, clientID)
	if err != nil {
		return err
	}
	if client == nil {
		return notFound("oauth client not found")
	}
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return badRequest("invalid permission")
	}
	ok, err = s.DB.Permissions.ClearPermissionOverrideForSubject(ctx, permissiondb.SubjectIDForClient(client.ID), def)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("permission override not found")
	}
	return s.Redis.DeleteOAuthAccessTokensByClient(ctx, client.ID)
}

func (s Service) grantReviewedClientPermissions(ctx context.Context, actor permission.Actor, clientID string, codes []string) error {
	subjectID := permissiondb.SubjectIDForClient(clientID)
	for _, code := range codes {
		def, ok := permission.DefinitionByCode(code)
		if !ok || def.Scope.ID == permission.ScopeSystem {
			return badRequest("invalid permission")
		}
		if !isAppOnlyPermission(def) {
			continue
		}
		if err := s.DB.Permissions.SetPermissionOverrideForSubject(ctx, subjectID, def, "allow", actor.SubjectID); err != nil {
			return err
		}
	}
	return nil
}

func isAppOnlyPermission(def permission.Definition) bool {
	return def.Scope.ID == permission.ScopeAny || def.Scope.ID == permission.ScopePublic || def.Scope.ID == permission.ScopeServer
}
