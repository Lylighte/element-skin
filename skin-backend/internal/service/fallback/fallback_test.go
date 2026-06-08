package fallback_test

import (
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestFallbackStoresDependencies(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	fb := newFallback(db, nil)
	if fb.DB != db {
		t.Fatal("Fallback should retain DB dependency")
	}
}
