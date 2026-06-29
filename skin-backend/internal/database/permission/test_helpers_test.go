package permission_test

import (
	"context"
	"errors"
	"testing"

	core "element-skin/backend/internal/permission"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func assertCancelled(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %T: %v", err, err)
	}
}

func assertPostgresError(t *testing.T, err error, code string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("expected PostgreSQL error, got %T: %v", err, err)
	}
	if pgErr.Code != code {
		t.Fatalf("expected SQLSTATE %s, got %s: %s", code, pgErr.Code, pgErr.Message)
	}
}

func assertPgErrorOrClosed(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	var scanErr pgx.ScanArgError
	if errors.As(err, &scanErr) {
		return
	}
	if err.Error() == "closed pool" {
		return
	}
	t.Fatalf("unexpected error type %T: %v", err, err)
}

func has(bits core.BitSet, code string) bool {
	return bits.Has(core.MustDefinitionByCode(code).BitIndex)
}
