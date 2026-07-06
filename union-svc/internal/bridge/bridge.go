package bridge

import (
	"context"
	"fmt"
	"net/http"

	"element-skin/union-svc/internal/oauth"
	"element-skin/union-svc/internal/union"
)

// ListProfileItem is a single profile returned by the list endpoint.
type ListProfileItem struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

// ImportProfileRequest is the payload for importing a profile.
type ImportProfileRequest struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// Bridge orchestrates Union profile discovery and Element-Skin profile import.
type Bridge struct {
	union         *union.Client
	oauth         *oauth.Manager
	serviceTokens *oauth.ServiceTokenManager
	elementskin   *ElementSkinClient
}

// New creates a Bridge from runtime dependencies.
func New(elementskinBaseURL string, unionClient *union.Client, manager *oauth.Manager, serviceTokens *oauth.ServiceTokenManager, httpClient *http.Client) *Bridge {
	return &Bridge{
		union:         unionClient,
		oauth:         manager,
		serviceTokens: serviceTokens,
		elementskin:   NewElementSkinClient(elementskinBaseURL, httpClient),
	}
}

// ListProfiles queries the Union Hub for profiles matching username.
func (b *Bridge) ListProfiles(ctx context.Context, username string) ([]ListProfileItem, error) {
	profiles, err := b.union.GetProfiles(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("list union profiles: %w", err)
	}

	items := make([]ListProfileItem, len(profiles))
	for i, p := range profiles {
		items[i] = ListProfileItem{
			UUID: p.UUID,
			Name: p.Name,
		}
	}
	return items, nil
}

// ImportProfile creates a profile on Element-Skin using the stored OAuth token.
func (b *Bridge) ImportProfile(ctx context.Context, req ImportProfileRequest) (*CreatedProfile, error) {
	token, err := b.oauth.AccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}
	return b.elementskin.CreateProfile(ctx, token, req.Name, req.Model)
}

// ListAllProfilesForSync returns every local Element-Skin profile for the
// Union sync handler, using the service account token.
func (b *Bridge) ListAllProfilesForSync(ctx context.Context) ([]AdminProfile, error) {
	token, err := b.serviceTokens.ServiceAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get service access token: %w", err)
	}
	return b.elementskin.ListAllProfiles(ctx, token, "")
}

// GetUserEmailByProfileName resolves a profile name to the profile owner's
// email using a single admin profiles list call.
func (b *Bridge) GetUserEmailByProfileName(ctx context.Context, name string) (string, error) {
	token, err := b.serviceTokens.ServiceAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("get service access token: %w", err)
	}
	profiles, err := b.elementskin.ListAllProfiles(ctx, token, name)
	if err != nil {
		return "", fmt.Errorf("list admin profiles: %w", err)
	}
	for _, p := range profiles {
		if p.Name == name {
			return p.OwnerEmail, nil
		}
	}
	return "", nil
}
