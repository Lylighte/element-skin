package fallback_test

import (
	"testing"

	"element-skin/backend/internal/service/fallback"
	"element-skin/backend/internal/testutil"
)

func TestFallbackStoresDependencies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	fb := fallback.Fallback{DB: db}
	if fb.DB != db {
		t.Fatal("Fallback should retain DB dependency")
	}
}
