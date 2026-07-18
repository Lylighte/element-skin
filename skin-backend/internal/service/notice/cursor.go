package notice

import (
	"errors"
	"strings"

	"element-skin/backend/internal/util"
)

type cursorState struct {
	lastPinned  *bool
	lastCreated *int64
	lastID      string
}

func parseCursor(raw string) (cursorState, error) {
	if strings.TrimSpace(raw) == "" {
		return cursorState{}, nil
	}
	m, err := util.DecodeCursor(raw)
	if err != nil || m == nil {
		return cursorState{}, errors.New("invalid cursor")
	}
	pinned, ok := m["last_pinned"].(bool)
	if !ok {
		return cursorState{}, errors.New("invalid cursor")
	}
	created, ok := util.CursorInt64(m["last_created_at"])
	if !ok {
		return cursorState{}, errors.New("invalid cursor")
	}
	id, ok := m["last_id"].(string)
	if !ok || id == "" {
		return cursorState{}, errors.New("invalid cursor")
	}
	return cursorState{lastPinned: &pinned, lastCreated: &created, lastID: id}, nil
}
