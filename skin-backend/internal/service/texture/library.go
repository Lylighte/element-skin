package texture

import (
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	settingssvc "element-skin/backend/internal/service/settings"
)

var (
	textureReadOwnedPermission             = permission.MustDefinitionByCode("texture.read.owned")
	textureReadPublicPermission            = permission.MustDefinitionByCode("texture.read.public")
	textureUpdateMetadataOwnedPermission   = permission.MustDefinitionByCode("texture.update_metadata.owned")
	textureUpdateVisibilityOwnedPermission = permission.MustDefinitionByCode("texture.update_visibility.owned")
	textureDeleteOwnedPermission           = permission.MustDefinitionByCode("texture.delete.owned")
	textureApplyOwnedPermission            = permission.MustDefinitionByCode("texture.apply.owned")
	textureApplyBoundPermission            = permission.MustDefinitionByCode("texture.apply.bound_profile")
	wardrobeEntryAddPermission             = permission.MustDefinitionByCode("wardrobe_entry.add.owned")
)

type LibraryService struct {
	DB       *database.DB
	Settings settingssvc.Settings
}
