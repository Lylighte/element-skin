package account

import (
	"context"
	"net/http"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s AccountService) modifiableUser(ctx context.Context, actor permission.Actor, targetID string) (*model.User, error) {
	target, err := s.DB.Users.GetByID(ctx, targetID)
	if err != nil {
		return nil, err
	}
	if target == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return nil, util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify protected subject"}
	}
	return target, nil
}

func (s AccountService) ensureProtectedSubjectMutationAllowed(ctx context.Context, actor permission.Actor, targetID string) error {
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, targetID)
	if err != nil {
		return err
	}
	if isProtected && !actor.Has(manageProtectedPermission) {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot modify protected subject"}
	}
	return nil
}

func (s AccountService) userExists(ctx context.Context, userID string) (bool, error) {
	user, err := s.DB.Users.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user != nil, nil
}

func ensureRoleMutationAllowed(actor permission.Actor, roleID string) error {
	if roleID == permission.RoleSystemMaintenance {
		if !actor.Has(manageProtectedPermission) {
			return util.HTTPError{Status: http.StatusForbidden, Detail: "protected role management required"}
		}
	}
	return nil
}

func permissionDenied() error {
	return util.HTTPError{Status: http.StatusForbidden, Detail: "permission denied"}
}
