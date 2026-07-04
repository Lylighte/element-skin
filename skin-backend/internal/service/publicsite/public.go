package publicsite

import (
	"context"
	"errors"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
	settingssvc "element-skin/backend/internal/service/settings"
)

type Service struct {
	DB       *database.DB
	Redis    redisstore.Store
	Settings settingssvc.Settings
	SiteURL  string
	APIURL   string
	CacheTTL time.Duration
}

func (s Service) PublicSettings(ctx context.Context) (map[string]any, error) {
	if cached, err := s.Redis.GetPublicSettings(ctx); err == nil {
		if currentPublicSettingsCache(cached) {
			return cached, nil
		}
		if err := s.Redis.InvalidatePublicSettings(ctx); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	res, err := s.Settings.Public(ctx, s.SiteURL, s.APIURL)
	if err != nil {
		return nil, err
	}
	if err := s.Redis.SetPublicSettings(ctx, res, s.CacheTTL); err != nil {
		return nil, err
	}
	return res, nil
}

func currentPublicSettingsCache(cached map[string]any) bool {
	_, ok := cached["require_invite"]
	return ok
}

func (s Service) HomepageMedia(ctx context.Context) ([]model.HomepageMedia, error) {
	if cached, err := s.Redis.GetPublicHomepageMedia(ctx); err == nil {
		return cached, nil
	} else if !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	items, err := s.DB.HomepageMedia.List(ctx, true)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []model.HomepageMedia{}
	}
	if err := s.Redis.SetPublicHomepageMedia(ctx, items, s.CacheTTL); err != nil {
		return nil, err
	}
	return items, nil
}

func (s Service) FallbackStatus(ctx context.Context, now time.Time) (map[string]any, error) {
	endpoints, err := s.DB.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	since := now.Add(-redisstore.ProbeHistoryRetention)
	history, err := s.Redis.GetProbeHistory(ctx, since)
	if err != nil && !errors.Is(err, redisstore.ErrCacheMiss) {
		return nil, err
	}
	return buildFallbackStatus(endpoints, history, now), nil
}

type fallbackStatusEntry struct {
	ID          int                  `json:"id"`
	Priority    int                  `json:"priority"`
	Note        string               `json:"note"`
	SessionURL  string               `json:"session_url"`
	AccountURL  string               `json:"account_url"`
	ServicesURL string               `json:"services_url"`
	Latest      *fallbackStatusTick  `json:"latest"`
	History     []fallbackStatusTick `json:"history"`
}

type fallbackStatusTick struct {
	CheckedAt int64  `json:"checked_at"`
	Session   string `json:"session"`
	Account   string `json:"account"`
	Services  string `json:"services"`
}

func buildFallbackStatus(endpoints []map[string]any, history []redisstore.ProbeSample, now time.Time) map[string]any {
	byID := make(map[int][]redisstore.ProbeSample, len(endpoints))
	for _, sample := range history {
		byID[sample.EndpointID] = append(byID[sample.EndpointID], sample)
	}
	out := make([]fallbackStatusEntry, 0, len(endpoints))
	for _, ep := range endpoints {
		id, _ := ep["id"].(int)
		priority, _ := ep["priority"].(int)
		note, _ := ep["note"].(string)
		sessionURL, _ := ep["session_url"].(string)
		accountURL, _ := ep["account_url"].(string)
		servicesURL, _ := ep["services_url"].(string)
		samples := byID[id]
		ticks := make([]fallbackStatusTick, 0, len(samples))
		for _, sample := range samples {
			ticks = append(ticks, fallbackStatusTick{
				CheckedAt: sample.CheckedAt,
				Session:   sample.Session,
				Account:   sample.Account,
				Services:  sample.Services,
			})
		}
		var latest *fallbackStatusTick
		if len(ticks) > 0 {
			latest = &ticks[len(ticks)-1]
		}
		out = append(out, fallbackStatusEntry{
			ID:          id,
			Priority:    priority,
			Note:        note,
			SessionURL:  sessionURL,
			AccountURL:  accountURL,
			ServicesURL: servicesURL,
			Latest:      latest,
			History:     ticks,
		})
	}
	return map[string]any{
		"endpoints":    out,
		"retention_ms": redisstore.ProbeHistoryRetention.Milliseconds(),
		"generated_at": now.UnixMilli(),
	}
}
