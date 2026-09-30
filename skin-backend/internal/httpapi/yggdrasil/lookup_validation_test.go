package yggdrasil

import "testing"

func TestValidateServicesBulkNamesMatchesMojangBounds(t *testing.T) {
	cases := []struct {
		name  string
		names []string
		valid bool
	}{
		{name: "one name", names: []string{"Player"}, valid: true},
		{name: "ten names", names: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"}, valid: true},
		{name: "empty", names: []string{}, valid: false},
		{name: "too many", names: []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10", "11"}, valid: false},
		{name: "empty name", names: []string{""}, valid: false},
		{name: "invalid character", names: []string{"Player-1"}, valid: false},
		{name: "too long", names: []string{"abcdefghijklmnopq"}, valid: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := validateServicesBulkNames(tc.names); got != tc.valid {
				t.Fatalf("validateServicesBulkNames(%#v)=%t, want %t", tc.names, got, tc.valid)
			}
		})
	}
}
