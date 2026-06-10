package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/testutil"
)

func TestAppNewWithDBUsesRealRedisAndCloseReleasesDatabase(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()

	application, err := app.NewWithDB(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("real-redis app handler status=%d body=%q", rec.Code, rec.Body.String())
	}

	application.Close()
	if err := db.Pool.Ping(context.Background()); err == nil {
		t.Fatal("App.Close should release the database pool")
	}

	// Close is intentionally idempotent so shutdown paths can safely defer it.
	application.Close()
}
