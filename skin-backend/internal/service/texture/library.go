package texture

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	profilestore "element-skin/backend/internal/database/profile"
	texturedb "element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/permission"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

var (
	textureReadOwnedPermission             = permission.MustDefinitionByCode("texture.read.owned")
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

func (s LibraryService) PublicLibrary(ctx context.Context, cursor string, limit int, typ, q, sort string) (map[string]any, error) {
	enabled, err := s.Settings.Get(ctx, "enable_skin_library", "true")
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, util.HTTPError{Status: http.StatusForbidden, Detail: "Skin library is disabled by administrator"}
	}
	lastCreated, lastHash, lastUsage, err := publicLibraryCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	parsedSort := texturedb.ParsePublicLibrarySort(sort)
	if cursor != "" && (lastCreated == nil || lastHash == "" ||
		(parsedSort == texturedb.PublicLibrarySortMostUsed && lastUsage == nil)) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListPublic(ctx, texturedb.PublicListOptions{
		Limit:       limit,
		TextureType: typ,
		Query:       strings.TrimSpace(q),
		Sort:        parsedSort,
		LastCreated: lastCreated,
		LastHash:    lastHash,
		LastUsage:   lastUsage,
	})
}

func (s LibraryService) ListMyTextures(ctx context.Context, actor permission.Actor, cursor string, limit int, typ string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadOwnedPermission); err != nil {
		return nil, err
	}
	lastCreated, lastHash, err := textureCursor(cursor, "last_hash")
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	if cursor != "" && (lastCreated == nil || lastHash == "") {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListForUser(ctx, actor.UserID, typ, limit, lastCreated, lastHash)
}

func (s LibraryService) AddTextureToWardrobe(ctx context.Context, actor permission.Actor, hash, textureType string) error {
	if err := requireActorPermission(actor, wardrobeEntryAddPermission); err != nil {
		return err
	}
	ok, err := s.DB.Textures.AddToWardrobe(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "Texture not found in library"}
	}
	return nil
}

func (s LibraryService) ApplyTextureToProfile(ctx context.Context, actor permission.Actor, profileID, hash, textureType string) error {
	return s.applyTextureToProfile(ctx, actor, profileID, hash, textureType, nil)
}

func (s LibraryService) ApplyTextureToProfileWithModel(ctx context.Context, actor permission.Actor, profileID, hash, textureType, skinModel string) error {
	return s.applyTextureToProfile(ctx, actor, profileID, hash, textureType, &skinModel)
}

func (s LibraryService) applyTextureToProfile(ctx context.Context, actor permission.Actor, profileID, hash, textureType string, skinModel *string) error {
	if err := requireOwnedOrBoundProfilePermission(actor, profileID, textureApplyOwnedPermission, textureApplyBoundPermission); err != nil {
		return err
	}
	userID := actor.UserID
	owns, err := s.DB.Textures.VerifyOwnership(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !owns {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "Texture not found in your library"}
	}
	profileOwner, err := s.DB.Profiles.VerifyOwnership(ctx, userID, profileID)
	if err != nil {
		return err
	}
	if !profileOwner {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "Profile not yours"}
	}
	info, err := s.DB.Textures.GetInfo(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if info == nil {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "Texture info not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		modelName, _ := info["model"].(string)
		if skinModel != nil {
			modelName = *skinModel
		}
		return profileUpdateError(s.DB.Profiles.UpdateSkinAndModel(ctx, profileID, &hash, profilestore.NormalizeModel(modelName)))
	case "cape":
		return s.setProfileTexture(ctx, profileID, "cape", &hash)
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid texture_type"}
	}
}

func (s LibraryService) TextureDetail(ctx context.Context, actor permission.Actor, hash, textureType string) (map[string]any, error) {
	if err := requireActorPermission(actor, textureReadOwnedPermission); err != nil {
		return nil, err
	}
	info, err := s.DB.Textures.GetInfo(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "Texture not found"}
	}
	return info, nil
}

func (s LibraryService) UpdateTexture(ctx context.Context, actor permission.Actor, hash, textureType string, body map[string]any) (map[string]any, error) {
	var patch texturedb.Patch
	if model, ok := body["model"].(string); ok && model != "default" && model != "slim" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid model"}
	} else if ok {
		if err := requireActorPermission(actor, textureUpdateMetadataOwnedPermission); err != nil {
			return nil, err
		}
		patch.Model = &model
	}
	if note, ok := body["note"].(string); ok {
		if err := requireActorPermission(actor, textureUpdateMetadataOwnedPermission); err != nil {
			return nil, err
		}
		patch.Note = &note
	}
	if value, ok := body["is_public"]; ok {
		if err := requireActorPermission(actor, textureUpdateVisibilityOwnedPermission); err != nil {
			return nil, err
		}
		parsed := false
		switch x := value.(type) {
		case bool:
			parsed = x
		case float64:
			parsed = x != 0
		case int:
			parsed = x != 0
		default:
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid is_public"}
		}
		patch.IsPublic = &parsed
	}
	if patch.Note != nil || patch.Model != nil || patch.IsPublic != nil {
		if err := s.DB.Textures.UpdateForUser(ctx, actor.UserID, hash, textureType, patch); err != nil {
			return nil, textureNotFoundError(err)
		}
	}
	info, err := s.TextureDetail(ctx, actor, hash, textureType)
	if err != nil {
		return nil, err
	}
	info["ok"] = true
	return info, nil
}

func (s LibraryService) DeleteTexture(ctx context.Context, actor permission.Actor, hash, textureType string) error {
	if err := requireActorPermission(actor, textureDeleteOwnedPermission); err != nil {
		return err
	}
	uploader, exists, err := s.DB.Textures.LibraryUploader(ctx, hash, textureType)
	if err != nil {
		return err
	}
	if exists && uploader == actor.UserID {
		return textureNotFoundError(s.DB.Textures.DeleteLibraryTexture(ctx, hash, textureType))
	}
	deleted, err := s.DB.Textures.DeleteFromLibrary(ctx, actor.UserID, hash, textureType)
	if err != nil {
		return err
	}
	if !deleted {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "Texture not found"}
	}
	return nil
}

func (s LibraryService) setProfileTexture(ctx context.Context, profileID, textureType string, hash *string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		if sameHash(p.SkinHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateSkin(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	case "cape":
		if sameHash(p.CapeHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateCape(ctx, profileID, hash); err != nil {
			return profileUpdateError(err)
		}
	default:
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid texture_type"}
	}
	return nil
}

func requireActorPermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func requireOwnedOrBoundProfilePermission(actor permission.Actor, profileID string, owned, bound permission.Definition) error {
	if actor.Has(owned) {
		return nil
	}
	if actor.BoundProfileID == profileID && actor.Has(bound) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func requireBoundProfilePermission(actor permission.Actor, profileID string, def permission.Definition) error {
	if actor.BoundProfileID == profileID && actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}

func textureNotFoundError(err error) error {
	if errors.Is(err, texturedb.ErrNotFound) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "Texture not found"}
	}
	return err
}

func textureCursor(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m[hashKey].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", errors.New("invalid cursor")
	}
	created := &value
	return created, h, nil
}

func publicLibraryCursor(cursor string) (*int64, string, *int64, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", nil, err
	}
	createdValue, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m["last_skin_hash"].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", nil, errors.New("invalid cursor")
	}
	created := &createdValue
	var usage *int64
	if rawUsage, exists := m["last_usage_count"]; exists {
		usageValue, ok := util.CursorInt64(rawUsage)
		if !ok {
			return nil, "", nil, errors.New("invalid cursor")
		}
		usage = &usageValue
	}
	return created, h, usage, nil
}

func sameHash(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func profileUpdateError(err error) error {
	if database.IsNoRows(err) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	return err
}
