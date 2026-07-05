package notice_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	noticesvc "element-skin/backend/internal/service/notice"
	"element-skin/backend/internal/testutil"
)

func TestNoticeServiceValidatesInputsWithoutPersistingInvalidRows(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	admin := testutil.CreateUser(t, db, "notice-service-admin@test.com", "Password123", "NoticeServiceAdmin", true)
	actor := noticeManagerActor(admin.ID)
	ctx := context.Background()

	cases := []struct {
		name  string
		input noticesvc.CreateInput
		want  string
	}{
		{
			name:  "detail requires summary",
			input: noticesvc.CreateInput{Title: "Detail", ContentMarkdown: "Body", DisplayMode: noticesvc.DisplayDetail},
			want:  "summary is required for detail notices",
		},
		{
			name:  "detail requires content",
			input: noticesvc.CreateInput{Title: "Detail", Summary: "Summary", DisplayMode: noticesvc.DisplayDetail},
			want:  "content_markdown is required for detail notices",
		},
		{
			name:  "invalid link protocol",
			input: noticesvc.CreateInput{Title: "Bad Link", ContentMarkdown: "Body", LinkText: "Open", LinkURL: "javascript:alert(1)"},
			want:  "invalid link_url",
		},
		{
			name:  "link text pair required",
			input: noticesvc.CreateInput{Title: "Half Link", ContentMarkdown: "Body", LinkURL: "/notifications/abc"},
			want:  "link_text and link_url must be provided together",
		},
		{
			name:  "ends after starts",
			input: noticesvc.CreateInput{Title: "Bad Time", ContentMarkdown: "Body", StartsAt: ptrInt64(20), EndsAt: ptrInt64(10)},
			want:  "ends_at must be greater than starts_at",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			created, err := svc.Create(ctx, actor, tc.input)
			if created != nil || !httpError(err, 400, tc.want) {
				t.Fatalf("Create()=%#v err=%#v; want nil and %q", created, err, tc.want)
			}
		})
	}
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM notices`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid notice creates persisted %d rows; want 0", count)
	}

	inline, err := svc.Create(ctx, actor, noticesvc.CreateInput{Title: "Inline", Summary: "Short text", DisplayMode: noticesvc.DisplayInline})
	if err != nil {
		t.Fatalf("inline notice without content should be valid: %v", err)
	}
	if inline.Title != "Inline" || inline.Summary != "Short text" || inline.ContentMarkdown != "" || inline.DisplayMode != noticesvc.DisplayInline {
		t.Fatalf("inline notice without content mismatch: %#v", inline)
	}

	system, err := svc.Create(ctx, actor, noticesvc.CreateInput{Type: noticesvc.TypeSystem, Title: "System", Summary: "System text", DisplayMode: noticesvc.DisplayInline})
	if err != nil {
		t.Fatalf("system notice should be valid: %v", err)
	}
	if system.Type != noticesvc.TypeSystem || system.Title != "System" || system.Summary != "System text" || system.ContentMarkdown != "" {
		t.Fatalf("system notice mismatch: %#v", system)
	}
}

func TestNoticeServiceValidationCoversLengthsLevelsAudienceLinksAndPatchClearsExactly(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	svc := noticesvc.Service{DB: db}
	ctx := context.Background()
	admin := testutil.CreateUser(t, db, "notice-service-validation2@test.com", "Password123", "NoticeServiceValidation2", true)
	actor := noticeManagerActor(admin.ID)
	base := noticesvc.CreateInput{Title: "Valid", ContentMarkdown: "Body"}
	cases := []struct {
		name   string
		mutate func(*noticesvc.CreateInput)
		detail string
	}{
		{"invalid type", func(in *noticesvc.CreateInput) { in.Type = "other" }, "invalid type"},
		{"title required", func(in *noticesvc.CreateInput) { in.Title = " " }, "title is required"},
		{"title too long", func(in *noticesvc.CreateInput) { in.Title = strings.Repeat("测", noticesvc.MaxTitleLen+1) }, "title too long"},
		{"summary too long", func(in *noticesvc.CreateInput) { in.Summary = strings.Repeat("测", noticesvc.MaxSummaryLen+1) }, "summary too long"},
		{"content too long", func(in *noticesvc.CreateInput) { in.ContentMarkdown = strings.Repeat("a", noticesvc.MaxContentLen+1) }, "content_markdown too long"},
		{"invalid display", func(in *noticesvc.CreateInput) { in.DisplayMode = "popup" }, "invalid display_mode"},
		{"invalid level", func(in *noticesvc.CreateInput) { in.Level = "loud" }, "invalid level"},
		{"invalid audience", func(in *noticesvc.CreateInput) { in.Audience = "guests" }, "invalid audience"},
		{"unsafe protocol-relative link", func(in *noticesvc.CreateInput) { in.LinkText = "Open"; in.LinkURL = "//evil.example" }, "invalid link_url"},
		{"unsafe control link", func(in *noticesvc.CreateInput) { in.LinkText = "Open"; in.LinkURL = "/ok\nbad" }, "invalid link_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := base
			tc.mutate(&input)
			got, err := svc.Create(ctx, actor, input)
			if got != nil || !httpError(err, 400, tc.detail) {
				t.Fatalf("Create invalid case got=%#v err=%#v want %q", got, err, tc.detail)
			}
		})
	}
	start := database.NowMS() - 1000
	end := database.NowMS() + 1000
	notice, err := svc.Create(ctx, actor, noticesvc.CreateInput{
		Title:           "Patch clear",
		Summary:         "Patch summary",
		ContentMarkdown: "Patch body",
		DisplayMode:     noticesvc.DisplayDetail,
		StartsAt:        &start,
		EndsAt:          &end,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := svc.Patch(ctx, actor, notice.ID, noticesvc.PatchInput{
		DisplayMode:     ptrString(noticesvc.DisplayInline),
		ContentMarkdown: ptrString(""),
		ClearStartsAt:   true,
		ClearEndsAt:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.StartsAt != nil || updated.EndsAt != nil || updated.DisplayMode != noticesvc.DisplayInline || updated.ContentMarkdown != "" {
		t.Fatalf("patch clear fields mismatch: %#v", updated)
	}
}
