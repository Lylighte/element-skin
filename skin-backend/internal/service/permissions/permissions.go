package permissions

import (
	"context"
	"net/http"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

var (
	manageProtectedPermission = permission.MustDefinitionByCode("permission_protected.manage.any")
	permissionReadAny         = permission.MustDefinitionByCode("permission.read.any")
	permissionGrantAny        = permission.MustDefinitionByCode("permission.grant.any")
	permissionRevokeAny       = permission.MustDefinitionByCode("permission.revoke.any")
)

type PermissionService struct {
	DB    *database.DB
	Redis redisstore.Store
}

type PermissionDefinitionResponse struct {
	ID                  int64  `json:"id"`
	Code                string `json:"code"`
	Description         string `json:"description"`
	BitIndex            int    `json:"bit_index"`
	Resource            string `json:"resource"`
	ResourceDescription string `json:"resource_description"`
	Action              string `json:"action"`
	ActionDescription   string `json:"action_description"`
	Scope               string `json:"scope"`
	ScopeDescription    string `json:"scope_description"`
}

type PermissionRoleResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SystemRole  bool     `json:"system_role"`
	Protected   bool     `json:"protected"`
	Permissions []string `json:"permissions"`
}

type PermissionOverrideResponse struct {
	PermissionCode string `json:"permission_code"`
	Effect         string `json:"effect"`
	CreatedAt      int64  `json:"created_at"`
}

type UserPermissionsResponse struct {
	Roles                []string                     `json:"roles"`
	Protected            bool                         `json:"protected"`
	EffectivePermissions []string                     `json:"effective_permissions"`
	Overrides            []PermissionOverrideResponse `json:"overrides"`
	Catalog              PermissionCatalogResponse    `json:"catalog"`
}

type PermissionCatalogResponse struct {
	Permissions []PermissionDefinitionResponse `json:"permissions"`
	Roles       []PermissionRoleResponse       `json:"roles"`
}

func (s PermissionService) UserPermissions(ctx context.Context, actor permission.Actor, targetID string) (UserPermissionsResponse, error) {
	if err := actor.Require(permissionReadAny); err != nil {
		return UserPermissionsResponse{}, permissionDenied()
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return UserPermissionsResponse{}, err
	} else if !ok {
		return UserPermissionsResponse{}, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	roles, err := s.DB.Permissions.RoleIDsForUser(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	bits, err := s.DB.Permissions.EffectivePermissionsForUser(ctx, targetID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForUser(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	protected, err := s.DB.Permissions.UserIsProtected(ctx, targetID)
	if err != nil {
		return UserPermissionsResponse{}, err
	}
	return UserPermissionsResponse{
		Roles:                roles,
		Protected:            protected,
		EffectivePermissions: permissionCodesFromBitSet(bits),
		Overrides:            permissionOverrideResponses(overrides),
		Catalog:              permissionCatalog(),
	}, nil
}

func (s PermissionService) SetUserPermissionOverride(ctx context.Context, actor permission.Actor, targetID, code, effect string) error {
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "permission not found"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	switch effect {
	case "allow":
		if err := actor.Require(permissionGrantAny); err != nil {
			return permissionDenied()
		}
	case "deny":
		if err := actor.Require(permissionRevokeAny); err != nil {
			return permissionDenied()
		}
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "effect must be allow or deny"}
	}
	if err := ensurePermissionOverrideAllowed(actor, targetID, def); err != nil {
		return err
	}
	if err := s.DB.Permissions.SetSubjectPermissionOverride(ctx, targetID, def, effect, actor.SubjectID); err != nil {
		return err
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s PermissionService) ClearUserPermissionOverride(ctx context.Context, actor permission.Actor, targetID, code string) error {
	def, ok := permission.DefinitionByCode(code)
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "permission not found"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	effect, err := s.permissionOverrideEffect(ctx, targetID, code)
	if err != nil {
		return err
	}
	if effect == "" {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "permission override not found"}
	}
	if effect == "allow" {
		if err := actor.Require(permissionRevokeAny); err != nil {
			return permissionDenied()
		}
	} else if err := actor.Require(permissionGrantAny); err != nil {
		return permissionDenied()
	}
	if err := ensurePermissionOverrideAllowed(actor, targetID, def); err != nil {
		return err
	}
	cleared, err := s.DB.Permissions.ClearSubjectPermissionOverride(ctx, targetID, def)
	if err != nil {
		return err
	}
	if !cleared {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "permission override not found"}
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s PermissionService) reconcileOAuthAfterUserPermissionChange(ctx context.Context, userID string) error {
	_, err := (oauthsvc.Service{DB: s.DB, Redis: s.Redis}).ReconcileUserPermissionDependents(ctx, userID)
	return err
}

func (s PermissionService) permissionOverrideEffect(ctx context.Context, userID, code string) (string, error) {
	overrides, err := s.DB.Permissions.SubjectPermissionOverridesForUser(ctx, userID)
	if err != nil {
		return "", err
	}
	for _, override := range overrides {
		if override.PermissionCode == code {
			return override.Effect, nil
		}
	}
	return "", nil
}

func (s PermissionService) userExists(ctx context.Context, userID string) (bool, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func ensurePermissionOverrideAllowed(actor permission.Actor, targetID string, def permission.Definition) error {
	if def.Code == manageProtectedPermission.Code {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "protected management permission must be transferred"}
	}
	if !protectedPermission(def) {
		return nil
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify protected permission on yourself"}
	}
	if !actor.Has(manageProtectedPermission) {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected permission management required"}
	}
	return nil
}

func protectedPermission(def permission.Definition) bool {
	return def.Scope.ID == permission.ScopeSystem || def.Resource.ID == permission.ResourcePermissionProtected
}

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

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}
