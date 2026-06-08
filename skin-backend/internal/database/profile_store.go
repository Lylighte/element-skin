package database

import (
	"context"
	"errors"
	"strings"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
)

func NormalizeProfileModel(m string) string {
	if m == "slim" {
		return "slim"
	}
	return "default"
}

func ProfileSummary(p model.Profile) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "model": p.TextureModel, "skin_hash": p.SkinHash, "cape_hash": p.CapeHash}
}

func ProfileModelKey(item map[string]any) map[string]any {
	if v, ok := item["texture_model"]; ok {
		item["model"] = v
	}
	return item
}

func IsProfileNameConflict(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}

func (db *DB) CreateProfile(ctx context.Context, p model.Profile) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash)
	return err
}

func (db *DB) GetProfileByID(ctx context.Context, id string) (*model.Profile, error) {
	p, err := scanProfile(db.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE id=$1`, id))
	if IsNoRows(err) {
		return nil, nil
	}
	return &p, err
}

func (db *DB) GetProfileByName(ctx context.Context, name string) (*model.Profile, error) {
	p, err := scanProfile(db.Pool.QueryRow(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name=$1`, name))
	if IsNoRows(err) {
		return nil, nil
	}
	return &p, err
}

func (db *DB) GetProfilesByUser(ctx context.Context, userID string, limit int) ([]model.Profile, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE user_id=$1 ORDER BY id LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (db *DB) VerifyProfileOwnership(ctx context.Context, userID, profileID string) (bool, error) {
	var one int
	err := db.Pool.QueryRow(ctx, `SELECT 1 FROM profiles WHERE id=$1 AND user_id=$2`, profileID, userID).Scan(&one)
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) CountProfilesByUser(ctx context.Context, userID string) (int, error) {
	var n int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE user_id=$1`, userID).Scan(&n)
	return n, err
}

func (db *DB) UpdateProfileName(ctx context.Context, id, name string) (bool, error) {
	tag, err := db.Pool.Exec(ctx, `UPDATE profiles SET name=$1 WHERE id=$2`, name, id)
	return tag.RowsAffected() > 0, err
}

func (db *DB) UpdateProfileSkin(ctx context.Context, id string, hash *string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET skin_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdateProfileCape(ctx context.Context, id string, hash *string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET cape_hash=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdateProfileModel(ctx context.Context, id, model string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE profiles SET texture_model=$1 WHERE id=$2`, model, id)
	return err
}

func (db *DB) DeleteProfileCascade(ctx context.Context, id string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM tokens WHERE profile_id=$1`, id); err != nil {
		return false, err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM profiles WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (db *DB) SearchProfilesByNames(ctx context.Context, names []string, limit int) ([]model.Profile, error) {
	rows, err := db.Pool.Query(ctx, `SELECT id,user_id,name,texture_model,skin_hash,cape_hash FROM profiles WHERE name = ANY($1) LIMIT $2`, names, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Profile
	for rows.Next() {
		var p model.Profile
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TextureModel, &p.SkinHash, &p.CapeHash); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
