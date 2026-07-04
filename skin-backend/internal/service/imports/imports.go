package imports

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	texturesvc "element-skin/backend/internal/service/texture"
	"element-skin/backend/internal/util"
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

func (s ImportService) ImportProfile(ctx context.Context, actor permission.Actor, profileID, profileName string, assets []TextureAsset) (map[string]any, error) {
	if !actor.Has(profileCreateOwnedPermission) {
		return nil, util.HTTPError{Status: 403, Detail: "permission denied"}
	}
	if hasTextureAsset(assets) && !actor.Has(textureCreateOwnedPermission) {
		return nil, util.HTTPError{Status: 403, Detail: "permission denied"}
	}
	if profileID == "" || profileName == "" {
		return nil, util.HTTPError{Status: 400, Detail: "profile_id and profile_name are required"}
	}
	userID := actor.UserID
	if !util.ValidProfileName(profileName) {
		return nil, util.HTTPError{Status: 400, Detail: "invalid profile name"}
	}
	existing, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, util.HTTPError{Status: 400, Detail: "UUID already exists"}
	}
	modelName := "default"
	var skinHash *string
	var capeHash *string
	for _, asset := range assets {
		asset.Kind = normalizedImportTextureKind(asset.Kind)
		if asset.Kind == "" {
			continue
		}
		if asset.URL == "" {
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

	for attempt := 0; attempt < 100; attempt++ {
		name := util.ProfileNameCandidate(profileName, attempt)
		existing, err := s.DB.Profiles.GetByName(ctx, name)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			continue
		}

		p := model.Profile{ID: profileID, UserID: userID, Name: name, TextureModel: modelName, SkinHash: skinHash, CapeHash: capeHash}
		err = s.DB.Profiles.Create(ctx, p)
		switch {
		case err == nil:
			return map[string]any{"ok": true, "profile": profile.Summary(p)}, nil
		case profile.IsNameConflict(err):
			continue
		case profile.IsIDConflict(err):
			return nil, util.HTTPError{Status: 400, Detail: "UUID already exists"}
		default:
			return nil, err
		}
	}
	return nil, util.HTTPError{Status: 500, Detail: "无法生成唯一角色名"}
}

func (s ImportService) ImportProfiles(ctx context.Context, actor permission.Actor, profiles []map[string]string, fetch func(context.Context, string) ([]TextureAsset, error)) map[string]any {
	var items []map[string]any
	var failed []map[string]any
	for _, p := range profiles {
		id := p["profile_id"]
		name := p["profile_name"]
		if id == "" || name == "" {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "profile_id and profile_name are required"})
			continue
		}
		assets, err := fetch(ctx, id)
		if err != nil {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "导入失败"})
			continue
		}
		res, err := s.ImportProfile(ctx, actor, id, name, assets)
		if err != nil {
			detail := "导入失败"
			if he, ok := err.(util.HTTPError); ok {
				detail = he.Detail
			}
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": detail})
			continue
		}
		items = append(items, res["profile"].(map[string]any))
	}
	return map[string]any{
		"success_count": len(items),
		"failure_count": len(failed),
		"items":         items,
		"failed":        failed,
	}
}

func hasTextureAsset(assets []TextureAsset) bool {
	for _, asset := range assets {
		if asset.URL != "" && normalizedImportTextureKind(asset.Kind) != "" {
			return true
		}
	}
	return false
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
