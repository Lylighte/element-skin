package notice

import (
	"context"
	"errors"
	"strconv"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) Get(ctx context.Context, id string) (*model.Notice, error) {
	row := s.Pool.QueryRow(ctx, noticeSelectSQL()+` WHERE id=$1`, id)
	n, err := scanNotice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s Store) GetForUser(ctx context.Context, id, userID string, canReadAdminAudience bool) (*model.NoticeView, error) {
	args := []any{id, userID}
	where := `n.id=$1 AND (n.audience='users'`
	if canReadAdminAudience {
		where += ` OR n.audience='admins'`
	}
	where += ` OR (n.audience='targeted' AND EXISTS (SELECT 1 FROM notice_targets nt WHERE nt.notice_id=n.id AND nt.user_id=$2)))`
	row := s.Pool.QueryRow(ctx, noticeViewSelectSQL("$2")+` WHERE `+where, args...)
	n, err := scanNoticeView(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (s Store) ListForUser(ctx context.Context, opts UserListOptions) (map[string]any, error) {
	actual := opts.Limit + 1
	args := []any{opts.UserID, opts.Now}
	where := `n.enabled=TRUE AND (n.starts_at IS NULL OR n.starts_at <= $2) AND (n.ends_at IS NULL OR n.ends_at > $2) AND (n.audience='users'`
	if opts.CanReadAdminAudience {
		where += ` OR n.audience='admins'`
	}
	where += ` OR (n.audience='targeted' AND EXISTS (SELECT 1 FROM notice_targets nt WHERE nt.notice_id=n.id AND nt.user_id=$1))) AND r.dismissed_at IS NULL`
	if opts.Type != "" {
		args = append(args, opts.Type)
		where += ` AND n.type=$` + strconv.Itoa(len(args))
	}
	if !opts.IncludeRead {
		where += ` AND r.read_at IS NULL`
	}
	where = addCursorWhere(where, &args, "n.", opts.LastPinned, opts.LastCreated, opts.LastID)
	args = append(args, actual)
	q := noticeViewSelectSQL("$1") + ` WHERE ` + where + ` ORDER BY n.pinned DESC, n.created_at DESC, n.id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []model.NoticeView{}
	for rows.Next() {
		item, err := scanNoticeView(rows)
		if err != nil {
			return nil, err
		}
		got = append(got, item)
	}
	return noticeViewPage(got, opts.Limit), rows.Err()
}

func (s Store) ListForAdmin(ctx context.Context, opts AdminListOptions) (map[string]any, error) {
	actual := opts.Limit + 1
	args := []any{}
	where := `TRUE`
	if opts.Type != "" {
		args = append(args, opts.Type)
		where += ` AND type=$` + strconv.Itoa(len(args))
	}
	switch opts.Status {
	case "enabled":
		where += ` AND enabled=TRUE`
	case "disabled":
		where += ` AND enabled=FALSE`
	case "expired":
		args = append(args, opts.Now)
		where += ` AND ends_at IS NOT NULL AND ends_at <= $` + strconv.Itoa(len(args))
	case "scheduled":
		args = append(args, opts.Now)
		where += ` AND starts_at IS NOT NULL AND starts_at > $` + strconv.Itoa(len(args))
	}
	where = addCursorWhere(where, &args, "", opts.LastPinned, opts.LastCreated, opts.LastID)
	args = append(args, actual)
	q := noticeSelectSQL() + ` WHERE ` + where + ` ORDER BY pinned DESC, created_at DESC, id DESC LIMIT $` + strconv.Itoa(len(args))
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []model.Notice{}
	for rows.Next() {
		item, err := scanNotice(rows)
		if err != nil {
			return nil, err
		}
		got = append(got, item)
	}
	return noticePage(got, opts.Limit), rows.Err()
}
