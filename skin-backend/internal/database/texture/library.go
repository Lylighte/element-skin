package texture

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"element-skin/backend/internal/util"

	"github.com/jackc/pgx/v5"
)

type PublicLibrarySort string

const (
	PublicLibrarySortLatest   PublicLibrarySort = "latest"
	PublicLibrarySortMostUsed PublicLibrarySort = "most_used"
)

type PublicListOptions struct {
	Limit       int
	TextureType string
	Query       string
	Sort        PublicLibrarySort
	LastCreated *int64
	LastHash    string
	LastUsage   *int64
}

func ParsePublicLibrarySort(value string) PublicLibrarySort {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case string(PublicLibrarySortMostUsed):
		return PublicLibrarySortMostUsed
	default:
		return PublicLibrarySortLatest
	}
}

func (s Store) ListPublic(ctx context.Context, opts PublicListOptions) (map[string]any, error) {
	actual := opts.Limit + 1
	args := []any{}
	where := `sl.is_public = 1`
	idx := 1
	if opts.TextureType != "" {
		where += ` AND sl.texture_type=$` + strconv.Itoa(idx)
		args = append(args, opts.TextureType)
		idx++
	}
	if opts.Query != "" {
		where += ` AND (sl.skin_hash ILIKE $` + strconv.Itoa(idx) + ` OR sl.name ILIKE $` + strconv.Itoa(idx) + ` OR u.display_name ILIKE $` + strconv.Itoa(idx) + `)`
		args = append(args, "%"+opts.Query+"%")
		idx++
	}
	orderBy := `created_at DESC, skin_hash DESC`
	switch opts.Sort {
	case PublicLibrarySortMostUsed:
		orderBy = `usage_count DESC, created_at DESC, skin_hash DESC`
		if opts.LastUsage != nil && opts.LastCreated != nil && opts.LastHash != "" {
			where += ` AND (sl.usage_count < $` + strconv.Itoa(idx) + ` OR (sl.usage_count = $` + strconv.Itoa(idx) + ` AND (sl.created_at < $` + strconv.Itoa(idx+1) + ` OR (sl.created_at = $` + strconv.Itoa(idx+1) + ` AND sl.skin_hash < $` + strconv.Itoa(idx+2) + `))))`
			args = append(args, *opts.LastUsage, *opts.LastCreated, opts.LastHash)
			idx += 3
		}
	default:
		if opts.LastCreated != nil && opts.LastHash != "" {
			where += ` AND (sl.created_at < $` + strconv.Itoa(idx) + ` OR (sl.created_at = $` + strconv.Itoa(idx) + ` AND sl.skin_hash < $` + strconv.Itoa(idx+1) + `))`
			args = append(args, *opts.LastCreated, opts.LastHash)
			idx += 2
		}
	}
	q := `SELECT sl.skin_hash,sl.texture_type,sl.is_public,sl.uploader,sl.created_at,sl.model,sl.name,COALESCE(u.display_name,''),sl.usage_count FROM skin_library sl LEFT JOIN users u ON sl.uploader=u.id WHERE ` + where + ` ORDER BY ` + orderBy + ` LIMIT $` + strconv.Itoa(idx)
	args = append(args, actual)
	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, t, uploader, model, name, display string
		var pub int
		var created, usage int64
		if err := rows.Scan(&h, &t, &pub, &uploader, &created, &model, &name, &display, &usage); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": t, "is_public": pub == 1, "uploader": uploader, "created_at": created, "model": model, "name": name, "uploader_display_name": display, "uploader_name": display, "usage_count": usage})
	}
	hasNext := len(got) > opts.Limit
	items := got
	if hasNext {
		items = got[:opts.Limit]
	}
	var next map[string]any
	if hasNext {
		last := got[opts.Limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_skin_hash": last["hash"]}
		if opts.Sort == PublicLibrarySortMostUsed {
			next["last_usage_count"] = last["usage_count"]
		}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_cursor": util.EncodeCursor(next), "page_size": len(items)}, rows.Err()
}

func (s Store) AddToWardrobe(ctx context.Context, userID, hash, textureType string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	var selectedType, model, uploader, name string
	var pub int
	args := []any{hash}
	where := `skin_hash=$1`
	if textureType != "" {
		where += ` AND texture_type=$2`
		args = append(args, textureType)
	}
	err = tx.QueryRow(ctx, `SELECT texture_type,model,uploader,name,is_public FROM skin_library WHERE `+where+` ORDER BY CASE WHEN texture_type='skin' THEN 0 ELSE 1 END LIMIT 1`, args...).Scan(&selectedType, &model, &uploader, &name, &pub)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if pub != 1 {
		return false, nil
	}
	tag, err := tx.Exec(ctx, `INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, userID, hash, selectedType, name, model, 2, time.Now().UnixMilli())
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE skin_library SET usage_count=usage_count+1 WHERE skin_hash=$1 AND texture_type=$2`, hash, selectedType); err != nil {
			return false, err
		}
	}
	return true, tx.Commit(ctx)
}

func (s Store) RecountUsage(ctx context.Context, hash, textureType string) error {
	textureType = strings.ToLower(textureType)
	if textureType != "skin" && textureType != "cape" {
		return errors.New("invalid texture_type")
	}
	_, err := s.Pool.Exec(ctx, `UPDATE skin_library SET usage_count=(SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2) WHERE skin_hash=$1 AND texture_type=$2`, hash, textureType)
	return err
}
