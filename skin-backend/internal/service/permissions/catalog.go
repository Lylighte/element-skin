package permissions

import (
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
)

func permissionCodesFromBitSet(bits permission.BitSet) []string {
	out := make([]string, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		if bits.Has(def.BitIndex) {
			out = append(out, def.Code)
		}
	}
	return out
}

func permissionOverrideResponses(overrides []permissiondb.SubjectPermissionOverride) []PermissionOverrideResponse {
	out := make([]PermissionOverrideResponse, 0, len(overrides))
	for _, override := range overrides {
		out = append(out, PermissionOverrideResponse{
			PermissionCode: override.PermissionCode,
			Effect:         override.Effect,
			CreatedAt:      override.CreatedAt,
		})
	}
	return out
}

func permissionCatalog() PermissionCatalogResponse {
	defs := make([]PermissionDefinitionResponse, 0, len(permission.Definitions))
	for _, def := range permission.Definitions {
		defs = append(defs, PermissionDefinitionResponse{
			ID:                  int64(def.ID),
			Code:                def.Code,
			Description:         def.Description,
			BitIndex:            def.BitIndex,
			Resource:            def.Resource.Code,
			ResourceDescription: def.Resource.Description,
			Action:              def.Action.Code,
			ActionDescription:   def.Action.Description,
			Scope:               def.Scope.Code,
			ScopeDescription:    def.Scope.Description,
		})
	}
	roles := make([]PermissionRoleResponse, 0, len(permission.Roles))
	for _, role := range permission.Roles {
		rolePermissions := make([]string, 0, len(role.Permissions))
		for _, def := range role.Permissions {
			rolePermissions = append(rolePermissions, def.Code)
		}
		roles = append(roles, PermissionRoleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
			SystemRole:  role.SystemRole,
			Protected:   role.Protected,
			Permissions: rolePermissions,
		})
	}
	return PermissionCatalogResponse{Permissions: defs, Roles: roles}
}
