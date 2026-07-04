package profile

import (
	"context"
	"net/http"
	"regexp"

	profilestore "element-skin/backend/internal/database/profile"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

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

func (s Service) UpdateAnyProfile(ctx context.Context, actor permission.Actor, profileID, name string) error {
	if err := requireActorPermission(actor, profileUpdateAnyPermission); err != nil {
		return err
	}
	p, err := s.DB.Profiles.GetByID(ctx, profileID)
	if err != nil {
		return err
	}
	if p == nil {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "profile not found"}
	}
	if name == "" {
		return nil
	}
	if !util.ValidProfileName(name) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid profile name"}
	}
	ok, err := s.DB.Profiles.UpdateName(ctx, profileID, name)
	if profilestore.IsNameConflict(err) {
		return util.HTTPError{Status: http.StatusConflict, Detail: "profile name already exists"}
	}
	if err != nil {
		return err
	}
	if !ok {
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
