package oauth

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/util"
)

func (h Handler) ClientPermissions(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ClientPermissions(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) SetClientPermission(w http.ResponseWriter, req *http.Request) {
	var body permissionOverrideBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.oauth.SetClientPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), req.PathValue("permission_code"), body.Effect); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) ClearClientPermission(w http.ResponseWriter, req *http.Request) {
	if err := h.oauth.ClearClientPermissionOverride(req.Context(), shared.CurrentActor(req), req.PathValue("client_id"), req.PathValue("permission_code")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) ListGrants(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.ListGrants(req.Context(), shared.CurrentActor(req), util.ClampLimit(req.URL.Query().Get("limit")))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"items": res})
}

func (h Handler) RevokeGrant(w http.ResponseWriter, req *http.Request) {
	if err := h.oauth.RevokeGrant(req.Context(), shared.CurrentActor(req), req.PathValue("grant_id")); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) AuthorizeInfo(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.AuthorizationDetails(req.Context(), shared.CurrentActor(req), authorizationRequest(req))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) ApproveAuthorization(w http.ResponseWriter, req *http.Request) {
	var body authorizeBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	res, err := h.oauth.ApproveAuthorization(req.Context(), shared.CurrentActor(req), body.request())
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) DeviceCode(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Detail: "invalid form"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	res, err := h.oauth.StartDeviceAuthorization(req.Context(), oauthsvc.DeviceAuthorizationRequest{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scope:        req.Form.Get("scope"),
	})
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	base := strings.TrimRight(h.cfg.SiteURL, "/")
	if base == "" {
		base = h.baseURL()
	}
	out := map[string]any{
		"device_code":               res.DeviceCode,
		"user_code":                 res.UserCode,
		"verification_uri":          base + "/oauth/device",
		"verification_uri_complete": base + "/oauth/device?user_code=" + res.UserCode,
		"expires_in":                res.ExpiresIn,
		"interval":                  res.Interval,
		"scope":                     res.Scope,
		"permissions":               res.Permissions,
	}
	util.JSON(w, http.StatusOK, out)
}

func (h Handler) DeviceInfo(w http.ResponseWriter, req *http.Request) {
	res, err := h.oauth.DeviceAuthorizationDetails(req.Context(), shared.CurrentActor(req), req.URL.Query().Get("user_code"))
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) DeviceDecision(w http.ResponseWriter, req *http.Request) {
	var body deviceDecisionBody
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	if err := h.oauth.DecideDeviceAuthorization(req.Context(), shared.CurrentActor(req), oauthsvc.DeviceDecisionRequest{
		UserCode: body.UserCode,
		Approve:  body.Approve,
	}); err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (h Handler) Token(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Detail: "invalid form"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	res, err := h.oauth.IssueToken(req.Context(), oauthsvc.TokenRequest{
		GrantType:    req.Form.Get("grant_type"),
		Code:         req.Form.Get("code"),
		RedirectURI:  req.Form.Get("redirect_uri"),
		ClientID:     clientID,
		ClientSecret: clientSecret,
		CodeVerifier: req.Form.Get("code_verifier"),
		RefreshToken: req.Form.Get("refresh_token"),
		Scope:        req.Form.Get("scope"),
		DeviceCode:   req.Form.Get("device_code"),
	})
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

func (h Handler) Revoke(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Detail: "invalid form"})
		return
	}
	clientID, clientSecret := clientCredentials(req)
	if err := h.oauth.RevokeToken(req.Context(), clientID, clientSecret, req.Form.Get("token")); err != nil {
		writeProtocolError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (h Handler) Introspect(w http.ResponseWriter, req *http.Request) {
	if err := req.ParseForm(); err != nil {
		writeProtocolError(w, util.HTTPError{Status: 400, Detail: "invalid form"})
		return
	}
	res, err := h.oauth.Introspect(req.Context(), shared.CurrentActor(req), req.Form.Get("token"))
	if err != nil {
		writeProtocolError(w, err)
		return
	}
	util.JSON(w, http.StatusOK, res)
}

type permissionOverrideBody struct {
	Effect string `json:"effect"`
}

type appReviewBody struct {
	Status string `json:"status"`
	Reason string `json:"reason"`
}

func (b appBody) input() oauthsvc.ClientInput {
	return oauthsvc.ClientInput{
		Name:            b.Name,
		Description:     b.Description,
		RedirectURI:     b.RedirectURI,
		WebsiteURL:      b.WebsiteURL,
		ClientType:      b.ClientType,
		PermissionCodes: b.PermissionCodes,
	}
}

type authorizeBody struct {
	ResponseType        string `json:"response_type"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
}

type deviceDecisionBody struct {
	UserCode string `json:"user_code"`
	Approve  bool   `json:"approve"`
}

func (b authorizeBody) request() oauthsvc.AuthorizationRequest {
	return oauthsvc.AuthorizationRequest{
		ResponseType:        b.ResponseType,
		ClientID:            b.ClientID,
		RedirectURI:         b.RedirectURI,
		Scope:               b.Scope,
		State:               b.State,
		CodeChallenge:       b.CodeChallenge,
		CodeChallengeMethod: b.CodeChallengeMethod,
	}
}

func authorizationRequest(req *http.Request) oauthsvc.AuthorizationRequest {
	q := req.URL.Query()
	return oauthsvc.AuthorizationRequest{
		ResponseType:        q.Get("response_type"),
		ClientID:            q.Get("client_id"),
		RedirectURI:         q.Get("redirect_uri"),
		Scope:               q.Get("scope"),
		State:               q.Get("state"),
		CodeChallenge:       q.Get("code_challenge"),
		CodeChallengeMethod: q.Get("code_challenge_method"),
	}
}

func clientCredentials(req *http.Request) (string, string) {
	if id, secret, ok := req.BasicAuth(); ok {
		return id, secret
	}
	return strings.TrimSpace(req.Form.Get("client_id")), req.Form.Get("client_secret")
}
