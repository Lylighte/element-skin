package permission

import (
	"context"
	"time"
)

func (s Store) SeedDefaults(ctx context.Context) error {
	tx, err := s.conn().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	now := time.Now().UnixMilli()
	if err := seedCatalog(ctx, tx, now); err != nil {
		return err
	}
	if err := seedRoles(ctx, tx, now); err != nil {
		return err
	}
	if err := seedSessionPolicies(ctx, tx, now); err != nil {
		return err
	}
	if err := seedUserSubjects(ctx, tx, now); err != nil {
		return err
	}
	if err := seedClientSubjects(ctx, tx, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
