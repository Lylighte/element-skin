package notice

import "context"

func (s Store) MarkRead(ctx context.Context, noticeID, userID string, now int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notice_receipts (notice_id,user_id,read_at,created_at)
		VALUES ($1,$2,$3,$3)
		ON CONFLICT (notice_id,user_id)
		DO UPDATE SET read_at=COALESCE(notice_receipts.read_at, EXCLUDED.read_at)
	`, noticeID, userID, now)
	return err
}

func (s Store) Dismiss(ctx context.Context, noticeID, userID string, now int64) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO notice_receipts (notice_id,user_id,dismissed_at,created_at)
		VALUES ($1,$2,$3,$3)
		ON CONFLICT (notice_id,user_id)
		DO UPDATE SET dismissed_at=COALESCE(notice_receipts.dismissed_at, EXCLUDED.dismissed_at)
	`, noticeID, userID, now)
	return err
}

func (s Store) DeleteExpired(ctx context.Context, cutoff int64) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM notices WHERE ends_at IS NOT NULL AND ends_at <= $1`, cutoff)
	return err
}
