package user

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (s Store) Update(ctx context.Context, id string, fields map[string]any) error {
	attempted := false
	for _, key := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		if _, ok := fields[key]; ok {
			attempted = true
			break
		}
	}
	if !attempted {
		return nil
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var one int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&one); err != nil {
		return err
	}
	if displayName, ok := fields["display_name"].(string); ok {
		if err := lockDisplayName(ctx, tx, displayName, id); err != nil {
			return err
		}
	}
	updated := false
	for _, k := range []string{"email", "display_name", "preferred_language", "avatar_hash"} {
		v, ok := fields[k]
		if !ok {
			continue
		}
		var tag pgconn.CommandTag
		switch k {
		case "email":
			tag, err = tx.Exec(ctx, `UPDATE users SET email=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "display_name":
			tag, err = tx.Exec(ctx, `UPDATE users SET display_name=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "preferred_language":
			tag, err = tx.Exec(ctx, `UPDATE users SET preferred_language=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		case "avatar_hash":
			tag, err = tx.Exec(ctx, `UPDATE users SET avatar_hash=$1 WHERE id=$2`, v, id)
			if err != nil {
				return err
			}
		}
		updated = updated || tag.RowsAffected() > 0
	}
	if !updated {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}
