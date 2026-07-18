package easteregg

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool *pgxpool.Pool
}

func (s Store) ListEnabled(ctx context.Context) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id FROM enabled_easter_eggs ORDER BY sort_order,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s Store) ReplaceEnabled(ctx context.Context, ids []string) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `DELETE FROM enabled_easter_eggs`); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for index, id := range ids {
		if _, err := tx.Exec(ctx, `
			INSERT INTO enabled_easter_eggs (id,sort_order,enabled_at)
			VALUES ($1,$2,$3)
		`, id, index+1, now); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
