package yggdrasil

import (
	"context"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

const tokenTTL = 15 * 24 * time.Hour

var (
	yggSessionCreatePermission     = permission.MustDefinitionByCode("yggdrasil_session.create.owned")
	yggSessionRefreshPermission    = permission.MustDefinitionByCode("yggdrasil_session.refresh.owned")
	yggSessionValidatePermission   = permission.MustDefinitionByCode("yggdrasil_session.validate.owned")
	yggSessionInvalidatePermission = permission.MustDefinitionByCode("yggdrasil_session.invalidate.owned")
	yggSessionSignoutPermission    = permission.MustDefinitionByCode("yggdrasil_session.signout.owned")
)

func (y Yggdrasil) Authenticate(ctx context.Context, username, password, clientToken string, requestUser bool) (map[string]any, error) {
	u, loginProfile, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	if err := y.requireYggPermission(ctx, u.ID, yggSessionCreatePermission); err != nil {
		return nil, err
	}
	access, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if clientToken == "" {
		clientToken = access
	}
	profiles, err := y.DB.Profiles.GetByUser(ctx, u.ID, 100)
	if err != nil {
		return nil, err
	}
	var selected *model.Profile
	if loginProfile != nil {
		selected = loginProfile
	} else if len(profiles) == 1 {
		selected = &profiles[0]
	}
	var pid *string
	if selected != nil {
		pid = &selected.ID
	}
	createdAt := database.NowMS()
	if err := y.Redis.SetYggToken(ctx, model.Token{AccessToken: access, ClientToken: clientToken, UserID: u.ID, ProfileID: pid, CreatedAt: createdAt}, tokenTTL); err != nil {
		return nil, err
	}
	if err := y.Redis.TrimYggTokensByUser(ctx, u.ID, 5); err != nil {
		_ = y.Redis.DeleteYggToken(ctx, access)
		return nil, err
	}
	available := make([]map[string]any, 0, len(profiles))
	for _, p := range profiles {
		available = append(available, map[string]any{"id": p.ID, "name": p.Name})
	}
	resp := map[string]any{"accessToken": access, "clientToken": clientToken, "availableProfiles": available}
	if selected != nil {
		resp["selectedProfile"] = map[string]any{"id": selected.ID, "name": selected.Name}
	}
	if requestUser {
		resp["user"] = yggUserPayload(*u)
	}
	return resp, nil
}
