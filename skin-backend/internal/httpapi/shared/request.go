package shared

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

const MaxJSONBodyBytes = 1 << 20

var (
	ErrJSONBodyTooLarge   = errors.New("json body too large")
	ErrMultipleJSONValues = errors.New("multiple json values")
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
	data, err := io.ReadAll(io.LimitReader(req.Body, MaxJSONBodyBytes+1))
	if err != nil {
		return err
	}
	if len(data) > MaxJSONBodyBytes {
		return ErrJSONBodyTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return ErrMultipleJSONValues
		}
		return err
	}
	return nil
}
