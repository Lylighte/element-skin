package database_test

import (
	"context"
	"strings"
	"testing"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/testutil"
)

func TestInitSQLContainsExpectedTablesConstraintsIndexesAndSeeds(t *testing.T) {
	required := []string{
		"CREATE TABLE IF NOT EXISTS users",
		"CREATE TABLE IF NOT EXISTS profiles",
		"email TEXT UNIQUE NOT NULL",
		"name TEXT UNIQUE NOT NULL",
		"PRIMARY KEY(user_id, hash, texture_type)",
		"PRIMARY KEY(skin_hash, texture_type)",
		"UNIQUE(username, endpoint_id)",
		"idx_profiles_user_id",
		"idx_site_refresh_expires",
		"('site_name', '皮肤站')",
		"ON CONFLICT (key) DO NOTHING",
	}
	for _, fragment := range required {
		if !strings.Contains(database.InitSQL, fragment) {
			t.Fatalf("InitSQL missing fragment %q", fragment)
		}
	}
}

func TestInitSQLPromotesOldestAdminOrFirstUserToSuperAdmin(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	ctx := context.Background()
	if _, err := db.Pool.Exec(ctx, `
		TRUNCATE users CASCADE;
		ALTER TABLE users DROP COLUMN IF EXISTS is_super_admin;
		ALTER TABLE users DROP COLUMN IF EXISTS created_at;
		INSERT INTO users (id,email,password,is_admin,display_name) VALUES
			('z-user','z@test.com','pw',FALSE,'Zed'),
			('a-admin','a@test.com','pw',TRUE,'AdminA'),
			('b-admin','b@test.com','pw',TRUE,'AdminB');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	var superID string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM users WHERE is_super_admin=TRUE`).Scan(&superID); err != nil {
		t.Fatal(err)
	}
	if superID != "a-admin" {
		t.Fatalf("expected oldest admin to become super admin, got %q", superID)
	}

	if _, err := db.Pool.Exec(ctx, `UPDATE users SET is_admin=FALSE, is_super_admin=FALSE`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM users WHERE is_super_admin=TRUE`).Scan(&superID); err != nil {
		t.Fatal(err)
	}
	if superID != "a-admin" {
		t.Fatalf("expected first user by migration ordering to become super admin, got %q", superID)
	}

	if _, err := db.Pool.Exec(ctx, `
		DROP INDEX IF EXISTS idx_users_single_super_admin;
		UPDATE users SET is_super_admin=TRUE, is_admin=TRUE WHERE id IN ('a-admin','b-admin');
	`); err != nil {
		t.Fatal(err)
	}
	if err := db.Init(ctx); err != nil {
		t.Fatal(err)
	}
	var superCount int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE is_super_admin=TRUE`).Scan(&superCount); err != nil {
		t.Fatal(err)
	}
	if superCount != 1 {
		t.Fatalf("Init should normalize to exactly one super admin, got %d", superCount)
	}
}
