package yggdrasil_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/service/yggdrasil"
	"element-skin/backend/internal/testutil"
)

func TestYggdrasilJoinAndHasJoined(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ygg-session@test.com", "Password123", "YggSession", false)
	profile := testutil.CreateProfile(t, db, user.ID, "ygg_session_profile", "YggSessionRole")
	ygg := yggdrasil.Yggdrasil{DB: db, Cfg: testutil.TestConfig()}

	auth, err := ygg.Authenticate(ctx, user.Email, "Password123", "client_token", false)
	if err != nil {
		t.Fatal(err)
	}
	access := auth["accessToken"].(string)
	if err := ygg.Join(ctx, access, profile.ID, "server_1", "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	joined, status, err := ygg.HasJoined(ctx, profile.Name, "server_1")
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || joined["id"] != profile.ID || joined["name"] != profile.Name {
		t.Fatalf("HasJoined mismatch: status=%d body=%#v", status, joined)
	}
	if miss, status, err := ygg.HasJoined(ctx, "WrongName", "server_1"); err != nil || status != 204 || miss != nil {
		t.Fatalf("wrong name should miss: status=%d body=%#v err=%v", status, miss, err)
	}
}
