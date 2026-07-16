package shared

import (
	"io"
	"net/http"

	"element-skin/backend/internal/util"
)

const (
	MaxMultipartParts      = 32
	MaxMultipartFieldBytes = 4096
)

func MultipartFileBytes(req *http.Request, field string, maxBytes int64) ([]byte, error) {
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

type MultipartUpload struct {
	Filename string
	Data     []byte
	Fields   map[string]string
}

func ReadMultipartUpload(req *http.Request, fileField string, maxBytes int64) (MultipartUpload, error) {
	reader, err := req.MultipartReader()
	if err != nil {
		return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "invalid multipart form"}
	}
	out := MultipartUpload{Fields: map[string]string{}}
	partCount := 0
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "invalid multipart form"}
		}
		partCount++
		if partCount > MaxMultipartParts {
			_ = part.Close()
			return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "too many multipart fields"}
		}
		formName := part.FormName()
		if formName == "" {
			_ = part.Close()
			continue
		}
		if formName == fileField {
			if out.Filename != "" {
				_ = part.Close()
				return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "duplicate file field"}
			}
			out.Filename = part.FileName()
			data, err := io.ReadAll(io.LimitReader(part, maxBytes+1))
			_ = part.Close()
			if err != nil {
				return MultipartUpload{}, err
			}
			if int64(len(data)) > maxBytes {
				return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "File too large"}
			}
			out.Data = data
			continue
		}
		if _, exists := out.Fields[formName]; exists {
			_ = part.Close()
			return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "duplicate multipart field"}
		}
		data, err := io.ReadAll(io.LimitReader(part, MaxMultipartFieldBytes+1))
		_ = part.Close()
		if err != nil {
			return MultipartUpload{}, err
		}
		if len(data) > MaxMultipartFieldBytes {
			return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "multipart field too large"}
		}
		out.Fields[formName] = string(data)
	}
	if out.Filename == "" {
		return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "file is required"}
	}
	return out, nil
}
