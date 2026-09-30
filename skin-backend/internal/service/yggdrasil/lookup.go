package yggdrasil

import (
	"context"
	"encoding/json"
	"errors"

	fallbacksvc "element-skin/backend/internal/service/fallback"
)

type LookupSource uint8

const (
	LookupAccount LookupSource = iota
	LookupServices
)

type LookupService struct {
	Ygg      Yggdrasil
	Fallback fallbacksvc.Fallback
}

func (s LookupService) Name(ctx context.Context, name string, source LookupSource) (map[string]any, bool, error) {
	profile, _, found, err := s.NameResponse(ctx, name, source)
	return profile, found, err
}

// NameResponse returns the normalized profile and, for a fallback hit, the
// original response so protocol-specific optional fields are preserved.
func (s LookupService) NameResponse(ctx context.Context, name string, source LookupSource) (map[string]any, *fallbacksvc.FallbackResponse, bool, error) {
	profile, status, err := s.Ygg.LookupName(ctx, name)
	if err != nil {
		return nil, nil, false, err
	}
	if status != 204 {
		return profile, nil, true, nil
	}

	var response *fallbacksvc.FallbackResponse
	switch source {
	case LookupAccount:
		response, err = s.Fallback.GetProfileByName(ctx, name)
	case LookupServices:
		response, err = s.Fallback.ServicesLookup(ctx, name)
	default:
		return nil, nil, false, errors.New("invalid profile lookup source")
	}
	if err != nil || response == nil {
		return nil, nil, false, err
	}
	var remote struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(response.Body, &remote); err != nil {
		return nil, nil, false, err
	}
	if remote.ID == "" || remote.Name == "" {
		return nil, nil, false, errors.New("invalid fallback profile lookup response")
	}
	return map[string]any{"id": remote.ID, "name": remote.Name}, response, true, nil
}

func (s LookupService) Names(ctx context.Context, names []string) ([]map[string]any, error) {
	return s.Fallback.LookupNames(ctx, names)
}

func (s LookupService) ServicesNames(ctx context.Context, names []string) ([]map[string]any, error) {
	return s.Fallback.LookupServicesNames(ctx, names)
}
