package site

import (
	"context"
	"regexp"
	"strings"

	"element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/database/texture"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func (s Site) CreateProfile(ctx context.Context, userID, name, mdl string) (map[string]any, error) {
	if name == "" {
		return nil, util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return nil, util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p, err := s.DB.Profiles.GetByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色名已被占用，请换一个名称"}
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	mode, err := s.settings().Get(ctx, "profile_uuid_mode", "random")
	if err != nil {
		return nil, err
	}
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.Profiles.GetByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: 400, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	mdl = profile.NormalizeModel(mdl)
	if err := s.DB.Profiles.Create(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: mdl}); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": mdl}, nil
}

func (s Site) PublicLibrary(ctx context.Context, cursor string, limit int, typ, q, sort string) (map[string]any, error) {
	enabled, err := s.settings().Get(ctx, "enable_skin_library", "true")
	if err != nil {
		return nil, err
	}
	if enabled != "true" {
		return nil, util.HTTPError{Status: 403, Detail: "Skin library is disabled by administrator"}
	}
	lastCreated, lastHash, lastUsage, err := publicLibraryCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListPublic(ctx, texture.PublicListOptions{
		Limit:       limit,
		TextureType: typ,
		Query:       strings.TrimSpace(q),
		Sort:        texture.ParsePublicLibrarySort(sort),
		LastCreated: lastCreated,
		LastHash:    lastHash,
		LastUsage:   lastUsage,
	})
}

func (s Site) ListMyProfiles(ctx context.Context, userID, cursor string, limit int) (map[string]any, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	res, err := s.DB.Profiles.ListByUser(ctx, userID, limit, last)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Site) ListMyTextures(ctx context.Context, userID, cursor string, limit int, typ string) (map[string]any, error) {
	lastCreated, lastHash, err := textureCursor(cursor, "last_hash")
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "Invalid cursor"}
	}
	return s.DB.Textures.ListForUser(ctx, userID, typ, limit, lastCreated, lastHash)
}

func (s Site) AddTextureToWardrobe(ctx context.Context, userID, hash, textureType string) error {
	ok, err := s.DB.Textures.AddToWardrobe(ctx, userID, hash, textureType)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "Texture not found in library"}
	}
	return nil
}

func (s Site) UpdateProfile(ctx context.Context, userID, profileID, name string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	if name == "" {
		return util.HTTPError{Status: 400, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return util.HTTPError{Status: 400, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p.Name != name {
		existing, err := s.DB.Profiles.GetByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return util.HTTPError{Status: 400, Detail: "角色名已被占用"}
		}
	}
	_, err = s.DB.Profiles.UpdateName(ctx, profileID, name)
	return err
}

func (s Site) DeleteProfile(ctx context.Context, userID, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Site) ClearProfileTexture(ctx context.Context, userID, profileID, textureType string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	if p.UserID != userID {
		return util.HTTPError{Status: 403, Detail: "not allowed"}
	}
	return s.SetProfileTexture(ctx, profileID, textureType, nil)
}

func (s Site) SetProfileTexture(ctx context.Context, profileID, textureType string, hash *string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	switch strings.ToLower(textureType) {
	case "skin":
		if sameHash(p.SkinHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateSkin(ctx, profileID, hash); err != nil {
			return err
		}
	case "cape":
		if sameHash(p.CapeHash, hash) {
			return nil
		}
		if err := s.DB.Profiles.UpdateCape(ctx, profileID, hash); err != nil {
			return err
		}
	default:
		return util.HTTPError{Status: 400, Detail: "Invalid texture_type"}
	}
	return nil
}

func (s Site) DeleteProfileByID(ctx context.Context, profileID string) error {
	return s.deleteProfile(ctx, profileID)
}

func (s Site) DeleteUser(ctx context.Context, userID string) (bool, error) {
	textures, err := s.DB.Textures.ListForUser(ctx, userID, "", 10000, nil, "")
	if err != nil {
		return false, err
	}
	recountAfterDelete := make([]map[string]string, 0)
	ok, err := s.DB.Users.Delete(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, item := range textures["items"].([]map[string]any) {
		hash, _ := item["hash"].(string)
		textureType, _ := item["type"].(string)
		if hash == "" || textureType == "" {
			continue
		}
		uploader, exists, err := s.DB.Textures.LibraryUploader(ctx, hash, textureType)
		if err != nil {
			return false, err
		}
		if exists && uploader == userID {
			if err := s.DB.Textures.DeleteLibraryTexture(ctx, hash, textureType); err != nil {
				return false, err
			}
			continue
		}
		recountAfterDelete = append(recountAfterDelete, map[string]string{"hash": hash, "type": textureType})
	}
	if !ok {
		return false, nil
	}
	for _, item := range recountAfterDelete {
		if err := s.DB.Textures.RecountUsage(ctx, item["hash"], item["type"]); err != nil {
			return false, err
		}
	}
	return true, nil
}

func (s Site) deleteProfile(ctx context.Context, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	ok, err := s.DB.Profiles.DeleteCascade(ctx, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: 404, Detail: "profile not found"}
	}
	return nil
}

func sameHash(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
