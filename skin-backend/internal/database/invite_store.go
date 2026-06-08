package database

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

var ErrInviteExhausted = errString("invite exhausted")

func (db *DB) CreateInvite(ctx context.Context, code string, totalUses int, note string) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO invites (code,created_at,total_uses,used_count,note) VALUES ($1,$2,$3,0,$4)`, code, NowMS(), totalUses, note)
	return err
}

func (db *DB) GetInvite(ctx context.Context, code string) (*model.Invite, error) {
	var inv model.Invite
	err := db.Pool.QueryRow(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites WHERE code=$1`, code).
		Scan(&inv.Code, &inv.CreatedAt, &inv.UsedBy, &inv.TotalUses, &inv.UsedCount, &inv.Note)
	if IsNoRows(err) {
		return nil, nil
	}
	return &inv, err
}

func (db *DB) DeleteInvite(ctx context.Context, code string) error {
	_, err := db.Pool.Exec(ctx, `DELETE FROM invites WHERE code=$1`, code)
	return err
}

func (db *DB) ListInvites(ctx context.Context, limit int, lastCreated *int64, lastCode string) (map[string]any, error) {
	actual := limit + 1
	var rows pgx.Rows
	var err error
	if lastCreated != nil && lastCode != "" {
		rows, err = db.Pool.Query(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites WHERE (created_at < $1) OR (created_at=$1 AND code < $2) ORDER BY created_at DESC, code DESC LIMIT $3`, *lastCreated, lastCode, actual)
	} else {
		rows, err = db.Pool.Query(ctx, `SELECT code,created_at,used_by,total_uses,used_count,note FROM invites ORDER BY created_at DESC, code DESC LIMIT $1`, actual)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var got []model.Invite
	for rows.Next() {
		var inv model.Invite
		if err := rows.Scan(&inv.Code, &inv.CreatedAt, &inv.UsedBy, &inv.TotalUses, &inv.UsedCount, &inv.Note); err != nil {
			return nil, err
		}
		got = append(got, inv)
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, inv := range items {
		out = append(out, map[string]any{"code": inv.Code, "created_at": inv.CreatedAt, "used_by": inv.UsedBy, "total_uses": inv.TotalUses, "used_count": inv.UsedCount, "note": inv.Note})
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": *last.CreatedAt, "last_code": last.Code}
	}
	return map[string]any{"items": out, "has_next": hasNext, "next_key": next, "page_size": len(out)}, rows.Err()
}
