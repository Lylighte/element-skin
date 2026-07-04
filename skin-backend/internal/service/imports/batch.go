package imports

import (
	"context"

	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s ImportService) ImportProfiles(ctx context.Context, actor permission.Actor, profiles []map[string]string, fetch func(context.Context, string) ([]TextureAsset, error)) map[string]any {
	var items []map[string]any
	var failed []map[string]any
	for _, p := range profiles {
		id := p["profile_id"]
		name := p["profile_name"]
		if id == "" || name == "" {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "profile_id and profile_name are required"})
			continue
		}
		assets, err := fetch(ctx, id)
		if err != nil {
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": "导入失败"})
			continue
		}
		res, err := s.ImportProfile(ctx, actor, id, name, assets)
		if err != nil {
			detail := "导入失败"
			if he, ok := err.(util.HTTPError); ok {
				detail = he.Detail
			}
			failed = append(failed, map[string]any{"profile_id": id, "profile_name": name, "detail": detail})
			continue
		}
		items = append(items, res["profile"].(map[string]any))
	}
	return map[string]any{
		"success_count": len(items),
		"failure_count": len(failed),
		"items":         items,
		"failed":        failed,
	}
}
