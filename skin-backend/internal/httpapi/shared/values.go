package shared

import (
	"strings"

	"element-skin/backend/internal/util"
)

func AsMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func PublicBool(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case float64:
		return x != 0
	case int:
		return x != 0
	case string:
		return x == "true" || x == "1"
	default:
		return false
	}
}

func ValidPublicValue(v any) bool {
	switch x := v.(type) {
	case bool:
		return true
	case float64:
		return x == 0 || x == 1
	case int:
		return x == 0 || x == 1
	case string:
		return x == "true" || x == "false" || x == "0" || x == "1"
	default:
		return false
	}
}

func ParseImportProfiles(raw any) ([]map[string]string, error) {
	items, ok := raw.([]any)
	if !ok {
		return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
	}
	if len(items) == 0 {
		return nil, util.HTTPError{Status: 400, Detail: "profiles cannot be empty"}
	}
	out := make([]map[string]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, util.HTTPError{Status: 400, Detail: "profiles must be a list"}
		}
		out = append(out, map[string]string{
			"profile_id":   strings.TrimSpace(AsString(m["profile_id"])),
			"profile_name": strings.TrimSpace(AsString(m["profile_name"])),
		})
	}
	return out, nil
}

func AsString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func ValueOrAny(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}
