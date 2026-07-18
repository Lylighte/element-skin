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

func TestMicrosoftHTTPClientXboxRequestBodies(t *testing.T) {
	var seen []string
	client := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Fatal(err)
		}
		var payload map[string]any
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request body %s: %v", body, err)
		}
		seen = append(seen, req.URL.String())
		switch req.URL.String() {
		case "https://user.auth.xboxlive.com/user/authenticate":
			props := payload["Properties"].(map[string]any)
			if props["RpsTicket"] != "d=ms_access" || payload["RelyingParty"] != "http://auth.xboxlive.com" || payload["TokenType"] != "JWT" {
				t.Fatalf("unexpected XBL payload: %#v", payload)
			}
			return jsonResponse(`{"Token":"xbl_token","DisplayClaims":{"xui":[{"uhs":"user_hash"}]}}`), nil
		case "https://xsts.auth.xboxlive.com/xsts/authorize":
			props := payload["Properties"].(map[string]any)
			tokens := props["UserTokens"].([]any)
			if len(tokens) != 1 || tokens[0] != "xbl_token" || props["SandboxId"] != "RETAIL" || payload["RelyingParty"] != "rp://api.minecraftservices.com/" {
				t.Fatalf("unexpected XSTS payload: %#v", payload)
			}
			return jsonResponse(`{"Token":"xsts_token","DisplayClaims":{"xui":[{"uhs":"user_hash"}]}}`), nil
		default:
			t.Fatalf("unexpected URL: %s", req.URL.String())
			return nil, nil
		}
	})}}

	xblToken, uhs, err := client.AuthenticateXBL(context.Background(), "ms_access")
	if err != nil || xblToken != "xbl_token" || uhs != "user_hash" {
		t.Fatalf("xbl got token=%q uhs=%q err=%v", xblToken, uhs, err)
	}
	xstsToken, uhs, err := client.AuthenticateXSTS(context.Background(), xblToken)
	if err != nil || xstsToken != "xsts_token" || uhs != "user_hash" {
		t.Fatalf("xsts got token=%q uhs=%q err=%v", xstsToken, uhs, err)
	}
	want := strings.Join([]string{
		"https://user.auth.xboxlive.com/user/authenticate",
		"https://xsts.auth.xboxlive.com/xsts/authorize",
	}, ",")
	if strings.Join(seen, ",") != want {
		t.Fatalf("unexpected call sequence: %#v", seen)
	}
}

func TestMicrosoftHTTPClientRejectsMalformedXboxResponses(t *testing.T) {
	xboxClient := microsoft.MicrosoftHTTPClient{Client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return jsonResponse(`{"DisplayClaims":{"xui":[]}}`), nil
	})}}
	if _, _, err := xboxClient.AuthenticateXBL(context.Background(), "ms_access"); err == nil || !strings.Contains(err.Error(), "missing token") {
		t.Fatalf("expected malformed xbox response error, got %v", err)
	}
}
