package remote_test

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
)

func withUserActor(req *http.Request, userID string) *http.Request {
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, rolePermissions(permission.RoleUser)...))
}

func withUserActorWithoutPermission(req *http.Request, userID, excludeCode string) *http.Request {
	defs := make([]permission.Definition, 0, len(rolePermissions(permission.RoleUser)))
	for _, def := range rolePermissions(permission.RoleUser) {
		if def.Code != excludeCode {
			defs = append(defs, def)
		}
	}
	return req.WithContext(shared.WithActorPermissions(req.Context(), userID, defs...))
}

func rolePermissions(roleID string) []permission.Definition {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return role.Permissions
		}
	}
	panic("missing role: " + roleID)
}
