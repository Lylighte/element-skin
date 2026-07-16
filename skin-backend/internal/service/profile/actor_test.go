package profile_test

import "element-skin/backend/internal/permission"

func testActorWithCodes(userID string, codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		def := permission.MustDefinitionByCode(code)
		bits.Set(def.BitIndex)
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}

func testUserActor(userID string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, role := range permission.Roles {
		if role.ID != permission.RoleUser {
			continue
		}
		for _, def := range role.Permissions {
			bits.Set(def.BitIndex)
		}
	}
	return permission.Actor{
		SubjectID:   "user:" + userID,
		UserID:      userID,
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
		Permissions: bits,
	}
}
