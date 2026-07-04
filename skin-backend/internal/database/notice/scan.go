package notice

import "element-skin/backend/internal/model"

func noticeSelectSQL() string {
	return `SELECT id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
		audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at FROM notices`
}

func noticeViewSelectSQL(userParam string) string {
	return `SELECT n.id,n.type,n.title,n.summary,n.content_markdown,n.display_mode,n.level,n.link_text,n.link_url,
		n.audience,n.enabled,n.pinned,n.dismissible,n.starts_at,n.ends_at,n.created_by,n.created_at,n.updated_at,
		r.read_at,r.dismissed_at
		FROM notices n LEFT JOIN notice_receipts r ON r.notice_id=n.id AND r.user_id=` + userParam
}

type noticeScanner interface {
	Scan(...any) error
}

func scanNotice(row noticeScanner) (model.Notice, error) {
	var n model.Notice
	err := row.Scan(&n.ID, &n.Type, &n.Title, &n.Summary, &n.ContentMarkdown, &n.DisplayMode, &n.Level, &n.LinkText, &n.LinkURL,
		&n.Audience, &n.Enabled, &n.Pinned, &n.Dismissible, &n.StartsAt, &n.EndsAt, &n.CreatedBy, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

func scanNoticeView(row noticeScanner) (model.NoticeView, error) {
	var v model.NoticeView
	err := row.Scan(&v.ID, &v.Type, &v.Title, &v.Summary, &v.ContentMarkdown, &v.DisplayMode, &v.Level, &v.LinkText, &v.LinkURL,
		&v.Audience, &v.Enabled, &v.Pinned, &v.Dismissible, &v.StartsAt, &v.EndsAt, &v.CreatedBy, &v.CreatedAt, &v.UpdatedAt, &v.ReadAt, &v.DismissedAt)
	v.Read = v.ReadAt != nil
	return v, err
}
