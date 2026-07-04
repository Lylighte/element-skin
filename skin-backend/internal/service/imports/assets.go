package imports

import (
	"context"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
)

func hasTextureAsset(assets []TextureAsset) bool {
	for _, asset := range assets {
		if asset.URL != "" && normalizedImportTextureKind(asset.Kind) != "" {
			return true
		}
	}
	return false
}

func (s ImportService) importTextureAssets(ctx context.Context, actor permission.Actor, assets []TextureAsset) (*string, *string, string) {
	modelName := "default"
	var skinHash *string
	var capeHash *string
	for _, asset := range assets {
		asset.Kind = normalizedImportTextureKind(asset.Kind)
		if asset.Kind == "" || asset.URL == "" {
			continue
		}
		data, err := s.download(ctx, asset.URL)
		if err != nil {
			continue
		}
		hash, err := s.process(ctx, actor, data, asset)
		if err != nil {
			continue
		}
		if asset.Kind == "skin" {
			skinHash = &hash
			if asset.Variant == "slim" {
				modelName = "slim"
			}
		}
		if asset.Kind == "cape" {
			capeHash = &hash
		}
	}
	return skinHash, capeHash, modelName
}

func (s ImportService) download(ctx context.Context, rawURL string) ([]byte, error) {
	if s.DownloadTexture != nil {
		return s.DownloadTexture(ctx, rawURL)
	}
	return util.DownloadTexture(s.HTTPClient, rawURL, 0)
}

func (s ImportService) process(ctx context.Context, actor permission.Actor, data []byte, asset TextureAsset) (string, error) {
	if s.ProcessTexture != nil {
		return s.ProcessTexture(data, asset.Kind)
	}
	storage, err := texturesvc.NewTextureStorage(s.TexturesDir)
	if err != nil {
		return "", err
	}
	hash, created, err := storage.ProcessAndSaveTracked(data, asset.Kind)
	if err != nil {
		return "", err
	}
	modelName := "default"
	if asset.Kind == "skin" {
		modelName = profile.NormalizeModel(asset.Variant)
	}
	if err := s.DB.Textures.AddToLibrary(ctx, actor.UserID, hash, asset.Kind, "", false, modelName); err != nil {
		if created {
			if inUse, checkErr := s.DB.Textures.ExistsHash(ctx, hash); checkErr == nil && !inUse {
				_ = storage.DeleteFile(hash)
			}
		}
		return "", err
	}
	return hash, nil
}

func normalizedImportTextureKind(raw string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if kind != "skin" && kind != "cape" {
		return ""
	}
	return kind
}
