package service

import (
	"testing"

	"element-skin/backend/internal/util"
)

func TestValidateFallbackServicesNormalizesExactValues(t *testing.T) {
	services, err := ValidateFallbackServices([]map[string]any{{
		"session_url":      " https://session.example ",
		"account_url":      "https://account.example",
		"services_url":     "https://services.example",
		"cache_ttl":        "30",
		"skin_domains":     []any{"skins.example", " cdn.example ", ""},
		"enable_profile":   "1",
		"enable_whitelist": 0,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 1 {
		t.Fatalf("expected one service, got %#v", services)
	}
	got := services[0]
	if got["session_url"] != "https://session.example" ||
		got["account_url"] != "https://account.example" ||
		got["services_url"] != "https://services.example" ||
		got["cache_ttl"] != 30 ||
		got["skin_domains"] != "skins.example,cdn.example" ||
		got["enable_profile"] != true ||
		got["enable_whitelist"] != false {
		t.Fatalf("unexpected normalized fallback service: %#v", got)
	}
}

func TestValidateFallbackEndpointsRejectsMissingURLs(t *testing.T) {
	_, err := ValidateFallbackEndpoints([]any{map[string]any{
		"session_url":  "https://session.example",
		"account_url":  "",
		"services_url": "https://services.example",
	}})
	httpErr, ok := err.(util.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T %[1]v", err)
	}
	if httpErr.Status != 400 || httpErr.Detail != "fallback[1] urls are required" {
		t.Fatalf("unexpected error: %#v", httpErr)
	}
}
