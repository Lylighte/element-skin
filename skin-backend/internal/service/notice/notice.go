package notice

import (
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
)

const (
	TypeAnnouncement = "announcement"
	TypeSystem       = "system"

	DisplayInline = "inline"
	DisplayDetail = "detail"

	LevelInfo    = "info"
	LevelSuccess = "success"
	LevelWarning = "warning"
	LevelDanger  = "danger"

	AudienceUsers    = "users"
	AudienceAdmins   = "admins"
	AudienceTargeted = "targeted"

	StatusAll       = "all"
	StatusEnabled   = "enabled"
	StatusDisabled  = "disabled"
	StatusExpired   = "expired"
	StatusScheduled = "scheduled"

	MaxTitleLen   = 80
	MaxSummaryLen = 160
	MaxContentLen = 20 * 1024
)

var (
	noticeReadPermission         = permission.MustDefinitionByCode("notice.read.any")
	noticeCreatePermission       = permission.MustDefinitionByCode("notice.create.any")
	noticeUpdatePermission       = permission.MustDefinitionByCode("notice.update.any")
	noticeDeletePermission       = permission.MustDefinitionByCode("notice.delete.any")
	noticeCreateSystemPermission = permission.MustDefinitionByCode("notice.create.system")
	noticeDeleteSystemPermission = permission.MustDefinitionByCode("notice.delete.system")
)

type Service struct {
	DB *database.DB
}

type CurrentUser struct {
	ID                   string
	CanReadAdminAudience bool
}

type ListParams struct {
	Type        string
	Status      string
	Limit       int
	Cursor      string
	IncludeRead bool
	Dashboard   bool
}

type CreateInput struct {
	Type            string   `json:"type"`
	Title           string   `json:"title"`
	Summary         string   `json:"summary"`
	ContentMarkdown string   `json:"content_markdown"`
	DisplayMode     string   `json:"display_mode"`
	Level           string   `json:"level"`
	LinkText        string   `json:"link_text"`
	LinkURL         string   `json:"link_url"`
	Audience        string   `json:"audience"`
	Enabled         *bool    `json:"enabled"`
	Pinned          *bool    `json:"pinned"`
	Dismissible     *bool    `json:"dismissible"`
	StartsAt        *int64   `json:"starts_at"`
	EndsAt          *int64   `json:"ends_at"`
	TargetUserIDs   []string `json:"target_user_ids"`
}

type PatchInput struct {
	Type            *string `json:"type"`
	Title           *string `json:"title"`
	Summary         *string `json:"summary"`
	ContentMarkdown *string `json:"content_markdown"`
	DisplayMode     *string `json:"display_mode"`
	Level           *string `json:"level"`
	LinkText        *string `json:"link_text"`
	LinkURL         *string `json:"link_url"`
	Audience        *string `json:"audience"`
	Enabled         *bool   `json:"enabled"`
	Pinned          *bool   `json:"pinned"`
	Dismissible     *bool   `json:"dismissible"`
	StartsAt        *int64  `json:"starts_at"`
	EndsAt          *int64  `json:"ends_at"`
	ClearStartsAt   bool    `json:"-"`
	ClearEndsAt     bool    `json:"-"`
}
