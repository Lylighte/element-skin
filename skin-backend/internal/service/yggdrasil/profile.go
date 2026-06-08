package yggdrasil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func (y Yggdrasil) Profile(ctx context.Context, id string, unsigned bool) (map[string]any, int, error) {
	p, err := y.DB.Profiles.GetByID(ctx, util.StripUUIDDashes(id))
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	body, err := y.ProfileJSON(*p, !unsigned)
	if err != nil {
		return nil, 0, err
	}
	return body, 200, nil
}

func (y Yggdrasil) ProfileJSON(p model.Profile, sign bool) (map[string]any, error) {
	base := strings.TrimRight(y.publicTextureBaseURL(), "/") + "/static/textures/"
	textures := map[string]any{}
	if p.SkinHash != nil {
		skin := map[string]any{"url": base + *p.SkinHash + ".png"}
		if p.TextureModel == "slim" {
			skin["metadata"] = map[string]any{"model": "slim"}
		}
		textures["SKIN"] = skin
	}
	if p.CapeHash != nil {
		textures["CAPE"] = map[string]any{"url": base + *p.CapeHash + ".png"}
	}
	payload := map[string]any{"timestamp": time.Now().UnixMilli(), "profileId": p.ID, "profileName": p.Name, "textures": textures}
	b, _ := json.Marshal(payload)
	prop := map[string]any{"name": "textures", "value": base64.StdEncoding.EncodeToString(b)}
	if sign {
		signer, err := y.signer()
		if err != nil {
			return nil, err
		}
		signature, err := signer.SignPropertyValue(prop["value"].(string))
		if err != nil {
			return nil, err
		}
		prop["signature"] = signature
	}
	return map[string]any{"id": p.ID, "name": p.Name, "properties": []map[string]any{prop, {"name": "uploadableTextures", "value": "skin,cape"}}}, nil
}

func (y Yggdrasil) LookupName(ctx context.Context, name string) (map[string]any, int, error) {
	p, err := y.DB.Profiles.GetByName(ctx, name)
	if err != nil {
		return nil, 0, err
	}
	if p == nil {
		return nil, 204, nil
	}
	return map[string]any{"id": p.ID, "name": p.Name}, 200, nil
}
