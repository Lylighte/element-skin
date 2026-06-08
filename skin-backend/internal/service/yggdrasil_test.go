package service_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"element-skin/backend/internal/config"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/service"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilProfileJSONExactTexturePayload(t *testing.T) {
	skin := "skin_hash"
	cape := "cape_hash"
	ygg := service.Yggdrasil{Cfg: config.Config{SiteURL: "https://skin.example/root/"}}

	signed := ygg.ProfileJSON(model.Profile{
		ID: "profile_id", Name: "SlimPlayer", TextureModel: "slim", SkinHash: &skin, CapeHash: &cape,
	}, true)
	if signed["id"] != "profile_id" || signed["name"] != "SlimPlayer" {
		t.Fatalf("unexpected profile envelope: %#v", signed)
	}
	props := signed["properties"].([]map[string]any)
	textureProp := props[0]
	if textureProp["name"] != "textures" || textureProp["signature"] == "" {
		t.Fatalf("signed texture property missing name/signature: %#v", textureProp)
	}
	decoded, err := base64.StdEncoding.DecodeString(textureProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	textures := payload["textures"].(map[string]any)
	skinPayload := textures["SKIN"].(map[string]any)
	if skinPayload["url"] != "https://skin.example/root/static/textures/skin_hash.png" ||
		skinPayload["metadata"].(map[string]any)["model"] != "slim" ||
		textures["CAPE"].(map[string]any)["url"] != "https://skin.example/root/static/textures/cape_hash.png" {
		t.Fatalf("unexpected textures payload: %#v", textures)
	}
	if props[1]["name"] != "uploadableTextures" || props[1]["value"] != "skin,cape" {
		t.Fatalf("missing uploadableTextures property: %#v", props)
	}

	unsigned := ygg.ProfileJSON(model.Profile{ID: "p2", Name: "DefaultPlayer", TextureModel: "default", SkinHash: &skin}, false)
	unsignedProp := unsigned["properties"].([]map[string]any)[0]
	if _, ok := unsignedProp["signature"]; ok {
		t.Fatalf("unsigned profile should not include signature: %#v", unsignedProp)
	}
	decoded, err = base64.StdEncoding.DecodeString(unsignedProp["value"].(string))
	if err != nil {
		t.Fatal(err)
	}
	payload = map[string]any{}
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatal(err)
	}
	defaultSkin := payload["textures"].(map[string]any)["SKIN"].(map[string]any)
	if _, ok := defaultSkin["metadata"]; ok {
		t.Fatalf("default model should not include metadata: %#v", defaultSkin)
	}
}

func TestYggdrasilLookupNameReturnsExactStatus(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "lookup-service@test.com", "Password123", "LookupService", false)
	testutil.CreateProfile(t, db, user.ID, "lookup_profile_id", "LookupProfile")
	ygg := service.Yggdrasil{DB: db}

	hit, status, err := ygg.LookupName(ctx, "LookupProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || hit["id"] != "lookup_profile_id" || hit["name"] != "LookupProfile" {
		t.Fatalf("unexpected lookup hit status=%d body=%#v", status, hit)
	}
	miss, status, err := ygg.LookupName(ctx, "MissingProfile")
	if err != nil {
		t.Fatal(err)
	}
	if status != 204 || miss != nil {
		t.Fatalf("unexpected lookup miss status=%d body=%#v", status, miss)
	}
}
