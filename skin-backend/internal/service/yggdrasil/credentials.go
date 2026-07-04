package yggdrasil

import (
	"context"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

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
