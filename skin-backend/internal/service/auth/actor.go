package auth

import (
	"context"
	"errors"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (s Service) ActorForWebAccessToken(ctx context.Context, token string) (permission.Actor, bool, error) {
	claims, ok := util.DecodeAccessToken(s.Cfg.JWTSecret, token)
	if !ok {
		return permission.Actor{}, false, nil
	}
	userID, _ := claims["sub"].(string)
	if userID == "" {
		return permission.Actor{}, false, nil
	}

	authUser, err := s.Redis.GetAuthUser(ctx, userID)
	if errors.Is(err, redisstore.ErrCacheMiss) {
		user, dbErr := s.DB.Users.GetByID(ctx, userID)
		if dbErr != nil {
			return permission.Actor{}, false, dbErr
		}
		if user == nil {
			return permission.Actor{}, false, nil
		}
		authUser = redisstore.AuthUserFromModel(*user)
		if setErr := s.Redis.SetAuthUser(ctx, authUser, time.Duration(s.Cfg.AuthCacheTTL)*time.Second); setErr != nil {
			return permission.Actor{}, false, setErr
		}
	} else if err != nil {
		return permission.Actor{}, false, err
	}

	actor, err := s.DB.Permissions.ActorForUser(ctx, authUser.ID, permissiondb.EffectiveOptions{
		SessionKind: permission.SessionKindWeb,
		Entrypoint:  permission.EntrypointDashboard,
	})
	if err != nil {
		return permission.Actor{}, false, err
	}
	return actor, true, nil
}
