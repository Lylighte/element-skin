package httpapi

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/util"
)

func (r *Router) setSessionCookies(w http.ResponseWriter, access, refresh string) {
	secure := strings.HasPrefix(r.cfg.SiteURL, "https://")
	http.SetCookie(w, &http.Cookie{Name: "access_token", Value: access, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: r.cfg.AccessMinutes * 60})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: refresh, Path: "/", HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: r.cfg.JWTExpireDays * 24 * 3600})
}

func (r *Router) siteLogin(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := r.site.Login(req.Context(), body["email"], body["password"])
	if err != nil {
		util.Error(w, err)
		return
	}
	r.setSessionCookies(w, res["access_token"].(string), res["refresh_token"].(string))
	util.JSON(w, 200, map[string]any{"user_id": res["user_id"], "is_admin": res["is_admin"]})
}

func (r *Router) siteLogout(w http.ResponseWriter, req *http.Request) {
	if c, err := req.Cookie("refresh_token"); err == nil {
		_ = r.site.RevokeRefresh(req.Context(), c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "access_token", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "refresh_token", Path: "/", MaxAge: -1})
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) register(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	id, err := r.site.Register(req.Context(), body["email"], body["password"], body["username"], body["invite"], body["code"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"id": id})
}

func (r *Router) sendVerificationCode(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	email := body["email"]
	if email == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "email required"})
		return
	}
	res, err := r.site.SendVerificationCode(req.Context(), email, body["type"])
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func (r *Router) resetPassword(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if body["email"] == "" || body["password"] == "" || body["code"] == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "email, password and code required"})
		return
	}
	if err := r.site.ResetPassword(req.Context(), body["email"], body["password"], body["code"]); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"ok": true})
}

func (r *Router) refreshToken(w http.ResponseWriter, req *http.Request) {
	c, err := req.Cookie("refresh_token")
	if err != nil || c.Value == "" {
		util.Error(w, util.HTTPError{Status: 401, Detail: "not authenticated"})
		return
	}
	res, err := r.site.RotateRefresh(req.Context(), c.Value)
	if err != nil {
		util.Error(w, err)
		return
	}
	r.setSessionCookies(w, res["access_token"].(string), res["refresh_token"].(string))
	util.JSON(w, 200, map[string]any{"is_admin": res["is_admin"]})
}
