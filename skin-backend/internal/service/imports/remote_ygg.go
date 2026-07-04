package imports

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type RemoteYggService struct {
	DB          *database.DB
	TexturesDir string
	HTTPClient  *http.Client
}

type RemoteYggProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (s RemoteYggService) PreviewProfiles(ctx context.Context, apiURL, username, password string) ([]RemoteYggProfile, error) {
	apiURL = strings.TrimSpace(apiURL)
	username = strings.TrimSpace(username)
	if apiURL == "" || username == "" || password == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "api_url, username and password are required"}
	}
	var out struct {
		AvailableProfiles []RemoteYggProfile `json:"availableProfiles"`
	}
	if err := s.doJSON(ctx, http.MethodPost, remoteYggURL(apiURL, "authserver/authenticate"), map[string]any{
		"username": username,
		"password": password,
		"agent": map[string]any{
			"name":    "Minecraft",
			"version": 1,
		},
		"requestUser": true,
	}, &out); err != nil {
		return nil, err
	}
	profiles := make([]RemoteYggProfile, 0, len(out.AvailableProfiles))
	for _, profile := range out.AvailableProfiles {
		profile.ID = strings.TrimSpace(profile.ID)
		profile.Name = strings.TrimSpace(profile.Name)
		if profile.ID == "" || profile.Name == "" {
			continue
		}
		profiles = append(profiles, profile)
	}
	return profiles, nil
}

func (s RemoteYggService) ImportProfile(ctx context.Context, actor permission.Actor, apiURL, profileID, profileName string) (map[string]any, error) {
	apiURL = strings.TrimSpace(apiURL)
	if apiURL == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "api_url is required"}
	}
	assets, err := s.FetchTextureAssets(ctx, apiURL, profileID)
	if err != nil {
		return nil, err
	}
	return (ImportService{DB: s.DB, TexturesDir: s.TexturesDir, HTTPClient: s.HTTPClient}).ImportProfile(ctx, actor, profileID, profileName, assets)
}

func (s RemoteYggService) ImportProfiles(ctx context.Context, actor permission.Actor, apiURL string, profiles []map[string]string) map[string]any {
	importer := ImportService{DB: s.DB, TexturesDir: s.TexturesDir, HTTPClient: s.HTTPClient}
	return importer.ImportProfiles(ctx, actor, profiles, func(ctx context.Context, id string) ([]TextureAsset, error) {
		apiURL = strings.TrimSpace(apiURL)
		if apiURL == "" {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "api_url is required"}
		}
		return s.FetchTextureAssets(ctx, apiURL, id)
	})
}

func (s RemoteYggService) FetchTextureAssets(ctx context.Context, apiURL, profileID string) ([]TextureAsset, error) {
	profileID = strings.TrimSpace(strings.ReplaceAll(profileID, "-", ""))
	if apiURL == "" || profileID == "" {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "api_url and profile_id are required"}
	}
	var out remoteYggProfileResponse
	if err := s.doJSON(ctx, http.MethodGet, remoteYggURL(apiURL, "sessionserver/session/minecraft/profile", profileID), nil, &out); err != nil {
		return nil, err
	}
	return out.textureAssets(), nil
}
