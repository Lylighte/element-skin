package imports

func verifiedMicrosoftProfile(flowProfile map[string]any) map[string]any {
	mcProfile, _ := flowProfile["profile"].(map[string]any)
	return map[string]any{
		"id":    mcProfile["id"],
		"name":  mcProfile["name"],
		"skins": valueOrAny(mcProfile["skins"], []any{}),
		"capes": valueOrAny(mcProfile["capes"], []any{}),
	}
}

func microsoftProfileAssets(profile map[string]any) []TextureAsset {
	var assets []TextureAsset
	assets = appendMicrosoftTextureAssets(assets, profile["skins"], "skin")
	assets = appendMicrosoftTextureAssets(assets, profile["capes"], "cape")
	return assets
}

func appendMicrosoftTextureAssets(assets []TextureAsset, raw any, kind string) []TextureAsset {
	if typed, ok := raw.([]map[string]string); ok {
		for _, item := range typed {
			assets = append(assets, TextureAsset{URL: item["url"], Kind: kind, Variant: item["variant"]})
		}
		return assets
	}
	items, ok := raw.([]any)
	if !ok {
		return assets
	}
	for _, item := range items {
		asset, ok := microsoftTextureAssetFromMap(item, kind)
		if ok {
			assets = append(assets, asset)
		}
	}
	return assets
}

func microsoftTextureAssetFromMap(raw any, kind string) (TextureAsset, bool) {
	item, ok := raw.(map[string]any)
	if !ok {
		return TextureAsset{}, false
	}
	u, _ := item["url"].(string)
	variant, _ := item["variant"].(string)
	return TextureAsset{URL: u, Kind: kind, Variant: variant}, true
}
