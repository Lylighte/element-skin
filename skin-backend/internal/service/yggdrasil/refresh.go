package yggdrasil

import (
	"context"
	"errors"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (y Yggdrasil) Refresh(ctx context.Context, accessToken, clientToken, selectedID string, requestUser bool) (map[string]any, error) {
	t, err := y.Redis.GetYggToken(ctx, accessToken)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err != nil {
		return nil, err
	}
	if clientToken != "" && clientToken != t.ClientToken {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	if err := y.requireYggPermission(ctx, t.UserID, yggSessionRefreshPermission); err != nil {
		return nil, err
	}
	if ok, err := y.tokenProfileOwned(ctx, t); err != nil {
		return nil, err
	} else if !ok {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	newProfile := t.ProfileID
	var selected map[string]any
	if selectedID != "" {
		selectedID = util.StripUUIDDashes(selectedID)
		if t.ProfileID != nil {
			return nil, yggErr(400, "IllegalArgumentException", "Access token already has a profile assigned.")
		}
		ok, err := y.DB.Profiles.VerifyOwnership(ctx, t.UserID, selectedID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid profile.")
		}
		newProfile = &selectedID
	}
	if newProfile != nil {
		p, err := y.DB.Profiles.GetByID(ctx, *newProfile)
		if err != nil {
			return nil, err
		}
		if p == nil {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
		}
		selected = map[string]any{"id": p.ID, "name": p.Name}
	}
	var responseUser *model.User
	if requestUser {
		responseUser, err = y.DB.Users.GetByID(ctx, t.UserID)
		if err != nil {
			return nil, err
		}
		if responseUser == nil {
			return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
		}
	}
	newAccess, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	createdAt := database.NowMS()
	replaced, err := y.Redis.ReplaceYggToken(ctx, accessToken, model.Token{AccessToken: newAccess, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: newProfile, CreatedAt: createdAt}, tokenTTL)
	if err != nil {
		return nil, err
	}
	if !replaced {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	resp := map[string]any{"accessToken": newAccess, "clientToken": t.ClientToken}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	if responseUser != nil {
		resp["user"] = yggUserPayload(*responseUser)
	}
	return resp, nil
}
