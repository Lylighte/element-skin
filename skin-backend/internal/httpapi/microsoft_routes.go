package httpapi

import (
	"net/http"
	"strings"
	"time"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) microsoftAuthURL(w http.ResponseWriter, req *http.Request) {
	state, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	clientID, _ := r.db.GetSetting(req.Context(), "microsoft_client_id", "")
	redirectURI, _ := r.db.GetSetting(req.Context(), "microsoft_redirect_uri", strings.TrimRight(r.cfg.APIURL, "/")+"/microsoft/callback")
	MicrosoftImportStates.Put(state, map[string]any{"user_id": currentUserID(req), "kind": "oauth_state"}, 10*time.Minute)
	util.JSON(w, 200, map[string]any{
		"auth_url": service.MicrosoftAuthorizationURL(clientID, redirectURI, state),
		"state":    state,
	})
}

func (r *Router) microsoftCallback(w http.ResponseWriter, req *http.Request) {
	siteURL := strings.TrimRight(r.cfg.SiteURL, "/")
	if siteURL == "" {
		siteURL = "http://localhost:5173"
	}
	if errText := req.URL.Query().Get("error"); errText != "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Authorization failed: " + errText})
		return
	}
	code := req.URL.Query().Get("code")
	state := req.URL.Query().Get("state")
	if code == "" || state == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Missing code or state parameter"})
		return
	}
	raw := MicrosoftImportStates.Pop(state)
	session, ok := raw.(map[string]any)
	if !ok || session["kind"] != "oauth_state" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid or expired state parameter"})
		return
	}
	clientID, _ := r.db.GetSetting(req.Context(), "microsoft_client_id", "")
	clientSecret, _ := r.db.GetSetting(req.Context(), "microsoft_client_secret", "")
	redirectURI, _ := r.db.GetSetting(req.Context(), "microsoft_redirect_uri", strings.TrimRight(r.cfg.APIURL, "/")+"/microsoft/callback")
	if clientID == "" || clientSecret == "" || redirectURI == "" {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	result, err := (service.MicrosoftAuthFlow{Client: service.MicrosoftHTTPClient{
		ClientID: clientID, ClientSecret: clientSecret, RedirectURI: redirectURI,
	}}).Complete(req.Context(), code)
	if err != nil || result["profile"] == nil {
		http.Redirect(w, req, siteURL+"/dashboard/roles?error=auth_failed", http.StatusFound)
		return
	}
	token, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	MicrosoftImportStates.Put(token, map[string]any{"user_id": session["user_id"], "kind": "profile", "profile": result}, 5*time.Minute)
	http.Redirect(w, req, siteURL+"/dashboard/roles?ms_token="+token, http.StatusFound)
}

func (r *Router) microsoftGetProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	raw := MicrosoftImportStates.Pop(body["ms_token"])
	session, ok := raw.(map[string]any)
	if !ok || session["kind"] != "profile" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "Invalid or expired token"})
		return
	}
	if session["user_id"] != currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "Unauthorized"})
		return
	}
	flowProfile, _ := session["profile"].(map[string]any)
	mcProfile, _ := flowProfile["profile"].(map[string]any)
	verified := map[string]any{
		"id":    mcProfile["id"],
		"name":  mcProfile["name"],
		"skins": valueOrAny(mcProfile["skins"], []any{}),
		"capes": valueOrAny(mcProfile["capes"], []any{}),
	}
	importToken, err := randomToken(64)
	if err != nil {
		util.Error(w, err)
		return
	}
	MicrosoftImportStates.Put(importToken, map[string]any{
		"user_id": currentUserID(req),
		"kind":    "import",
		"profile": verified,
	}, 5*time.Minute)
	util.JSON(w, 200, map[string]any{
		"profile":      verified,
		"has_game":     valueOrAny(flowProfile["has_game"], false),
		"import_token": importToken,
	})
}

func (r *Router) microsoftImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	token := body["ms_token"]
	raw := MicrosoftImportStates.Pop(token)
	if raw == nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	session, ok := raw.(map[string]any)
	if !ok {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	if session["kind"] != "import" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	if session["user_id"] != currentUserID(req) {
		util.Error(w, util.HTTPError{Status: 403, Detail: "not allowed"})
		return
	}
	profile, ok := session["profile"].(map[string]any)
	if !ok {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid import token"})
		return
	}
	profileID, _ := profile["id"].(string)
	profileName, _ := profile["name"].(string)
	var assets []service.TextureAsset
	if skins, ok := profile["skins"].([]map[string]string); ok {
		for _, skin := range skins {
			assets = append(assets, service.TextureAsset{URL: skin["url"], Kind: "skin", Variant: skin["variant"]})
		}
	} else if skins, ok := profile["skins"].([]any); ok {
		for _, rawSkin := range skins {
			if skin, ok := rawSkin.(map[string]any); ok {
				u, _ := skin["url"].(string)
				variant, _ := skin["variant"].(string)
				assets = append(assets, service.TextureAsset{URL: u, Kind: "skin", Variant: variant})
			}
		}
	}
	if capes, ok := profile["capes"].([]map[string]string); ok {
		for _, cape := range capes {
			assets = append(assets, service.TextureAsset{URL: cape["url"], Kind: "cape"})
		}
	} else if capes, ok := profile["capes"].([]any); ok {
		for _, rawCape := range capes {
			if cape, ok := rawCape.(map[string]any); ok {
				u, _ := cape["url"].(string)
				assets = append(assets, service.TextureAsset{URL: u, Kind: "cape"})
			}
		}
	}
	res, err := (service.ImportService{DB: r.db}).ImportProfile(req.Context(), currentUserID(req), profileID, profileName, assets)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, res)
}

func randomToken(length int) (string, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return "", err
	}
	token := id
	for len(token) < length {
		next, err := util.GenerateUUIDNoDash()
		if err != nil {
			return "", err
		}
		token += next
	}
	return token[:length], nil
}
