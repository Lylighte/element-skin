package permissions

import (
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
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
