package database

import (
	"context"
	"errors"
	"strconv"
)

func (db *DB) ListAllTextures(ctx context.Context, limit int, lastCreated *int64, lastHash, query, typeFilter string) (map[string]any, error) {
	actual := limit + 1
	args := []any{}
	where := "TRUE"
	idx := 1
	if typeFilter != "" {
		where += ` AND sl.texture_type=$` + strconv.Itoa(idx)
		args = append(args, typeFilter)
		idx++
	}
	if query != "" {
		where += ` AND (sl.skin_hash ILIKE $` + strconv.Itoa(idx) + ` OR sl.name ILIKE $` + strconv.Itoa(idx) + ` OR u.display_name ILIKE $` + strconv.Itoa(idx) + `)`
		args = append(args, "%"+query+"%")
		idx++
	}
	if lastCreated != nil && lastHash != "" {
		where += ` AND (sl.created_at < $` + strconv.Itoa(idx) + ` OR (sl.created_at = $` + strconv.Itoa(idx) + ` AND sl.skin_hash < $` + strconv.Itoa(idx+1) + `))`
		args = append(args, *lastCreated, lastHash)
		idx += 2
	}
	q := `SELECT sl.skin_hash,sl.texture_type,sl.is_public,sl.uploader,sl.created_at,sl.model,sl.name,COALESCE(u.email,''),COALESCE(u.display_name,'') FROM skin_library sl LEFT JOIN users u ON sl.uploader=u.id WHERE ` + where + ` ORDER BY sl.created_at DESC, sl.skin_hash DESC LIMIT $` + strconv.Itoa(idx)
	args = append(args, actual)
	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	got := []map[string]any{}
	for rows.Next() {
		var h, typ, uploader, model, name, email, display string
		var pub int
		var created int64
		if err := rows.Scan(&h, &typ, &pub, &uploader, &created, &model, &name, &email, &display); err != nil {
			return nil, err
		}
		got = append(got, map[string]any{"hash": h, "type": typ, "is_public": pub == 1, "uploader_user_id": uploader, "created_at": created, "model": model, "name": name, "uploader_email": email, "uploader_display_name": display})
	}
	hasNext := len(got) > limit
	items := got
	if hasNext {
		items = got[:limit]
	}
	var next map[string]any
	if hasNext {
		last := got[limit-1]
		next = map[string]any{"last_created_at": last["created_at"], "last_skin_hash": last["hash"]}
	}
	return map[string]any{"items": items, "has_next": hasNext, "next_key": next, "page_size": len(items)}, rows.Err()
}

func (db *DB) AdminUpdateTexturePublic(ctx context.Context, hash string, isPublic bool) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	pub := 0
	if isPublic {
		pub = 1
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2`, pub, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET is_public=$1 WHERE hash=$2 AND is_public != 2`, pub, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) AdminUpdateTextureNote(ctx context.Context, hash, note string) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET name=$1 WHERE skin_hash=$2`, note, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET note=$1 WHERE hash=$2`, note, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) AdminUpdateTextureModel(ctx context.Context, hash, model string) error {
	exists, err := db.TextureExists(ctx, hash)
	if err != nil {
		return err
	}
	if !exists {
		return ErrNotFound
	}
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET model=$1 WHERE skin_hash=$2`, model, hash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE user_textures SET model=$1 WHERE hash=$2`, model, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (db *DB) TextureExists(ctx context.Context, hash string) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM skin_library WHERE skin_hash=$1`, hash).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) AdminDeleteTexture(ctx context.Context, hash, textureType, userID string, force bool) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if force {
		if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type=$2`, hash, textureType); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1`, hash); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if userID == "" {
		return errors.New("per-user deletion requires user_id")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType); err != nil {
		return err
	}
	var remaining int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2`, hash, textureType).Scan(&remaining); err != nil {
		return err
	}
	if remaining == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1`, hash); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
