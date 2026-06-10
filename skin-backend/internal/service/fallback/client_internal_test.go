package fallback

import (
	"testing"
	"time"
)

func TestFallbackEndpointKeyUsesStablePriorityOrder(t *testing.T) {
	tests := []struct {
		name string
		ep   map[string]any
		want string
	}{
		{name: "numeric id", ep: map[string]any{"id": 7, "session_url": "https://session.example"}, want: "7"},
		{name: "session url", ep: map[string]any{"session_url": "https://session.example", "account_url": "https://account.example"}, want: "https://session.example"},
		{name: "account url", ep: map[string]any{"account_url": "https://account.example", "services_url": "https://services.example"}, want: "https://account.example"},
		{name: "services url", ep: map[string]any{"services_url": "https://services.example"}, want: "https://services.example"},
		{name: "unknown", ep: map[string]any{"id": 0}, want: "unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fallbackEndpointKey(tc.ep); got != tc.want {
				t.Fatalf("fallbackEndpointKey(%#v)=%q, want %q", tc.ep, got, tc.want)
			}
		})
	}
}

func TestFallbackTTLNeverDropsBelowLoopGuardMinimum(t *testing.T) {
	tests := []struct {
		name string
		ep   map[string]any
		want time.Duration
	}{
		{name: "missing", ep: map[string]any{}, want: 30 * time.Second},
		{name: "zero", ep: map[string]any{"cache_ttl": 0}, want: 30 * time.Second},
		{name: "below minimum", ep: map[string]any{"cache_ttl": 10}, want: 30 * time.Second},
		{name: "at minimum", ep: map[string]any{"cache_ttl": 30}, want: 30 * time.Second},
		{name: "custom", ep: map[string]any{"cache_ttl": 120}, want: 2 * time.Minute},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fallbackTTL(tc.ep); got != tc.want {
				t.Fatalf("fallbackTTL(%#v)=%v, want %v", tc.ep, got, tc.want)
			}
		})
	}
}
