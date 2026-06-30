package minecraft_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/httpapi/minecraft"
	"element-skin/backend/internal/httpapi/shared"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	yggsvc "element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestMinecraftRoutesReturnExactProfileAndTextureResponses(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	user := testutil.CreateUser(t, db, "minecraft-route@test.com", "pw", "MinecraftRoute", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_route_profile", "MinecraftRoute")
	skin := "abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd"
	if err := db.Profiles.UpdateSkin(t.Context(), profile.ID, &skin); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/minecraft/profiles/by-name/"+profile.Name, nil)
	req.SetPathValue("name", profile.Name)
	rec := httptest.NewRecorder()
	h.ProfileByName(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile by name status=%d body=%q", rec.Code, rec.Body.String())
	}
	var byName map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &byName); err != nil {
		t.Fatal(err)
	}
	if byName["id"] != profile.ID || byName["name"] != profile.Name || byName["owner_user_id"] != user.ID || byName["public"] != true {
		t.Fatalf("profile by name body mismatch: %#v", byName)
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/minecraft/profiles/"+profile.ID+"/textures-property", nil)
	req.SetPathValue("profile_id", profile.ID)
	rec = httptest.NewRecorder()
	h.TexturesProperty(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("textures property status=%d body=%q", rec.Code, rec.Body.String())
	}
	var textureBody map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &textureBody); err != nil {
		t.Fatal(err)
	}
	property := textureBody["textures_property"].(map[string]any)
	if textureBody["profile_id"] != profile.ID || property["name"] != "textures" || property["value"] == "" || property["signature"] == "" {
		t.Fatalf("textures property body mismatch: %#v", textureBody)
	}
}

func TestMinecraftRoutesValidateBulkBodyAndMissingProfilesExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})

	req := httptest.NewRequest(http.MethodPost, "/v1/minecraft/profiles/by-names", strings.NewReader(`{"names":["Missing"]}`))
	rec := httptest.NewRecorder()
	h.ProfilesByNames(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf("missing bulk profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/minecraft/profiles/by-names", strings.NewReader(`[`))
	rec = httptest.NewRecorder()
	h.ProfilesByNames(rec, req)
	if rec.Code != http.StatusBadRequest || rec.Body.String() != "{\"detail\":\"invalid json\"}\n" {
		t.Fatalf("invalid bulk json response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/minecraft/profiles/missing-profile", nil)
	req.SetPathValue("profile_id", "missing-profile")
	rec = httptest.NewRecorder()
	h.ProfileByID(rec, req)
	if rec.Code != http.StatusNotFound || rec.Body.String() != "{\"detail\":\"minecraft profile not found\"}\n" {
		t.Fatalf("missing profile response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestMinecraftHasJoinedRouteUsesCurrentActorExactly(t *testing.T) {
	db, _, redis := testutil.NewTestAppWithRedisTB(t)
	cfg := testutil.TestConfig()
	cfg.SiteURL = "https://skin.example"
	h := minecraft.New(db, nil, yggsvc.Yggdrasil{DB: db, Cfg: cfg, Redis: redis})
	user := testutil.CreateUser(t, db, "minecraft-route-hasjoined@test.com", "pw", "MinecraftRouteHasJoined", false)
	profile := testutil.CreateProfile(t, db, user.ID, "minecraft_route_hasjoined", "MinecraftJoined")
	profileID := profile.ID
	if err := redis.SetYggToken(t.Context(), model.Token{AccessToken: "route-access", ClientToken: "client", UserID: user.ID, ProfileID: &profileID, CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := redis.SetYggSession(t.Context(), model.Session{ServerID: "route-server", AccessToken: "route-access", CreatedAt: database.NowMS()}, time.Minute); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/minecraft/session/has-joined", strings.NewReader(`{"username":"MinecraftJoined","server_id":"route-server"}`))
	req = req.WithContext(shared.WithActor(req.Context(), clientActorWith("minecraft_session.hasjoined.server")))
	rec := httptest.NewRecorder()
	h.HasJoined(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("has joined status=%d body=%q", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	profileBody := body["profile"].(map[string]any)
	if body["joined"] != true || profileBody["id"] != profile.ID || profileBody["name"] != profile.Name {
		t.Fatalf("has joined body mismatch: %#v", body)
	}
}

func clientActorWith(codes ...string) permission.Actor {
	bits := permission.NewBitSet(len(permission.Definitions))
	for _, code := range codes {
		bits.Set(permission.MustDefinitionByCode(code).BitIndex)
	}
	return permission.Actor{
		SubjectID:   "client:test-client",
		SessionKind: permission.SessionKindClient,
		Entrypoint:  permission.EntrypointAPI,
		Permissions: bits,
	}
}
