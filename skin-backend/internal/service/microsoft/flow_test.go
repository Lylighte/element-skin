package microsoft_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

type fakeMicrosoftClient struct {
	calls  []string
	failAt string
}

func (f *fakeMicrosoftClient) stage(name string) error {
	f.calls = append(f.calls, name)
	if f.failAt == name {
		return errors.New(name + " failed")
	}
	return nil
}

func (f *fakeMicrosoftClient) ExchangeCodeForToken(context.Context, string) (map[string]any, error) {
	if err := f.stage("exchange"); err != nil {
		return nil, err
	}
	return map[string]any{"access_token": "ms_access_token"}, nil
}

func (f *fakeMicrosoftClient) AuthenticateXBL(context.Context, string) (string, string, error) {
	if err := f.stage("xbl"); err != nil {
		return "", "", err
	}
	return "xbl_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateXSTS(context.Context, string) (string, string, error) {
	if err := f.stage("xsts"); err != nil {
		return "", "", err
	}
	return "xsts_token", "user_hash", nil
}

func (f *fakeMicrosoftClient) AuthenticateMinecraft(context.Context, string, string) (string, error) {
	if err := f.stage("minecraft"); err != nil {
		return "", err
	}
	return "mc_access_token", nil
}

func (f *fakeMicrosoftClient) CheckGameOwnership(context.Context, string) (bool, error) {
	if err := f.stage("ownership"); err != nil {
		return false, err
	}
	return true, nil
}

func (f *fakeMicrosoftClient) GetMinecraftProfile(context.Context, string) (map[string]any, error) {
	if err := f.stage("profile"); err != nil {
		return nil, err
	}
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

func TestMicrosoftAuthFlowStopsExactlyAtEachFailedStage(t *testing.T) {
	stages := []string{"exchange", "xbl", "xsts", "minecraft", "ownership", "profile"}
	for failedIndex, stage := range stages {
		t.Run(stage, func(t *testing.T) {
			client := &fakeMicrosoftClient{failAt: stage}
			result, err := (microsoft.MicrosoftAuthFlow{Client: client}).Complete(context.Background(), "auth_code")
			if result != nil || err == nil || err.Error() != stage+" failed" {
				t.Fatalf("failed stage %q result=%#v err=%v; want nil and exact stage error", stage, result, err)
			}
			wantCalls := stages[:failedIndex+1]
			if strings.Join(client.calls, ",") != strings.Join(wantCalls, ",") {
				t.Fatalf("failed stage %q calls=%#v; want exact prefix %#v", stage, client.calls, wantCalls)
			}
		})
	}
}
