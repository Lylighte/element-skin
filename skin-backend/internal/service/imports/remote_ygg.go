package imports

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

type RemoteYggService struct {
	DB         *database.DB
	HTTPClient *http.Client
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
	return (ImportService{DB: s.DB}).ImportProfile(ctx, actor, profileID, profileName, assets)
}

func (s RemoteYggService) ImportProfiles(ctx context.Context, actor permission.Actor, apiURL string, profiles []map[string]string) map[string]any {
	importer := ImportService{DB: s.DB}
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

func (s RemoteYggService) doJSON(ctx context.Context, method, rawURL string, payload any, out any) error {
	if err := util.ValidateOutboundURL(rawURL); err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid remote api url"}
	}
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid remote api url"}
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := remoteYggHTTPClient(s.HTTPClient).Do(req)
	if err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "无法获取远端资料，请检查账号或稍后重试"}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: remoteYggErrorDetail(resp)}
	}
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 1<<20))
	if err := decoder.Decode(out); err != nil {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "远端资料格式无效"}
	}
	return nil
}

func remoteYggHTTPClient(base *http.Client) *http.Client {
	if base == nil {
		base = &http.Client{Timeout: 10 * time.Second}
	}
	client := *base
	if client.Timeout == 0 {
		client.Timeout = 10 * time.Second
	}
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &client
}

func remoteYggURL(apiURL string, parts ...string) string {
	base := strings.TrimRight(strings.TrimSpace(apiURL), "/")
	all := append([]string{base}, parts...)
	joined, err := url.JoinPath(all[0], all[1:]...)
	if err != nil {
		return base
	}
	return joined
}

func remoteYggErrorDetail(resp *http.Response) string {
	var body struct {
		ErrorMessage string `json:"errorMessage"`
		Error        string `json:"error"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 8192)).Decode(&body)
	detail := strings.TrimSpace(body.ErrorMessage)
	if detail == "" {
		detail = strings.TrimSpace(body.Error)
	}
	if detail == "" {
		detail = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return "远端认证失败: " + detail
}

type remoteYggProfileResponse struct {
	ID         string                     `json:"id"`
	Name       string                     `json:"name"`
	Properties []remoteYggProfileProperty `json:"properties"`
}

type remoteYggProfileProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (p remoteYggProfileResponse) textureAssets() []TextureAsset {
	for _, property := range p.Properties {
		if property.Name != "textures" || strings.TrimSpace(property.Value) == "" {
			continue
		}
		payload, err := decodeRemoteYggTextures(property.Value)
		if err != nil {
			return nil
		}
		var assets []TextureAsset
		if skin := payload.Textures.Skin; strings.TrimSpace(skin.URL) != "" {
			variant := strings.TrimSpace(skin.Metadata.Model)
			if variant == "" {
				variant = "classic"
			}
			assets = append(assets, TextureAsset{URL: skin.URL, Kind: "skin", Variant: variant})
		}
		if cape := payload.Textures.Cape; strings.TrimSpace(cape.URL) != "" {
			assets = append(assets, TextureAsset{URL: cape.URL, Kind: "cape"})
		}
		return assets
	}
	return nil
}

func decodeRemoteYggTextures(raw string) (remoteYggTexturesPayload, error) {
	var payload remoteYggTexturesPayload
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil {
		return payload, err
	}
	err = json.Unmarshal(data, &payload)
	return payload, err
}

type remoteYggTexturesPayload struct {
	Textures struct {
		Skin remoteYggTexture `json:"SKIN"`
		Cape remoteYggTexture `json:"CAPE"`
	} `json:"textures"`
}

type remoteYggTexture struct {
	URL      string `json:"url"`
	Metadata struct {
		Model string `json:"model"`
	} `json:"metadata"`
}
