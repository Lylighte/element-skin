package integration_test

import (
	"context"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/testutil"
)

func TestDatabaseInitScripts(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	for _, table := range []string{"users", "settings", "fallback_endpoints"} {
		var got string
		if err := db.Pool.QueryRow(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema='public' AND table_name=$1", table).Scan(&got); err != nil {
			t.Fatalf("expected table %s: %v", table, err)
		}
	}
	v, err := db.Settings.Get(ctx, "enable_skin_library", "")
	if err != nil {
		t.Fatal(err)
	}
	if v != "true" {
		t.Fatalf("enable_skin_library=%q", v)
	}
}

func TestYggdrasilTokenStructStillUsable(t *testing.T) {
	_ = model.Token{AccessToken: "a", ClientToken: "c", UserID: "u", CreatedAt: time.Now().UnixMilli()}
}
