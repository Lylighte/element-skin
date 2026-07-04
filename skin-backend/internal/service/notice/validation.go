package notice

import (
	"net/http"
	"net/url"
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/util"
)

func validateNotice(notice model.Notice) error {
	if !validType(notice.Type) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	if notice.Title == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "title is required"}
	}
	if len([]rune(notice.Title)) > MaxTitleLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "title too long"}
	}
	if len([]rune(notice.Summary)) > MaxSummaryLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "summary too long"}
	}
	if len(notice.ContentMarkdown) > MaxContentLen {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "content_markdown too long"}
	}
	if notice.DisplayMode != DisplayInline && notice.DisplayMode != DisplayDetail {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid display_mode"}
	}
	if notice.DisplayMode == DisplayDetail && notice.Summary == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "summary is required for detail notices"}
	}
	if notice.DisplayMode == DisplayDetail && notice.ContentMarkdown == "" {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "content_markdown is required for detail notices"}
	}
	if !validLevel(notice.Level) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid level"}
	}
	if notice.Audience != AudienceUsers && notice.Audience != AudienceAdmins && notice.Audience != AudienceTargeted {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid audience"}
	}
	if (notice.LinkText == "") != (notice.LinkURL == "") {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "link_text and link_url must be provided together"}
	}
	if notice.LinkURL != "" && !safeNoticeLink(notice.LinkURL) {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid link_url"}
	}
	if notice.StartsAt != nil && notice.EndsAt != nil && *notice.EndsAt <= *notice.StartsAt {
		return util.HTTPError{Status: http.StatusBadRequest, Detail: "ends_at must be greater than starts_at"}
	}
	return nil
}

func normalizedTargetUserIDs(ids []string, audience string) ([]string, error) {
	seen := map[string]bool{}
	targets := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		targets = append(targets, id)
	}
	if audience == AudienceTargeted {
		if len(targets) == 0 {
			return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "target_user_ids are required for targeted notices"}
		}
		return targets, nil
	}
	if len(targets) > 0 {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "target_user_ids require targeted audience"}
	}
	return nil, nil
}

func validLevel(level string) bool {
	switch level {
	case LevelInfo, LevelSuccess, LevelWarning, LevelDanger:
		return true
	default:
		return false
	}
}

func validType(typ string) bool {
	return typ == TypeAnnouncement || typ == TypeSystem
}

func validStatus(status string) bool {
	switch status {
	case StatusAll, StatusEnabled, StatusDisabled, StatusExpired, StatusScheduled:
		return true
	default:
		return false
	}
}

func safeNoticeLink(raw string) bool {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "/") {
		return !strings.HasPrefix(raw, "//") && !strings.ContainsAny(raw, "\r\n\t")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}
