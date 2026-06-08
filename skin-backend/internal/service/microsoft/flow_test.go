package microsoft_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

type fakeMicrosoftClient struct {
	calls []string
}

func (f *fakeMicrosoftClient) ExchangeCodeForToken(context.Context, string) (map[string]any, error) {
	f.calls = append(f.calls, "exchange")
	return map[string]any{"access_token": "ms_access_token"}, nil
}

func (f *fakeMicrosoftClient) AuthenticateXBL(context.Context, string) (string, string, error) {
	f.calls = append(f.calls, "xbl")
	return "xbl_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateXSTS(context.Context, string) (string, string, error) {
	f.calls = append(f.calls, "xsts")
	return "xsts_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateMinecraft(context.Context, string, string) (string, error) {
	f.calls = append(f.calls, "minecraft")
	return "mc_access_token", nil
}

func (f *fakeMicrosoftClient) CheckGameOwnership(context.Context, string) (bool, error) {
	f.calls = append(f.calls, "ownership")
	return true, nil
}

func (f *fakeMicrosoftClient) GetMinecraftProfile(context.Context, string) (map[string]any, error) {
	f.calls = append(f.calls, "profile")
	return map[string]any{"id": "uuid", "name": "McPlayer"}, nil
}

type missingAccessMicrosoftClient struct {
	fakeMicrosoftClient
}

func (m *missingAccessMicrosoftClient) ExchangeCodeForToken(context.Context, string) (map[string]any, error) {
	return map[string]any{}, nil
}

func TestMicrosoftAuthFlowComplete(t *testing.T) {
	client := &fakeMicrosoftClient{}
	res, err := (microsoft.MicrosoftAuthFlow{Client: client}).Complete(context.Background(), "auth_code")
	if err != nil {
		t.Fatal(err)
	}
	if res["mc_access_token"] != "mc_access_token" || res["has_game"] != true {
		t.Fatalf("unexpected auth flow result: %#v", res)
	}
	profile := res["profile"].(map[string]any)
	if profile["name"] != "McPlayer" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
	want := []string{"exchange", "xbl", "xsts", "minecraft", "ownership", "profile"}
	if strings.Join(client.calls, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected call order: %#v", client.calls)
	}
}

func TestMicrosoftAuthFlowRejectsMissingAccessToken(t *testing.T) {
	_, err := (microsoft.MicrosoftAuthFlow{Client: &missingAccessMicrosoftClient{}}).Complete(context.Background(), "auth_code")
	if err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing access_token error, got %v", err)
	}
}
