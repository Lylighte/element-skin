package texture

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (s Store) UpdateNote(ctx context.Context, userID, hash, textureType, note string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE user_textures SET note=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4`, note, userID, hash, textureType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET name=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`, note, hash, userID, textureType); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) UpdateModel(ctx context.Context, userID, hash, textureType, model string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE user_textures SET model=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4`, model, userID, hash, textureType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET model=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`, model, hash, userID, textureType); err != nil {
		return err
	}
	if strings.EqualFold(textureType, "skin") {
		if _, err := tx.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE skin_hash=$2 AND user_id=$3`, model, hash, userID); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) UpdatePublic(ctx context.Context, userID, hash, textureType string, isPublic bool) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	pub := 0
	if isPublic {
		pub = 1
	}
	tag, err := tx.Exec(ctx, `UPDATE user_textures SET is_public=$1 WHERE user_id=$2 AND hash=$3 AND texture_type=$4 AND is_public != 2`, pub, userID, hash, textureType)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var one int
		err = tx.QueryRow(ctx, `SELECT 1 FROM user_textures WHERE user_id=$1 AND hash=$2 AND texture_type=$3`, userID, hash, textureType).Scan(&one)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE skin_library SET is_public=$1 WHERE skin_hash=$2 AND uploader=$3 AND texture_type=$4`, pub, hash, userID, textureType); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
