package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"element-skin/backend/internal/util"
)

func asMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	m, _ := v.(map[string]any)
	return m
}

func cursorCreatedHash(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	var created *int64
	switch v := m["last_created_at"].(type) {
	case float64:
		x := int64(v)
		created = &x
	case int64:
		created = &v
	}
	hash, _ := m[hashKey].(string)
	return created, hash, nil
}

func publicBool(v any) bool {
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

func validPublicValue(v any) bool {
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

func parseImportProfiles(raw any) ([]map[string]string, error) {
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
			"profile_id":   strings.TrimSpace(asString(m["profile_id"])),
			"profile_name": strings.TrimSpace(asString(m["profile_name"])),
		})
	}
	return out, nil
}

func asString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func valueOrAny(v any, fallback any) any {
	if v == nil {
		return fallback
	}
	return v
}

func parsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive int")
	}
	return n, nil
}

func bearerToken(req *http.Request) (string, bool) {
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

func formBool(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
}
