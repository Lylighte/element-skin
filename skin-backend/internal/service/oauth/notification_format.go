package oauth

import (
	"strings"
	"time"

	"element-skin/backend/internal/database"
	noticesvc "element-skin/backend/internal/service/notice"
)

func noticeExpiresAt() *int64 {
	expiresAt := database.NowMS() + int64(reviewNoticeTTL/time.Millisecond)
	return &expiresAt
}

func reviewStatusLabel(status string) string {
	switch status {
	case StatusActive:
		return "已通过"
	case StatusRejected:
		return "已驳回"
	case StatusDisabled:
		return "已停用"
	default:
		return status
	}
}

func fitNoticeTitle(prefix, value string) string {
	return fitNoticeText(prefix+value, noticesvc.MaxTitleLen)
}

func fitNoticeText(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	if maxRunes <= 1 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-1]) + "…"
}
