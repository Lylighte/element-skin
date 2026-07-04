package texture

import (
	"context"
	"errors"
	"strconv"

	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

func (s Store) CountForUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := s.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM user_textures WHERE user_id=$1`, userID).Scan(&n)
	return n, err
}

func (s Store) VerifyOwnership(ctx context.Context, userID, hash, textureType string) (bool, error) {
	var one int
	err := s.Pool.QueryRow(ctx, `SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (s Store) GetInfo(ctx context.Context, userID, hash, textureType string) (map[string]any, error) {
	var h, t, note, model string
	var created int64
	var pub int
	err := s.Pool.QueryRow(ctx, `SELECT hash,texture_type,note,model,created_at,is_public FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).
		Scan(&h, &t, &note, &model, &created, &pub)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return map[string]any{"hash": h, "type": t, "note": note, "model": model, "created_at": created, "is_public": pub}, nil
}

func (s Store) ListForUser(ctx context.Context, userID, textureType string, limit int, lastCreated *int64, lastHash string) (map[string]any, error) {
	actual := limit + 1
	args := []any{userID}
	where := `user_id=$1`
	idx := 2
	if textureType != "" {
		where += ` AND texture_type=$2`
		args = append(args, textureType)
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (created_at < $` + strconv.Itoa(idx) + ` OR (created_at = $` + strconv.Itoa(idx) + ` AND hash < $` + strconv.Itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT hash,texture_type,note,created_at,model,is_public FROM user_textures WHERE ` + where + ` ORDER BY created_at DESC, hash DESC LIMIT $` + strconv.Itoa(idx)
	args = append(args, actual)
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, t, note, model string
		var created int64
		var pub int
		if err := rows.Scan(&h, &t, &note, &created, &model, &pub); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": t, "note": note, "created_at": created, "model": model, "is_public": pub})
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_hash": last["hash"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "next_cursor": util.EncodeCursor(next), "page_size": len(items)}, rows.Err()
}
