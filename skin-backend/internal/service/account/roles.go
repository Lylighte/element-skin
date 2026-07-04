package account

import (
	"context"
	"net/http"

	"element-skin/backend/internal/permission"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (s AccountService) GrantUserRole(ctx context.Context, actor permission.Actor, targetID, roleID string) error {
	if roleID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "role_id required"}
	}
	if err := actor.Require(permissionGrantAny); err != nil {
		return permissionDenied()
	}
	if err := ensureRoleMutationAllowed(actor, roleID); err != nil {
		return err
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	if err := s.ensureProtectedSubjectMutationAllowed(ctx, actor, targetID); err != nil {
		return err
	}
	if err := s.DB.Permissions.GrantRole(ctx, targetID, roleID, actor.SubjectID); err != nil {
		return err
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createRoleChangeNotice(ctx, targetID, roleID, "grant"); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s AccountService) RevokeUserRole(ctx context.Context, actor permission.Actor, targetID, roleID string) error {
	if roleID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "role_id required"}
	}
	if err := actor.Require(permissionRevokeAny); err != nil {
		return permissionDenied()
	}
	if err := ensureRoleMutationAllowed(actor, roleID); err != nil {
		return err
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	if err := s.ensureProtectedSubjectMutationAllowed(ctx, actor, targetID); err != nil {
		return err
	}
	ok, err := s.DB.Permissions.RevokeRole(ctx, targetID, roleID)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "role assignment not found"}
	}
	if err := s.reconcileOAuthAfterUserPermissionChange(ctx, targetID); err != nil {
		return err
	}
	if err := s.createRoleChangeNotice(ctx, targetID, roleID, "revoke"); err != nil {
		return err
	}
	return s.Redis.InvalidateAuthUser(ctx, targetID)
}

func (s AccountService) TransferProtectedSubject(ctx context.Context, actor permission.Actor, targetID string) error {
	if targetID == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "user_id required"}
	}
	if targetID == actor.UserID {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "cannot transfer protected subject to yourself"}
	}
	if err := actor.Require(manageProtectedPermission); err != nil {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected subject management required"}
	}
	isProtected, err := s.DB.Permissions.UserIsProtected(ctx, actor.UserID)
	if err != nil {
		return err
	}
	if !isProtected {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "protected subject ownership required"}
	}
	if ok, err := s.userExists(ctx, targetID); err != nil {
		return err
	} else if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "user not found"}
	}
	affectedUserIDs, err := s.DB.Permissions.TransferProtectedSubject(ctx, actor.UserID, targetID, actor.SubjectID)
	if err != nil {
		return err
	}
	for _, userID := range affectedUserIDs {
		if err := s.Redis.InvalidateAuthUser(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s AccountService) reconcileOAuthAfterUserPermissionChange(ctx context.Context, userID string) error {
	_, err := (oauthsvc.Service{DB: s.DB, Redis: s.Redis}).ReconcileUserPermissionDependents(ctx, userID)
	return err
}
