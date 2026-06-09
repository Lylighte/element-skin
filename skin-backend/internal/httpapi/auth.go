package httpapi

import (
	"errors"
	"net/http"
	"time"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/util"
)

func (r *Router) auth(next http.HandlerFunc, requireAdmin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		cookie, err := req.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		claims, ok := util.DecodeAccessToken(r.cfg.JWTSecret, cookie.Value)
		if !ok {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		userID, _ := claims["sub"].(string)
		authUser, err := r.redis.GetAuthUser(req.Context(), userID)
		if errors.Is(err, redisstore.ErrCacheMiss) {
			user, dbErr := r.db.Users.GetByID(req.Context(), userID)
			if dbErr != nil {
				util.Error(w, dbErr)
				return
			}
			if user == nil {
				util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
				return
			}
			authUser = redisstore.AuthUserFromModel(*user)
			if setErr := r.redis.SetAuthUser(req.Context(), authUser, time.Duration(r.cfg.AuthCacheTTL)*time.Second); setErr != nil {
				util.Error(w, setErr)
				return
			}
		} else if err != nil {
			util.Error(w, err)
			return
		}
		if authUser.Banned(time.Now()) {
			util.Error(w, util.HTTPError{Status: 403, Detail: "user is banned"})
			return
		}
		if requireAdmin && !authUser.IsAdmin {
			util.Error(w, util.HTTPError{Status: 403, Detail: "admin required"})
			return
		}
		next(w, req.WithContext(shared.WithUser(req.Context(), authUser.ID, authUser.IsAdmin, authUser.IsSuperAdmin)))
	}
}
