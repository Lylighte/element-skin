package settings

import (
	"reflect"
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

func TestFallbackNormalizationHelpersExactValues(t *testing.T) {
	if got := intValue("7", 1); got != 7 {
		t.Fatalf("intValue string mismatch: %d", got)
	}
	if got := intValue(float64(8), 1); got != 8 {
		t.Fatalf("intValue float mismatch: %d", got)
	}
	if got := intValue("bad", 9); got != 9 {
		t.Fatalf("intValue fallback mismatch: %d", got)
	}
	if !boolValue("1") || !boolValue(float64(1)) || boolValue("false") || boolValue(0) {
		t.Fatalf("boolValue exact coercion failed")
	}
	if got := normalizeDomains([]any{" skins.example ", "", "cdn.example"}); got != "skins.example,cdn.example" {
		t.Fatalf("normalizeDomains list mismatch: %q", got)
	}
	if got := valueOr(nil, "fallback"); got != "fallback" {
		t.Fatalf("valueOr nil mismatch: %#v", got)
	}
	if got := valueOr("value", "fallback"); got != "value" {
		t.Fatalf("valueOr value mismatch: %#v", got)
	}
}

func TestValidateFallbackInputsRejectInvalidShapesExactly(t *testing.T) {
	for _, tc := range []struct {
		name   string
		call   func() error
		detail string
	}{
		{
			name: "endpoints must be list",
			call: func() error {
				_, err := ValidateFallbackEndpoints(map[string]any{})
				return err
			},
			detail: "fallbacks must be a list",
		},
		{
			name: "endpoint must be object",
			call: func() error {
				_, err := ValidateFallbackEndpoints([]any{"not-an-object"})
				return err
			},
			detail: "invalid fallback entry",
		},
		{
			name: "services must be list",
			call: func() error {
				_, err := ValidateFallbackServices("not-a-list")
				return err
			},
			detail: "fallback_services must be a list",
		},
		{
			name: "service must be object",
			call: func() error {
				_, err := ValidateFallbackServices([]any{7})
				return err
			},
			detail: "fallback service must be an object",
		},
		{
			name: "service requires all URLs",
			call: func() error {
				_, err := ValidateFallbackServices([]any{map[string]any{
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "",
				}})
				return err
			},
			detail: "fallback[1] urls are required",
		},
		{
			name: "negative cache TTL",
			call: func() error {
				_, err := ValidateFallbackServices([]any{map[string]any{
					"session_url":  "https://session.example",
					"account_url":  "https://account.example",
					"services_url": "https://services.example",
					"cache_ttl":    -1,
				}})
				return err
			},
			detail: "fallback[1] cache_ttl must be non-negative",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			httpErr, ok := err.(util.HTTPError)
			if !ok || httpErr.Status != 400 || httpErr.Detail != tc.detail {
				t.Fatalf("error=%#v; want exact HTTP 400 detail %q", err, tc.detail)
			}
		})
	}
}

func TestValidateFallbackEndpointsNormalizesJSONCompatibleValuesExactly(t *testing.T) {
	endpoints, err := ValidateFallbackEndpoints([]any{map[string]any{
		"priority":         int64(4),
		"session_url":      " https://session.example ",
		"account_url":      " https://account.example ",
		"services_url":     " https://services.example ",
		"cache_ttl":        int64(90),
		"skin_domains":     []string{" skins.example ", "", "cdn.example"},
		"enable_profile":   true,
		"enable_hasjoined": 0,
		"enable_whitelist": float64(1),
		"note":             " primary ",
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := []any{
		4,
		"https://session.example",
		"https://account.example",
		"https://services.example",
		90,
		"skins.example,cdn.example",
		true,
		false,
		true,
		"primary",
	}
	got := endpoints[0]
	actual := []any{
		got.Priority,
		got.SessionURL,
		got.AccountURL,
		got.ServicesURL,
		got.CacheTTL,
		got.SkinDomains,
		got.EnableProfile,
		got.EnableHasJoined,
		got.EnableWhitelist,
		got.Note,
	}
	if len(endpoints) != 1 || !reflect.DeepEqual(actual, want) {
		t.Fatalf("normalized endpoint=%#v; want exact values %#v", got, want)
	}
}
