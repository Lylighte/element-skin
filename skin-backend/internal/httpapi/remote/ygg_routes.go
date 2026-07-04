package remote

import (
	"net/http"
	"strings"

	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/permission"
	importsvc "element-skin/backend/internal/service/imports"
	"element-skin/backend/internal/util"
)

var (
	remoteProfileCreatePermission = permission.MustDefinitionByCode("profile.create.owned")
	remoteTextureCreatePermission = permission.MustDefinitionByCode("texture.create.owned")
)

func (h Handler) GetProfiles(w http.ResponseWriter, req *http.Request) {
	var body struct {
		APIURL   string `json:"api_url"`
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, err := (importsvc.RemoteYggService{DB: h.db, HTTPClient: h.httpClient}).PreviewProfiles(req.Context(), body.APIURL, body.Username, body.Password)
	if err != nil {
		util.Error(w, err)
		return
	}
	util.JSON(w, 200, map[string]any{"profiles": profiles})
}

func (h Handler) ImportProfiles(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, remoteProfileCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := shared.RequirePermission(req, remoteTextureCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]any
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, err := shared.ParseImportProfiles(body["profiles"])
	if err != nil {
		util.Error(w, err)
		return
	}
	res := (importsvc.RemoteYggService{DB: h.db, HTTPClient: h.httpClient}).ImportProfiles(req.Context(), shared.CurrentActor(req), shared.AsString(body["api_url"]), profiles)
	util.JSON(w, 200, res)
}

func (h Handler) ImportProfile(w http.ResponseWriter, req *http.Request) {
	if err := shared.RequirePermission(req, remoteProfileCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
	if err := shared.RequirePermission(req, remoteTextureCreatePermission); err != nil {
		util.Error(w, err)
		return
	}
	var body map[string]string
	if err := shared.DecodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := strings.TrimSpace(body["profile_id"])
	profileName := strings.TrimSpace(body["profile_name"])
	if profileID == "" || profileName == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "profile_id and profile_name are required"})
		return
	}
	res, err := (importsvc.RemoteYggService{DB: h.db, HTTPClient: h.httpClient}).ImportProfile(req.Context(), shared.CurrentActor(req), body["api_url"], profileID, profileName)
	if err != nil {
		util.Error(w, err)
		return
	}
	profile := res["profile"].(map[string]any)
	util.JSON(w, 200, map[string]any{"id": profile["id"], "name": profile["name"]})
}
