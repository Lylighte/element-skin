package yggdrasil

import (
	"context"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
)

func (y Yggdrasil) Join(ctx context.Context, access, profileID, serverID, ip string) error {
	t, err := y.DB.Tokens.Get(ctx, access)
	if err != nil {
		return err
	}
	if t == nil || t.ProfileID == nil || *t.ProfileID != profileID {
		return yggErr(403, "ForbiddenOperationException", "Invalid token.")
	}
	return y.DB.Tokens.ReplaceSession(ctx, model.Session{ServerID: serverID, AccessToken: access, IP: &ip, CreatedAt: database.NowMS()})
}

func (y Yggdrasil) HasJoined(ctx context.Context, username, serverID string) (map[string]any, int, error) {
	s, err := y.DB.Tokens.GetSession(ctx, serverID)
	if err != nil {
		return nil, 0, err
	}
	if s == nil || database.NowMS()-s.CreatedAt > 30000 {
		return nil, 204, nil
	}
	t, err := y.DB.Tokens.Get(ctx, s.AccessToken)
	if err != nil {
		return nil, 0, err
	}
	if t == nil || t.ProfileID == nil {
		return nil, 204, nil
	}
	p, err := y.DB.Profiles.GetByID(ctx, *t.ProfileID)
	if err != nil {
		return nil, 0, err
	}
	if p == nil || p.Name != username {
		return nil, 204, nil
	}
	if banned, err := y.DB.Users.IsBanned(ctx, p.UserID); err != nil {
		return nil, 0, err
	} else if banned {
		return nil, 0, yggErr(403, "ForbiddenOperationException", "Account is banned. Please contact administrator.")
	}
	body, err := y.ProfileJSON(*p, true)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}
