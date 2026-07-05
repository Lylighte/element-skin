package shared

import (
	"io"
	"net/http"

	"element-skin/backend/internal/util"
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
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "invalid multipart form"}
		}
		formName := part.FormName()
		if formName == "" {
			_ = part.Close()
			continue
		}
		if formName == fileField {
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
		data, err := io.ReadAll(io.LimitReader(part, 4097))
		_ = part.Close()
		if err != nil {
			return MultipartUpload{}, err
		}
		out.Fields[formName] = string(data)
	}
	if out.Filename == "" {
		return MultipartUpload{}, util.HTTPError{Status: 400, Detail: "file is required"}
	}
	return out, nil
}
