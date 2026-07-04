package notice

import (
	"context"
	"net/http"
	"strings"

	"element-skin/backend/internal/database"
	noticedb "element-skin/backend/internal/database/notice"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
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

func (s Service) ListForUser(ctx context.Context, user CurrentUser, params ListParams) (map[string]any, error) {
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	typ := strings.TrimSpace(params.Type)
	if params.Dashboard && typ == "" {
		typ = TypeAnnouncement
	}
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	return s.DB.Notices.ListForUser(ctx, noticedb.UserListOptions{
		UserID:               user.ID,
		CanReadAdminAudience: user.CanReadAdminAudience,
		Type:                 typ,
		Limit:                params.Limit,
		Now:                  database.NowMS(),
		IncludeRead:          params.IncludeRead || params.Dashboard,
		LastPinned:           cur.lastPinned,
		LastCreated:          cur.lastCreated,
		LastID:               cur.lastID,
	})
}

func (s Service) GetForUser(ctx context.Context, id string, user CurrentUser) (*model.NoticeView, error) {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return nil, err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	now := database.NowMS()
	if err := s.DB.Notices.MarkRead(ctx, id, user.ID, now); err != nil {
		return nil, err
	}
	if item.ReadAt == nil {
		item.ReadAt = &now
		item.Read = true
	}
	return item, nil
}

func (s Service) MarkRead(ctx context.Context, id string, user CurrentUser) error {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return s.DB.Notices.MarkRead(ctx, id, user.ID, database.NowMS())
}

func (s Service) Dismiss(ctx context.Context, id string, user CurrentUser) error {
	item, err := s.DB.Notices.GetForUser(ctx, id, user.ID, user.CanReadAdminAudience)
	if err != nil {
		return err
	}
	if item == nil || !visibleToUser(*item, user, database.NowMS()) {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	if !item.Dismissible {
		return util.HTTPError{Status: http.StatusForbidden, Detail: "notice is not dismissible"}
	}
	return s.DB.Notices.Dismiss(ctx, id, user.ID, database.NowMS())
}

func (s Service) ListForManagement(ctx context.Context, actor permission.Actor, params ListParams) (map[string]any, error) {
	if err := requirePermission(actor, noticeReadPermission); err != nil {
		return nil, err
	}
	cur, err := parseCursor(params.Cursor)
	if err != nil {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "Invalid cursor"}
	}
	status := strings.TrimSpace(params.Status)
	if status == "" {
		status = StatusAll
	}
	if !validStatus(status) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid status"}
	}
	typ := strings.TrimSpace(params.Type)
	if typ != "" && !validType(typ) {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "invalid type"}
	}
	return s.DB.Notices.ListForAdmin(ctx, noticedb.AdminListOptions{
		Type:        typ,
		Status:      status,
		Limit:       params.Limit,
		Now:         database.NowMS(),
		LastPinned:  cur.lastPinned,
		LastCreated: cur.lastCreated,
		LastID:      cur.lastID,
	})
}

func (s Service) Create(ctx context.Context, actor permission.Actor, input CreateInput) (*model.Notice, error) {
	if err := requireCreatePermission(actor, input); err != nil {
		return nil, err
	}
	notice, err := noticeFromCreate(input, actorCreatedBy(actor))
	if err != nil {
		return nil, err
	}
	targets, err := normalizedTargetUserIDs(input.TargetUserIDs, notice.Audience)
	if err != nil {
		return nil, err
	}
	if notice.Audience == AudienceTargeted {
		err = s.DB.Notices.CreateWithTargets(ctx, notice, targets)
	} else {
		err = s.DB.Notices.Create(ctx, notice)
	}
	if err != nil {
		return nil, err
	}
	return &notice, nil
}

func (s Service) Patch(ctx context.Context, actor permission.Actor, id string, input PatchInput) (*model.Notice, error) {
	if err := requirePermission(actor, noticeUpdatePermission); err != nil {
		return nil, err
	}
	existing, err := s.DB.Notices.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	updated := *existing
	applyPatch(&updated, input)
	newID, err := util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	now := database.NowMS()
	updated.ID = newID
	updated.CreatedAt = now
	updated.UpdatedAt = now
	updated.CreatedBy = actorCreatedBy(actor)
	if err := validateNotice(updated); err != nil {
		return nil, err
	}
	if updated.Audience == AudienceTargeted && existing.Audience != AudienceTargeted {
		return nil, util.HTTPError{Status: http.StatusBadRequest, Detail: "target_user_ids are required for targeted notices"}
	}
	ok, err := s.DB.Notices.Replace(ctx, id, updated)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return &updated, nil
}

func (s Service) Delete(ctx context.Context, actor permission.Actor, id string) error {
	if err := requirePermission(actor, noticeDeletePermission); err != nil {
		return err
	}
	ok, err := s.DB.Notices.Delete(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return util.HTTPError{Status: http.StatusNotFound, Detail: "notice not found"}
	}
	return nil
}

func (s Service) DeleteExpired(ctx context.Context, actor permission.Actor, cutoff int64) error {
	if err := requirePermission(actor, noticeDeleteSystemPermission); err != nil {
		return err
	}
	return s.DB.Notices.DeleteExpired(ctx, cutoff)
}
