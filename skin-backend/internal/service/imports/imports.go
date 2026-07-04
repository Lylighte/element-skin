package imports

import (
	"context"
	"net/http"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
)

type TextureAsset struct {
	URL     string
	Kind    string
	Variant string
}

type ImportService struct {
	DB              *database.DB
	TexturesDir     string
	HTTPClient      *http.Client
	DownloadTexture func(context.Context, string) ([]byte, error)
	ProcessTexture  func([]byte, string) (string, error)
}

var (
	profileCreateOwnedPermission = permission.MustDefinitionByCode("profile.create.owned")
	textureCreateOwnedPermission = permission.MustDefinitionByCode("texture.create.owned")
)
