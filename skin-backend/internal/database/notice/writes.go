package notice

import (
	"context"
	"errors"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
)

func (s Store) Create(ctx context.Context, n model.Notice) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notices (
			id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.CreatedBy, n.CreatedAt, n.UpdatedAt)
	return err
}

func (s Store) CreateWithTargets(ctx context.Context, n model.Notice, targetUserIDs []string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO notices (
			id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.CreatedBy, n.CreatedAt, n.UpdatedAt); err != nil {
		return err
	}
	for _, userID := range targetUserIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO notice_targets (notice_id,user_id,created_at)
			VALUES ($1,$2,$3)
		`, n.ID, userID, n.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s Store) Update(ctx context.Context, n model.Notice) (*model.Notice, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE notices
		SET type=$2,title=$3,summary=$4,content_markdown=$5,display_mode=$6,level=$7,
			link_text=$8,link_url=$9,audience=$10,enabled=$11,pinned=$12,dismissible=$13,
			starts_at=$14,ends_at=$15,updated_at=$16
		WHERE id=$1
		RETURNING id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.UpdatedAt)
	updated, err := scanNotice(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &updated, nil
}

func (s Store) Replace(ctx context.Context, oldID string, n model.Notice) (bool, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx)

	targetUserIDs := []string{}
	if n.Audience == "targeted" {
		rows, err := tx.Query(ctx, `SELECT user_id FROM notice_targets WHERE notice_id=$1 ORDER BY user_id`, oldID)
		if err != nil {
			return false, err
		}
		for rows.Next() {
			var userID string
			if err := rows.Scan(&userID); err != nil {
				rows.Close()
				return false, err
			}
			targetUserIDs = append(targetUserIDs, userID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return false, err
		}
		rows.Close()
	}
	tag, err := tx.Exec(ctx, `DELETE FROM notices WHERE id=$1`, oldID)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO notices (
			id,type,title,summary,content_markdown,display_mode,level,link_text,link_url,
			audience,enabled,pinned,dismissible,starts_at,ends_at,created_by,created_at,updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
	`, n.ID, n.Type, n.Title, n.Summary, n.ContentMarkdown, n.DisplayMode, n.Level, n.LinkText, n.LinkURL,
		n.Audience, n.Enabled, n.Pinned, n.Dismissible, n.StartsAt, n.EndsAt, n.CreatedBy, n.CreatedAt, n.UpdatedAt); err != nil {
		return false, err
	}
	for _, userID := range targetUserIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO notice_targets (notice_id,user_id,created_at)
			VALUES ($1,$2,$3)
		`, n.ID, userID, n.CreatedAt); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s Store) Delete(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM notices WHERE id=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
