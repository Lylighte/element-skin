package fallback_test

import (
	"errors"
	"testing"

	"element-skin/backend/internal/database/fallback"

	"github.com/jackc/pgx/v5"
)

func TestIsNoRowsDetectsPgxNoRowsOnly(t *testing.T) {
	if !fallback.IsNoRows(pgx.ErrNoRows) {
		t.Fatal("pgx.ErrNoRows should be detected")
	}
	if fallback.IsNoRows(errors.New("other")) {
		t.Fatal("unrelated errors should not be detected")
	}
}
