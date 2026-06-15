package homepage

import (
	"context"
	"encoding/json"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Create(ctx context.Context, item model.HomepageMedia) error {
	cfg, err := json.Marshal(item.Config)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, `
		INSERT INTO homepage_media
			(id, media_type, title, storage_path, config, sort_order, enabled, duration_ms, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, item.ID, item.Type, item.Title, item.StoragePath, cfg, item.SortOrder, item.Enabled, item.DurationMS, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s Store) NextSortOrder(ctx context.Context) (int, error) {
	var next int
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM homepage_media`).Scan(&next)
	return next, err
}

func (s Store) List(ctx context.Context, onlyEnabled bool) ([]model.HomepageMedia, error) {
	sql := `SELECT id, media_type, title, storage_path, config, sort_order, enabled, duration_ms, created_at, updated_at FROM homepage_media`
	if onlyEnabled {
		sql += ` WHERE enabled = TRUE`
	}
	sql += ` ORDER BY sort_order ASC, id ASC`
	rows, err := s.Pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.HomepageMedia{}
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s Store) Get(ctx context.Context, id string) (model.HomepageMedia, error) {
	row := s.Pool.QueryRow(ctx, `SELECT id, media_type, title, storage_path, config, sort_order, enabled, duration_ms, created_at, updated_at FROM homepage_media WHERE id=$1`, id)
	return scan(row)
}

type Patch struct {
	Title      *string
	Config     map[string]any
	HasConfig  bool
	Enabled    *bool
	DurationMS *int
	UpdatedAt  int64
}

func (s Store) Patch(ctx context.Context, id string, patch Patch) (model.HomepageMedia, error) {
	var cfg []byte
	var err error
	if patch.HasConfig {
		cfg, err = json.Marshal(patch.Config)
		if err != nil {
			return model.HomepageMedia{}, err
		}
	}
	row := s.Pool.QueryRow(ctx, `
		UPDATE homepage_media SET
			title = CASE WHEN $2 THEN $3 ELSE title END,
			config = CASE WHEN $4 THEN $5 ELSE config END,
			enabled = CASE WHEN $6 THEN $7 ELSE enabled END,
			duration_ms = CASE WHEN $8 THEN $9 ELSE duration_ms END,
			updated_at = $10
		WHERE id=$1
		RETURNING id, media_type, title, storage_path, config, sort_order, enabled, duration_ms, created_at, updated_at
	`, id,
		patch.Title != nil, derefString(patch.Title),
		patch.HasConfig, cfg,
		patch.Enabled != nil, derefBool(patch.Enabled),
		patch.DurationMS != nil, derefInt(patch.DurationMS),
		patch.UpdatedAt,
	)
	return scan(row)
}

func (s Store) Reorder(ctx context.Context, ids []string, updatedAt int64) error {
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for i, id := range ids {
		tag, err := tx.Exec(ctx, `UPDATE homepage_media SET sort_order=$1, updated_at=$2 WHERE id=$3`, i, updatedAt, id)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return pgx.ErrNoRows
		}
	}
	return tx.Commit(ctx)
}

func (s Store) Delete(ctx context.Context, id string) (model.HomepageMedia, error) {
	row := s.Pool.QueryRow(ctx, `
		DELETE FROM homepage_media WHERE id=$1
		RETURNING id, media_type, title, storage_path, config, sort_order, enabled, duration_ms, created_at, updated_at
	`, id)
	return scan(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (model.HomepageMedia, error) {
	var item model.HomepageMedia
	var raw []byte
	if err := row.Scan(&item.ID, &item.Type, &item.Title, &item.StoragePath, &raw, &item.SortOrder, &item.Enabled, &item.DurationMS, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return model.HomepageMedia{}, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &item.Config); err != nil {
			return model.HomepageMedia{}, err
		}
	}
	if item.Config == nil {
		item.Config = map[string]any{}
	}
	return item, nil
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func derefBool(v *bool) bool {
	return v != nil && *v
}

func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
