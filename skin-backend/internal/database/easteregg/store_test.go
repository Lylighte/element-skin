package easteregg_test

import (
	"context"
	"reflect"
	"testing"

	"element-skin/backend/internal/testutil"
)

func TestStoreReplacesEnabledEasterEggsInExactOrder(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.EasterEggs.ReplaceEnabled(ctx, []string{"christmas", "april-fools", "dragon-boat"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EasterEggs.ReplaceEnabled(ctx, []string{"mid-autumn", "children-day"}); err != nil {
		t.Fatal(err)
	}
	got, err := db.EasterEggs.ListEnabled(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"mid-autumn", "children-day"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enabled easter eggs=%#v, want %#v", got, want)
	}
	var rowCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM enabled_easter_eggs`).Scan(&rowCount); err != nil || rowCount != 2 {
		t.Fatalf("enabled easter egg row count=%d err=%v, want 2", rowCount, err)
	}
}

func TestStoreReplaceEnabledRollsBackOnInvalidDuplicate(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.EasterEggs.ReplaceEnabled(ctx, []string{"christmas"}); err != nil {
		t.Fatal(err)
	}
	if err := db.EasterEggs.ReplaceEnabled(ctx, []string{"april-fools", "april-fools"}); err == nil {
		t.Fatal("duplicate IDs should violate the primary key")
	}
	got, err := db.EasterEggs.ListEnabled(ctx)
	if err != nil || !reflect.DeepEqual(got, []string{"christmas"}) {
		t.Fatalf("failed replacement changed rows: got=%#v err=%v", got, err)
	}
}
