package microsoft_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

func TestMicrosoftHTTPClientMinecraftRequestBody(t *testing.T) {
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body %s: %v", body, err)
		}
		if req.URL.String() != "https://api.minecraftservices.com/authentication/login_with_xbox" {
			t.Fatalf("unexpected URL: %s", req.URL.String())
		}
		if payload["identityToken"] != "XBL3.0 x=user_hash;xsts_token" {
			t.Fatalf("unexpected Minecraft payload: %#v", payload)
		}
		return jsonResponse(`{"access_token":"mc_access"}`), nil
	})}}

	mcToken, err := client.AuthenticateMinecraft(context.Background(), "user_hash", "xsts_token")
	if err != nil || mcToken != "mc_access" {
		t.Fatalf("minecraft got token=%q err=%v", mcToken, err)
	}
}

func TestMicrosoftHTTPClientRejectsMalformedMinecraftResponse(t *testing.T) {
	minecraftClient := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{}`), nil
	})}}
	if _, err := minecraftClient.AuthenticateMinecraft(context.Background(), "uhs", "xsts"); err == nil || !strings.Contains(err.Error(), "access_token") {
		t.Fatalf("expected missing minecraft access_token error, got %v", err)
	}
}
