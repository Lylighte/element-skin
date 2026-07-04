package notice

import (
	"strconv"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func addCursorWhere(where string, args *[]any, columnPrefix string, lastPinned *bool, lastCreated *int64, lastID string) string {
	if lastPinned == nil || lastCreated == nil || lastID == "" {
		return where
	}
	pinned := 0
	if *lastPinned {
		pinned = 1
	}
	*args = append(*args, pinned, *lastCreated, lastID)
	i := len(*args) - 2
	return where + ` AND ((CASE WHEN ` + columnPrefix + `pinned THEN 1 ELSE 0 END) < $` + strconv.Itoa(i) +
		` OR ((CASE WHEN ` + columnPrefix + `pinned THEN 1 ELSE 0 END) = $` + strconv.Itoa(i) +
		` AND (` + columnPrefix + `created_at < $` + strconv.Itoa(i+1) +
		` OR (` + columnPrefix + `created_at = $` + strconv.Itoa(i+1) + ` AND ` + columnPrefix + `id < $` + strconv.Itoa(i+2) + `))))`
}

func noticePage(items []model.Notice, limit int) map[string]any {
	hasNext := len(items) > limit
	page := items
	if hasNext {
		page = items[:limit]
	}
	next := noticeCursor(page, hasNext)
	return map[string]any{"items": page, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(page)}
}

func noticeViewPage(items []model.NoticeView, limit int) map[string]any {
	hasNext := len(items) > limit
	page := items
	if hasNext {
		page = items[:limit]
	}
	next := noticeViewCursor(page, hasNext)
	return map[string]any{"items": page, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(page)}
}

func noticeCursor(items []model.Notice, hasNext bool) map[string]any {
	if !hasNext || len(items) == 0 {
		return nil
	}
	last := items[len(items)-1]
	return map[string]any{"last_pinned": last.Pinned, "last_created_at": last.CreatedAt, "last_id": last.ID}
}

func noticeViewCursor(items []model.NoticeView, hasNext bool) map[string]any {
	if !hasNext || len(items) == 0 {
		return nil
	}
	last := items[len(items)-1]
	return map[string]any{"last_pinned": last.Pinned, "last_created_at": last.CreatedAt, "last_id": last.ID}
}
