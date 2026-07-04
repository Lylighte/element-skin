package texture

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s Store) AddToLibrary(ctx context.Context, userID, hash, textureType, note string, isPublic bool, model string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockLibraryTexture(ctx, tx, hash, textureType); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	created := time.Now().UnixMilli()
	pub := 0
	if isPublic {
		pub = 1
	}
	tag, err := tx.Exec(ctx, `INSERT INTO user_textures (user_id,hash,texture_type,note,model,is_public,created_at) VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`,
		userID, hash, textureType, note, model, pub, created)
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO skin_library (skin_hash,texture_type,is_public,uploader,model,name,created_at,usage_count) VALUES ($1,$2,$3,$4,$5,$6,$7,1) ON CONFLICT DO NOTHING`,
		hash, textureType, pub, userID, model, note, created); err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		if _, err := tx.Exec(ctx, `UPDATE skin_library SET usage_count=(SELECT COUNT(*) FROM user_textures WHERE hash=$1 AND texture_type=$2) WHERE skin_hash=$1 AND texture_type=$2`, hash, textureType); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) DeleteFromLibrary(ctx context.Context, userID, hash, textureType string) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if err := lockLibraryTexture(ctx, tx, hash, textureType); err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	var one int
	err = tx.QueryRow(ctx, `SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE skin_library
		SET usage_count=(
			SELECT COUNT(*) FROM user_textures
			WHERE hash=$1 AND texture_type=$2
		)
		WHERE skin_hash=$1 AND texture_type=$2
	`, hash, textureType); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}
