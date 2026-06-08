package yggdrasil

import (
	"context"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func (y Yggdrasil) Authenticate(ctx context.Context, username, password, clientToken string, requestUser bool) (map[string]any, error) {
	u, loginProfile, err := y.verifyCredentials(ctx, username, password)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid credentials. Invalid username or password.")
	}
	access, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if clientToken == "" {
		clientToken = access
	}
	var profiles []model.Profile
	var selected *model.Profile
	if loginProfile != nil {
		profiles = []model.Profile{*loginProfile}
		selected = loginProfile
	} else {
		profiles, err = y.DB.Profiles.GetByUser(ctx, u.ID, 100)
		if err != nil {
			return nil, err
		}
		if len(profiles) == 1 {
			selected = &profiles[0]
		}
	}
	var pid *string
	if selected != nil {
		pid = &selected.ID
	}
	if err := y.DB.Tokens.Add(ctx, model.Token{AccessToken: access, ClientToken: clientToken, UserID: u.ID, ProfileID: pid, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	_ = y.DB.Tokens.Cleanup(ctx, u.ID, database.NowMS()-15*24*3600*1000, 5)
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

func (y Yggdrasil) verifyCredentials(ctx context.Context, username, password string) (*model.User, *model.Profile, error) {
	u, err := y.DB.Users.GetByEmail(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	var p *model.Profile
	if u == nil {
		p, err = y.DB.Profiles.GetByName(ctx, username)
		if err != nil {
			return nil, nil, err
		}
		if p != nil {
			u, err = y.DB.Users.GetByID(ctx, p.UserID)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if u == nil || !util.VerifyPassword(password, u.Password) {
		return nil, nil, nil
	}
	return u, p, nil
}

func (y Yggdrasil) Refresh(ctx context.Context, accessToken, clientToken, selectedID string, requestUser bool) (map[string]any, error) {
	t, err := y.DB.Tokens.Get(ctx, accessToken)
	if err != nil {
		return nil, err
	}
	if t == nil || (clientToken != "" && clientToken != t.ClientToken) {
		return nil, yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	_ = y.DB.Tokens.Delete(ctx, accessToken)
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
		p, _ := y.DB.Profiles.GetByID(ctx, *newProfile)
		if p != nil {
			selected = map[string]any{"id": p.ID, "name": p.Name}
		}
	}
	newAccess, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	if err := y.DB.Tokens.Add(ctx, model.Token{AccessToken: newAccess, ClientToken: t.ClientToken, UserID: t.UserID, ProfileID: newProfile, CreatedAt: database.NowMS()}); err != nil {
		return nil, err
	}
	resp := map[string]any{"accessToken": newAccess, "clientToken": t.ClientToken}
	if selected != nil {
		resp["selectedProfile"] = selected
	}
	if requestUser {
		u, _ := y.DB.Users.GetByID(ctx, t.UserID)
		if u != nil {
			resp["user"] = yggUserPayload(*u)
		}
	}
	return resp, nil
}

func (y Yggdrasil) Validate(ctx context.Context, access, client string) error {
	t, err := y.DB.Tokens.Get(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || (client != "" && client != t.ClientToken) || database.NowMS()-t.CreatedAt > 15*24*3600*1000 {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return nil
}

func yggUserPayload(u model.User) map[string]any {
	return map[string]any{"id": u.ID, "properties": []map[string]any{{"name": "preferredLanguage", "value": u.PreferredLanguage}}}
}
