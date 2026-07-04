package texture

import (
	"errors"

	"element-skin/backend/internal/util"
)

func textureCursor(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m[hashKey].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", errors.New("invalid cursor")
	}
	created := &value
	return created, h, nil
}

func publicLibraryCursor(cursor string) (*int64, string, *int64, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", nil, err
	}
	createdValue, ok := util.CursorInt64(m["last_created_at"])
	h, hashOK := m["last_skin_hash"].(string)
	if !ok || !hashOK || h == "" {
		return nil, "", nil, errors.New("invalid cursor")
	}
	created := &createdValue
	var usage *int64
	if rawUsage, exists := m["last_usage_count"]; exists {
		usageValue, ok := util.CursorInt64(rawUsage)
		if !ok {
			return nil, "", nil, errors.New("invalid cursor")
		}
		usage = &usageValue
	}
	return created, h, usage, nil
}
