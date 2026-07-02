package profile

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"element-skin/backend/internal/database"
	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	settingssvc "element-skin/backend/internal/service/settings"
	"element-skin/backend/internal/util"
)

var (
	profileReadOwnedPermission   = permission.MustDefinitionByCode("profile.read.owned")
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

func (s Service) CreateProfile(ctx context.Context, actor permission.Actor, name, modelName string) (map[string]any, error) {
	if err := requireActorPermission(actor, profileCreateOwnedPermission); err != nil {
		return nil, err
	}
	userID := actor.UserID
	if name == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p, err := s.DB.Profiles.GetByName(ctx, name); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名已被占用，请换一个名称"}
	}
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	mode, err := s.Settings.Get(ctx, "profile_uuid_mode", "random")
	if err != nil {
		return nil, err
	}
	if mode == "offline" {
		id = util.OfflineUUIDNoDash(name)
	}
	if p, err := s.DB.Profiles.GetByID(ctx, id); err != nil {
		return nil, err
	} else if p != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "角色 UUID 冲突，无法新建角色"}
	}
	modelName = profilestore.NormalizeModel(modelName)
	if err := s.DB.Profiles.Create(ctx, model.Profile{ID: id, UserID: userID, Name: name, TextureModel: modelName}); err != nil {
		if profilestore.IsNameConflict(err) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名已被占用，请换一个名称"}
		}
		if profilestore.IsIDConflict(err) {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "角色 UUID 冲突，无法新建角色"}
		}
		return nil, err
	}
	return map[string]any{"id": id, "name": name, "model": modelName}, nil
}

func (s Service) ListMyProfiles(ctx context.Context, actor permission.Actor, cursor string, limit int) (map[string]any, error) {
	if err := requireActorPermission(actor, profileReadOwnedPermission); err != nil {
		return nil, err
	}
	m, err := util.DecodeCursor(cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	last := ""
	if m != nil {
		last, _ = m["last_id"].(string)
	}
	if cursor != "" && last == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	res, err := s.DB.Profiles.ListByUser(ctx, actor.UserID, limit, last)
	if err != nil {
		return nil, err
	}
	res["next_cursor"] = util.EncodeCursor(asCursorMap(res["next_key"]))
	delete(res, "next_key")
	return res, nil
}

func (s Service) UpdateProfile(ctx context.Context, actor permission.Actor, profileID, name string) error {
	if err := requireActorPermission(actor, profileUpdateOwnedPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	if p.UserID != actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "not allowed"}
	}
	if name == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "name required"}
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_]{1,16}$`).MatchString(name) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名只能包含字母、数字、下划线，长度1-16字符"}
	}
	if p.Name != name {
		existing, err := s.DB.Profiles.GetByName(ctx, name)
		if err != nil {
			return err
		}
		if existing != nil {
			return util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名已被占用"}
		}
	}
	updated, err := s.DB.Profiles.UpdateName(ctx, profileID, name)
	if profilestore.IsNameConflict(err) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "角色名已被占用"}
	}
	if err != nil {
		return err
	}
	if !updated {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	return nil
}

func (s Service) DeleteProfile(ctx context.Context, actor permission.Actor, profileID string) error {
	if err := requireActorPermission(actor, profileDeleteOwnedPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	if p.UserID != actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "not allowed"}
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Service) DeleteProfileByID(ctx context.Context, actor permission.Actor, profileID string) error {
	if err := requireActorPermission(actor, profileDeleteAnyPermission); err != nil {
		return err
	}
	return s.deleteProfile(ctx, profileID)
}

func (s Service) ClearProfileTexture(ctx context.Context, actor permission.Actor, profileID, textureType string) error {
	if err := requireOwnedOrBoundProfilePermission(actor, profileID, textureClearOwnedPermission, textureClearBoundPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	if p.UserID != actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "not allowed"}
	}
	return s.setProfileTexture(ctx, profileID, textureType, nil)
}

func (s Service) SetProfileTexture(ctx context.Context, actor permission.Actor, profileID, textureType string, hash *string) error {
	if err := requireActorPermission(actor, profileUpdateAnyPermission); err != nil {
		return err
	}
	return s.setProfileTexture(ctx, profileID, textureType, hash)
}

func (s Service) setProfileTexture(ctx context.Context, profileID, textureType string, hash *string) error {
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

func (s Service) deleteProfile(ctx context.Context, profileID string) error {
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	ok, err := s.DB.Profiles.DeleteCascade(ctx, profileID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
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

func asCursorMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}
