package shared

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func ParsePositiveInt(raw string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid positive int")
	}
	return n, nil
}

func BearerToken(req *http.Request) (string, bool) {
	auth := req.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	return token, token != ""
}

func FormBool(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	return raw == "true" || raw == "1" || raw == "yes" || raw == "on"
}

func DecodeJSON(req *http.Request, dst any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(dst)
}
