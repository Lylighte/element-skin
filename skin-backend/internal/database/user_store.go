package database

import (
	"context"

	"element-skin/backend/internal/model"
)

func (db *DB) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE email=$1`, email))
	if IsNoRows(err) {
		return nil, nil
	}
	return &u, err
}

func (db *DB) GetUserByID(ctx context.Context, id string) (*model.User, error) {
	u, err := scanUser(db.Pool.QueryRow(ctx, `SELECT id,email,password,is_admin,preferred_language,display_name,banned_until,avatar_hash FROM users WHERE id=$1`, id))
	if IsNoRows(err) {
		return nil, nil
	}
	return &u, err
}

func (db *DB) CreateUser(ctx context.Context, u model.User) error {
	_, err := db.Pool.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash)
	return err
}

func (db *DB) CreateUserWithProfile(ctx context.Context, u model.User, p model.Profile, inviteCode, usedBy string) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `INSERT INTO users (id,email,password,is_admin,display_name,avatar_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		u.ID, u.Email, u.Password, u.IsAdmin, u.DisplayName, u.AvatarHash); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO profiles (id,user_id,name,texture_model,skin_hash,cape_hash) VALUES ($1,$2,$3,$4,$5,$6)`,
		p.ID, p.UserID, p.Name, p.TextureModel, p.SkinHash, p.CapeHash); err != nil {
		return err
	}
	if inviteCode != "" {
		tag, err := tx.Exec(ctx, `UPDATE invites SET used_count=used_count+1 WHERE code=$1 AND (total_uses IS NULL OR used_count < total_uses)`, inviteCode)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrInviteExhausted
		}
		if usedBy != "" {
			if _, err := tx.Exec(ctx, `UPDATE invites SET used_by=$1 WHERE code=$2 AND used_by IS NULL`, usedBy, inviteCode); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}

func (db *DB) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (db *DB) IsDisplayNameTaken(ctx context.Context, name string, exclude string) (bool, error) {
	var one int
	var err error
	if exclude != "" {
		err = db.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1 AND id<>$2`, name, exclude).Scan(&one)
	} else {
		err = db.Pool.QueryRow(ctx, `SELECT 1 FROM users WHERE display_name=$1`, name).Scan(&one)
	}
	if IsNoRows(err) {
		return false, nil
	}
	return err == nil, err
}

func (db *DB) UpdateUser(ctx context.Context, id string, fields map[string]any) error {
	for k, v := range fields {
		switch k {
		case "email":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET email=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "display_name":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET display_name=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "preferred_language":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET preferred_language=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "avatar_hash":
			_, err := db.Pool.Exec(ctx, `UPDATE users SET avatar_hash=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (db *DB) UpdatePassword(ctx context.Context, id, hash string) error {
	_, err := db.Pool.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	return err
}

func (db *DB) UpdatePasswordAndRevokeRefresh(ctx context.Context, id, hash string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE users SET password=$1 WHERE id=$2`, hash, id)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `DELETE FROM site_refresh_tokens WHERE user_id=$1`, id); err != nil {
		return false, err
	}
	return true, tx.Commit(ctx)
}

func (db *DB) DeleteUser(ctx context.Context, id string) (bool, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)
	for _, q := range []string{
		`DELETE FROM profiles WHERE user_id=$1`,
		`DELETE FROM tokens WHERE user_id=$1`,
		`DELETE FROM site_refresh_tokens WHERE user_id=$1`,
		`DELETE FROM user_textures WHERE user_id=$1`,
	} {
		if _, err := tx.Exec(ctx, q, id); err != nil {
			return false, err
		}
	}
	tag, err := tx.Exec(ctx, `DELETE FROM users WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, tx.Commit(ctx)
}

func (db *DB) IsBanned(ctx context.Context, id string) (bool, error) {
	var until *int64
	err := db.Pool.QueryRow(ctx, `SELECT banned_until FROM users WHERE id=$1`, id).Scan(&until)
	if IsNoRows(err) || until == nil {
		return false, nil
	}
	return *until > NowMS(), err
}
