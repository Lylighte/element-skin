package profile

import (
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	settingssvc "element-skin/backend/internal/service/settings"
)

var (
	profileReadOwnedPermission   = permission.MustDefinitionByCode("profile.read.owned")
	profileReadAnyPermission     = permission.MustDefinitionByCode("profile.read.any")
	profileCreateOwnedPermission = permission.MustDefinitionByCode("profile.create.owned")
	profileUpdateOwnedPermission = permission.MustDefinitionByCode("profile.update.owned")
	profileUpdateAnyPermission   = permission.MustDefinitionByCode("profile.update.any")
	profileDeleteOwnedPermission = permission.MustDefinitionByCode("profile.delete.owned")
	profileDeleteAnyPermission   = permission.MustDefinitionByCode("profile.delete.any")
	textureClearOwnedPermission  = permission.MustDefinitionByCode("texture.clear.owned")
	textureClearBoundPermission  = permission.MustDefinitionByCode("texture.clear.bound_profile")
)

type Service struct {
	DB       *database.DB
	Settings settingssvc.Settings
}
