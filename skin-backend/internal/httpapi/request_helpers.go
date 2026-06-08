package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"element-skin/backend/internal/util"
)

func decodeJSON(req *http.Request, dst any) error {
	defer req.Body.Close()
	return json.NewDecoder(req.Body).Decode(dst)
}

func multipartFileBytes(req *http.Request, field string, maxBytes int64) ([]byte, error) {
	file, _, err := req.FormFile(field)
	if err != nil {
		return nil, util.HTTPError{Status: 400, Detail: "file is required"}
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, util.HTTPError{Status: 400, Detail: "File too large"}
	}
	return data, nil
}
