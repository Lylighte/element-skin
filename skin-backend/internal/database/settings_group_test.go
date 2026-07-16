package database_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/database/fallback"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestSaveSettingsGroupPersistsExactSettingsAndEndpoints(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Fallbacks.SaveEndpoints(ctx, []fallback.Endpoint{{
		Priority:   99,
		SessionURL: "https://old.example/session",
		AccountURL: "https://old.example/account",
	}}); err != nil {
		t.Fatal(err)
	}
	endpoints := []fallback.Endpoint{
		{
			Priority:        10,
			SessionURL:      "https://first.example/session",
			AccountURL:      "https://first.example/account",
			ServicesURL:     "https://first.example/services",
			CacheTTL:        45,
			SkinDomains:     []string{"first.example", "cdn.first.example"},
			EnableProfile:   true,
			EnableHasJoined: false,
			EnableWhitelist: true,
			Note:            "first",
		},
		{
			Priority:        20,
			SessionURL:      "https://second.example/session",
			AccountURL:      "https://second.example/account",
			ServicesURL:     "https://second.example/services",
			CacheTTL:        90,
			SkinDomains:     []string{"second.example"},
			EnableProfile:   false,
			EnableHasJoined: true,
			EnableWhitelist: false,
			Note:            "second",
		},
	}
	err := db.SaveSettingsGroup(ctx, []database.SettingUpdate{
		{Key: "fallback_strategy", Value: "priority"},
		{Key: "enable_skin_library", Value: false},
		{Key: "session_ttl", Value: 7200},
	}, endpoints, true)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"fallback_strategy":   "priority",
		"enable_skin_library": "false",
		"session_ttl":         "7200",
	} {
		got, err := db.Settings.Get(ctx, key, "")
		if err != nil || got != want {
			t.Fatalf("setting %q = %q, %v; want %q, nil", key, got, err, want)
		}
	}
	got, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("endpoint count = %d; want 2: %#v", len(got), got)
	}
	for i, want := range endpoints {
		item := got[i]
		if item["priority"] != want.Priority ||
			item["session_url"] != want.SessionURL ||
			item["account_url"] != want.AccountURL ||
			item["services_url"] != want.ServicesURL ||
			item["cache_ttl"] != want.CacheTTL ||
			!reflect.DeepEqual(item["skin_domains"], want.SkinDomains) ||
			item["enable_profile"] != want.EnableProfile ||
			item["enable_hasjoined"] != want.EnableHasJoined ||
			item["enable_whitelist"] != want.EnableWhitelist ||
			item["note"] != want.Note {
			t.Fatalf("endpoint %d mismatch: got=%#v want=%#v", i, item, want)
		}
	}
}

func TestSaveSettingsGroupRollsBackSettingsWhenEndpointInsertFails(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if err := db.Settings.Set(ctx, "fallback_strategy", "old-strategy"); err != nil {
		t.Fatal(err)
	}
	old := fallback.Endpoint{
		Priority:   1,
		SessionURL: "https://old.example/session",
		AccountURL: "https://old.example/account",
		CacheTTL:   30,
		Note:       "old",
	}
	if err := db.Fallbacks.SaveEndpoints(ctx, []fallback.Endpoint{old}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Pool.Exec(ctx, `
		ALTER TABLE fallback_endpoints
		ADD CONSTRAINT fallback_endpoint_positive_ttl CHECK (cache_ttl > 0)
	`); err != nil {
		t.Fatal(err)
	}
	err := db.SaveSettingsGroup(ctx,
		[]database.SettingUpdate{{Key: "fallback_strategy", Value: "new-strategy"}},
		[]fallback.Endpoint{{
			Priority:   2,
			SessionURL: "https://new.example/session",
			AccountURL: "https://new.example/account",
			CacheTTL:   0,
			Note:       "new",
		}},
		true,
	)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("SaveSettingsGroup error = %#v; want PostgreSQL 23514", err)
	}
	if got, err := db.Settings.Get(ctx, "fallback_strategy", ""); err != nil || got != "old-strategy" {
		t.Fatalf("strategy after rollback = %q, %v; want old-strategy, nil", got, err)
	}
	got, err := db.Fallbacks.ListEndpoints(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 ||
		got[0]["priority"] != old.Priority ||
		got[0]["session_url"] != old.SessionURL ||
		got[0]["account_url"] != old.AccountURL ||
		got[0]["note"] != old.Note {
		t.Fatalf("endpoints after rollback = %#v; want original endpoint", got)
	}
}
