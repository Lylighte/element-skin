package texture

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func (s Store) LibraryUploader(ctx context.Context, hash, textureType string) (string, bool, error) {
	var uploader string
	err := s.Pool.QueryRow(ctx, `SELECT uploader FROM skin_library WHERE skin_hash=$1 AND texture_type=$2`, hash, textureType).Scan(&uploader)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return uploader, true, nil
}

func (s Store) DeleteLibraryTexture(ctx context.Context, hash, textureType string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := lockLibraryTexture(ctx, tx, hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_textures WHERE hash=$1 AND texture_type=$2`, hash, textureType); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM skin_library WHERE skin_hash=$1 AND texture_type=$2`, hash, textureType); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
