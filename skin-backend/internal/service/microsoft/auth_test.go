package microsoft_test

import (
	"strings"
	"testing"

	"element-skin/backend/internal/service/microsoft"
)

func TestMicrosoftAuthorizationURL(t *testing.T) {
	u := microsoft.MicrosoftAuthorizationURL("client_id", "https://redirect.com", "state123")
	if !strings.Contains(u, "client_id=client_id") || !strings.Contains(u, "state=state123") || !strings.Contains(u, "redirect_uri=https%3A%2F%2Fredirect.com") {
		t.Fatalf("unexpected authorization URL: %s", u)
	}
}
