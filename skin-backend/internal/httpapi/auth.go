package httpapi

import (
	"net/http"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	authsvc "element-skin/backend/internal/service/auth"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (r *Router) auth(next http.HandlerFunc, required ...permission.Definition) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if bearer, ok := shared.BearerToken(req); ok {
			actor, authenticated, err := (oauthsvc.Service{DB: r.db, Redis: r.redis}).ActorForBearer(req.Context(), bearer)
			if err != nil {
				util.Error(w, err)
				return
			}
			if !authenticated {
				util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
				return
			}
			for _, def := range required {
				if err := actor.Require(def); err != nil {
					util.Error(w, util.HTTPError{Status: 403, Detail: "permission denied"})
					return
				}
			}
			next(w, req.WithContext(shared.WithActor(req.Context(), actor)))
			return
		}
		cookie, err := req.Cookie("access_token")
		if err != nil || cookie.Value == "" {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		actor, authenticated, err := (authsvc.Service{DB: r.db, Cfg: r.cfg, Redis: r.redis}).ActorForWebAccessToken(req.Context(), cookie.Value)
		if err != nil {
			util.Error(w, err)
			return
		}
		if !authenticated {
			util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
			return
		}
		for _, def := range required {
			if err := actor.Require(def); err != nil {
				util.Error(w, util.HTTPError{Status: 403, Detail: "permission denied"})
				return
			}
		}
		next(w, req.WithContext(shared.WithActor(req.Context(), actor)))
	}
}
