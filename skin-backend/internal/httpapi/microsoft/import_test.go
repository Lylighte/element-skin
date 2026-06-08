package microsoft

import "testing"

func TestMicrosoftProfileAssetsAcceptsTypedAndDecodedShapes(t *testing.T) {
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
		t.Fatalf("expected two assets, got %#v", assets)
	}
	if assets[0].URL != "http://typed-skin" || assets[0].Kind != "skin" || assets[0].Variant != "slim" {
		t.Fatalf("unexpected skin asset: %#v", assets[0])
	}
	if assets[1].URL != "http://decoded-cape" || assets[1].Kind != "cape" {
		t.Fatalf("unexpected cape asset: %#v", assets[1])
	}
}
