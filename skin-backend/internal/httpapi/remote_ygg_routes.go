package httpapi

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/service"
	"element-skin/backend/internal/util"
)

func (r *Router) remoteYggGetProfiles(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, _ := body["profiles"].([]any)
	if profiles == nil {
		profiles = []any{}
	}
	util.JSON(w, 200, map[string]any{"profiles": profiles})
}

func (r *Router) remoteYggImportProfiles(w http.ResponseWriter, req *http.Request) {
	var body map[string]any
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profiles, err := parseImportProfiles(body["profiles"])
	if err != nil {
		util.Error(w, err)
		return
	}
	importer := service.ImportService{DB: r.db}
	res := importer.ImportProfiles(req.Context(), currentUserID(req), profiles, func(ctx context.Context, id string) ([]service.TextureAsset, error) {
		return []service.TextureAsset{{URL: id + ":skin", Kind: "skin", Variant: "classic"}}, nil
	})
	util.JSON(w, 200, res)
}

func (r *Router) remoteYggImportProfile(w http.ResponseWriter, req *http.Request) {
	var body map[string]string
	if err := decodeJSON(req, &body); err != nil {
		util.Error(w, util.HTTPError{Status: 400, Detail: "invalid json"})
		return
	}
	profileID := strings.TrimSpace(body["profile_id"])
	profileName := strings.TrimSpace(body["profile_name"])
	if profileID == "" || profileName == "" {
		util.Error(w, util.HTTPError{Status: 400, Detail: "profile_id and profile_name are required"})
		return
	}
	importer := service.ImportService{DB: r.db}
	res, err := importer.ImportProfile(req.Context(), currentUserID(req), profileID, profileName, []service.TextureAsset{{URL: profileID + ":skin", Kind: "skin", Variant: "classic"}})
	if err != nil {
		util.Error(w, err)
		return
	}
	profile := res["profile"].(map[string]any)
	util.JSON(w, 200, map[string]any{"id": profile["id"], "name": profile["name"]})
}
