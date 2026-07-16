package imports

import "testing"

func TestMicrosoftProfileAssetsAcceptsTypedAndDecodedShapesExactly(t *testing.T) {
	assets := microsoftProfileAssets(map[string]any{
		"skins": []map[string]string{
			{"url": "http://typed-skin", "variant": "slim"},
		},
		"capes": []any{
			map[string]any{"url": "http://decoded-cape"},
			"ignored",
		},
	})
	if len(assets) != 2 {
		t.Fatalf("assets=%#v, want exactly two", assets)
	}
	if assets[0] != (TextureAsset{URL: "http://typed-skin", Kind: "skin", Variant: "slim"}) {
		t.Fatalf("skin asset=%#v, want exact typed asset", assets[0])
	}
	if assets[1] != (TextureAsset{URL: "http://decoded-cape", Kind: "cape"}) {
		t.Fatalf("cape asset=%#v, want exact decoded asset", assets[1])
	}
}

func TestVerifiedMicrosoftProfileKeepsOnlyVerifiedFieldsExactly(t *testing.T) {
	verified := verifiedMicrosoftProfile(map[string]any{
		"has_game": false,
		"profile": map[string]any{
			"id":    "profile-id",
			"name":  "Player",
			"skins": []any{"skin"},
		},
	})
	if verified["id"] != "profile-id" || verified["name"] != "Player" {
		t.Fatalf("verified identity=%#v, want exact profile identity", verified)
	}
	if _, ok := verified["has_game"]; ok {
		t.Fatalf("verified profile contains flow metadata: %#v", verified)
	}
	skins, skinsOK := verified["skins"].([]any)
	capes, capesOK := verified["capes"].([]any)
	if !skinsOK || len(skins) != 1 || skins[0] != "skin" || !capesOK || len(capes) != 0 {
		t.Fatalf("verified texture fields=%#v, want exact skin and empty capes", verified)
	}
}
