package shared

import (
	"errors"

	"element-skin/backend/internal/util"
)

func CursorCreatedHash(cursor, hashKey string) (*int64, string, error) {
	m, err := util.DecodeCursor(cursor)
	if err != nil || m == nil {
		return nil, "", err
	}
	value, ok := util.CursorInt64(m["last_created_at"])
	hash, hashOK := m[hashKey].(string)
	if !ok || !hashOK || hash == "" {
		return nil, "", errors.New("invalid cursor")
	}
	created := &value
	return created, hash, nil
}
