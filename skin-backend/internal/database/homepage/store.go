package homepage

import (
	"context"

	"element-skin/backend/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) Create(ctx context.Context, item model.HomepageMedia) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO homepage_media
			(id, media_type, title, storage_path, overlay_opacity_light, overlay_opacity_dark, start_yaw, start_pitch, yaw_speed_dps, pitch_speed_dps, sort_order, enabled, duration_ms, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
	`, item.ID, item.Type, item.Title, item.StoragePath, item.OverlayOpacityLight, item.OverlayOpacityDark, item.StartYaw, item.StartPitch, item.YawSpeedDPS, item.PitchSpeedDPS, item.SortOrder, item.Enabled, item.DurationMS, item.CreatedAt, item.UpdatedAt)
	return err
}

func (s Store) NextSortOrder(ctx context.Context) (int, error) {
	var next int
	err := s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(sort_order), -1) + 1 FROM homepage_media`).Scan(&next)
	return next, err
}

func (s Store) List(ctx context.Context, onlyEnabled bool) ([]model.HomepageMedia, error) {
	sql := `SELECT id, media_type, title, storage_path, overlay_opacity_light, overlay_opacity_dark, start_yaw, start_pitch, yaw_speed_dps, pitch_speed_dps, sort_order, enabled, duration_ms, created_at, updated_at FROM homepage_media`
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
	row := s.Pool.QueryRow(ctx, `SELECT id, media_type, title, storage_path, overlay_opacity_light, overlay_opacity_dark, start_yaw, start_pitch, yaw_speed_dps, pitch_speed_dps, sort_order, enabled, duration_ms, created_at, updated_at FROM homepage_media WHERE id=$1`, id)
	return scan(row)
}

type Patch struct {
	Title               *string
	OverlayOpacityLight *float64
	OverlayOpacityDark  *float64
	StartYaw            *float64
	StartPitch          *float64
	YawSpeedDPS         *float64
	PitchSpeedDPS       *float64
	Enabled             *bool
	DurationMS          *int
	UpdatedAt           int64
}

func (s Store) Patch(ctx context.Context, id string, patch Patch) (model.HomepageMedia, error) {
	row := s.Pool.QueryRow(ctx, `
		UPDATE homepage_media SET
			title = CASE WHEN $2 THEN $3 ELSE title END,
			overlay_opacity_light = CASE WHEN $4 THEN $5 ELSE overlay_opacity_light END,
			overlay_opacity_dark = CASE WHEN $6 THEN $7 ELSE overlay_opacity_dark END,
			start_yaw = CASE WHEN $8 THEN $9 ELSE start_yaw END,
			start_pitch = CASE WHEN $10 THEN $11 ELSE start_pitch END,
			yaw_speed_dps = CASE WHEN $12 THEN $13 ELSE yaw_speed_dps END,
			pitch_speed_dps = CASE WHEN $14 THEN $15 ELSE pitch_speed_dps END,
			enabled = CASE WHEN $16 THEN $17 ELSE enabled END,
			duration_ms = CASE WHEN $18 THEN $19 ELSE duration_ms END,
			updated_at = $20
		WHERE id=$1
		RETURNING id, media_type, title, storage_path, overlay_opacity_light, overlay_opacity_dark, start_yaw, start_pitch, yaw_speed_dps, pitch_speed_dps, sort_order, enabled, duration_ms, created_at, updated_at
	`, id,
		patch.Title != nil, derefString(patch.Title),
		patch.OverlayOpacityLight != nil, derefFloat64(patch.OverlayOpacityLight),
		patch.OverlayOpacityDark != nil, derefFloat64(patch.OverlayOpacityDark),
		patch.StartYaw != nil, derefFloat64(patch.StartYaw),
		patch.StartPitch != nil, derefFloat64(patch.StartPitch),
		patch.YawSpeedDPS != nil, derefFloat64(patch.YawSpeedDPS),
		patch.PitchSpeedDPS != nil, derefFloat64(patch.PitchSpeedDPS),
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
		RETURNING id, media_type, title, storage_path, overlay_opacity_light, overlay_opacity_dark, start_yaw, start_pitch, yaw_speed_dps, pitch_speed_dps, sort_order, enabled, duration_ms, created_at, updated_at
	`, id)
	return scan(row)
}

type scanner interface {
	Scan(dest ...any) error
}

func scan(row scanner) (model.HomepageMedia, error) {
	var item model.HomepageMedia
	if err := row.Scan(
		&item.ID,
		&item.Type,
		&item.Title,
		&item.StoragePath,
		&item.OverlayOpacityLight,
		&item.OverlayOpacityDark,
		&item.StartYaw,
		&item.StartPitch,
		&item.YawSpeedDPS,
		&item.PitchSpeedDPS,
		&item.SortOrder,
		&item.Enabled,
		&item.DurationMS,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return model.HomepageMedia{}, err
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

func derefFloat64(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}
