package notice

import (
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func actorCreatedBy(actor permission.Actor) *string {
	if actor.UserID != "" {
		return &actor.UserID
	}
	return nil
}

func noticeFromCreate(input CreateInput, createdBy *string) (model.Notice, error) {
	id, err := util.GenerateUUIDNoDash()
	if err != nil {
		return model.Notice{}, err
	}
	now := database.NowMS()
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	pinned := false
	if input.Pinned != nil {
		pinned = *input.Pinned
	}
	dismissible := true
	if input.Dismissible != nil {
		dismissible = *input.Dismissible
	}
	typ := strings.TrimSpace(input.Type)
	if typ == "" {
		typ = TypeAnnouncement
	}
	displayMode := strings.TrimSpace(input.DisplayMode)
	if displayMode == "" {
		displayMode = DisplayInline
	}
	level := strings.TrimSpace(input.Level)
	if level == "" {
		level = LevelInfo
	}
	audience := strings.TrimSpace(input.Audience)
	if audience == "" {
		audience = AudienceUsers
	}
	notice := model.Notice{
		ID:              id,
		Type:            typ,
		Title:           strings.TrimSpace(input.Title),
		Summary:         strings.TrimSpace(input.Summary),
		ContentMarkdown: strings.TrimSpace(input.ContentMarkdown),
		DisplayMode:     displayMode,
		Level:           level,
		LinkText:        strings.TrimSpace(input.LinkText),
		LinkURL:         strings.TrimSpace(input.LinkURL),
		Audience:        audience,
		Enabled:         enabled,
		Pinned:          pinned,
		Dismissible:     dismissible,
		StartsAt:        input.StartsAt,
		EndsAt:          input.EndsAt,
		CreatedBy:       createdBy,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	return notice, validateNotice(notice)
}

func applyPatch(notice *model.Notice, input PatchInput) {
	if input.Type != nil {
		notice.Type = strings.TrimSpace(*input.Type)
	}
	if input.Title != nil {
		notice.Title = strings.TrimSpace(*input.Title)
	}
	if input.Summary != nil {
		notice.Summary = strings.TrimSpace(*input.Summary)
	}
	if input.ContentMarkdown != nil {
		notice.ContentMarkdown = strings.TrimSpace(*input.ContentMarkdown)
	}
	if input.DisplayMode != nil {
		notice.DisplayMode = strings.TrimSpace(*input.DisplayMode)
	}
	if input.Level != nil {
		notice.Level = strings.TrimSpace(*input.Level)
	}
	if input.LinkText != nil {
		notice.LinkText = strings.TrimSpace(*input.LinkText)
	}
	if input.LinkURL != nil {
		notice.LinkURL = strings.TrimSpace(*input.LinkURL)
	}
	if input.Audience != nil {
		notice.Audience = strings.TrimSpace(*input.Audience)
	}
	if input.Enabled != nil {
		notice.Enabled = *input.Enabled
	}
	if input.Pinned != nil {
		notice.Pinned = *input.Pinned
	}
	if input.Dismissible != nil {
		notice.Dismissible = *input.Dismissible
	}
	if input.StartsAt != nil {
		notice.StartsAt = input.StartsAt
	} else if input.ClearStartsAt {
		notice.StartsAt = nil
	}
	if input.EndsAt != nil {
		notice.EndsAt = input.EndsAt
	} else if input.ClearEndsAt {
		notice.EndsAt = nil
	}
}
