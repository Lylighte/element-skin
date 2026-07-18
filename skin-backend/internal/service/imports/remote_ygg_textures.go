package imports

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

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
