package user

import (
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func IsEmailConflict(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == "users_email_key"
}

func PublicUser(u model.User) map[string]any {
	return map[string]any{
		"id":                 u.ID,
		"email":              u.Email,
		"display_name":       u.DisplayName,
		"banned_until":       u.BannedUntil,
		"preferred_language": u.PreferredLanguage,
		"avatar_hash":        u.AvatarHash,
		"created_at":         u.CreatedAt,
	}
}

const userSelectSQL = `SELECT id,email,password,preferred_language,display_name,created_at,banned_until,avatar_hash FROM users`

func scan(row pgx.Row) (model.User, error) {
	var u model.User
	err := row.Scan(&u.ID, &u.Email, &u.Password, &u.PreferredLanguage, &u.DisplayName, &u.CreatedAt, &u.BannedUntil, &u.AvatarHash)
	return u, err
}
