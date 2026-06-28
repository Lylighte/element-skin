package setting_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database/setting"
	"element-skin/backend/internal/testutil"
)

func TestStoreSetGetIntGroupAndAllExactValues(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := setting.Store{Pool: db.Pool}
	if err := store.Set(ctx, "sub_bool", true); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(ctx, "sub_int", "42"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, "sub_bool", "false"); err != nil || got != "true" {
		t.Fatalf("setting get mismatch: got=%q err=%v", got, err)
	}
	if got, err := store.Int(ctx, "sub_int", 7); err != nil || got != 42 {
		t.Fatalf("setting int mismatch: got=%d err=%v", got, err)
	}
	group, err := store.Group(ctx, map[string]string{"sub_bool": "false", "missing": "false"})
	if err != nil || group["sub_bool"] != true || group["missing"] != false {
		t.Fatalf("setting group mismatch: group=%#v err=%v", group, err)
	}
	all, err := store.All(ctx)
	if err != nil || all["sub_bool"] != "true" || all["sub_int"] != "42" {
		t.Fatalf("setting all mismatch: all=%#v err=%v", all, err)
	}
}

func TestStoreSetFalseExactValue(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := setting.Store{Pool: db.Pool}
	if err := store.Set(ctx, "bool_false", false); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get(ctx, "bool_false", "true"); err != nil || got != "false" {
		t.Fatalf("setting get false mismatch: got=%q err=%v", got, err)
	}
}

func TestStoreGetIntNonIntegerFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := setting.Store{Pool: db.Pool}
	if err := store.Set(ctx, "non_int", "not-a-number"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Int(ctx, "non_int", 7); err != nil || got != 7 {
		t.Fatalf("setting int non-integer fallback mismatch: got=%d err=%v", got, err)
	}
}

func TestStoreGetMissingKeyReturnsFallback(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	store := setting.Store{Pool: db.Pool}
	if got, err := store.Get(ctx, "nonexistent_key", "default_val"); err != nil || got != "default_val" {
		t.Fatalf("setting get missing key mismatch: got=%q err=%v", got, err)
	}
}
