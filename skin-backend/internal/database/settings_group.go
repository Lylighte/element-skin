package database

import (
	"context"
	"fmt"

	"element-skin/backend/internal/database/fallback"
)

type SettingUpdate struct {
	Key   string
	Value any
}

func (db *DB) SaveSettingsGroup(ctx context.Context, updates []SettingUpdate, endpoints []fallback.Endpoint, replaceEndpoints bool) error {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, update := range updates {
		value := fmt.Sprint(update.Value)
		if boolean, ok := update.Value.(bool); ok {
			value = fmt.Sprintf("%t", boolean)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO settings (key,value) VALUES ($1,$2)
			 ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`,
			update.Key, value,
		); err != nil {
			return err
		}
	}
	if replaceEndpoints {
		if err := fallback.ReplaceEndpoints(ctx, tx, endpoints); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
