package notice

import (
	"strings"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func requirePermission(actor permission.Actor, def permission.Definition) error {
	if actor.Has(def) {
		return nil
	}
	return util.HTTPError{Status: 403, Detail: "permission denied"}
}

func requireCreatePermission(actor permission.Actor, input CreateInput) error {
	if actor.Has(noticeCreatePermission) {
		return nil
	}
	typ := strings.TrimSpace(input.Type)
	if typ == "" {
		typ = TypeAnnouncement
	}
	if typ == TypeSystem && actor.Has(noticeCreateSystemPermission) {
		return nil
	}
	return util.HTTPError{Status: 403, Detail: "permission denied"}
}

func visibleToUser(item model.NoticeView, user CurrentUser, now int64) bool {
	if !item.Enabled {
		return false
	}
	if item.StartsAt != nil && *item.StartsAt > now {
		return false
	}
	if item.EndsAt != nil && *item.EndsAt <= now {
		return false
	}
	if item.Audience == AudienceAdmins && !user.CanReadAdminAudience {
		return false
	}
	return item.Audience == AudienceUsers || item.Audience == AudienceAdmins || item.Audience == AudienceTargeted
}
