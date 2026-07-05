package permission_test

import (
	"context"
	"testing"
	"time"

	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func TestSessionPolicyAndBanNarrowPermissionsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "permission-ygg@test.com", "pw", "PermissionYgg", false)
	unbanned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindYggdrasil,
		Entrypoint:  core.EntrypointYggdrasil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(unbanned, "yggdrasil_server.join.bound_profile") || !has(unbanned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("yggdrasil session should include join and hasjoined before ban policy")
	}
	if has(unbanned, "account.read.self") {
		t.Fatal("yggdrasil session should not include dashboard account permissions")
	}
	bannedUntil := time.Now().Add(time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, bannedUntil, user.ID); err != nil {
		t.Fatal(err)
	}
	banned, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if has(banned, "yggdrasil_server.join.bound_profile") {
		t.Fatal("ban policy should revoke only yggdrasil_server.join.bound_profile")
	}
	if !has(banned, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("ban policy should keep yggdrasil_server.hasjoined.bound_profile")
	}
}

func TestExpiredBanRestoresJoinPermission(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "expired-ban@test.com", "pw", "ExpiredBan", false)
	expiredUntil := time.Now().Add(-time.Hour).UnixMilli()
	if _, err := db.Pool.Exec(ctx, `UPDATE users SET banned_until=$1 WHERE id=$2`, expiredUntil, user.ID); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind:    core.SessionKindYggdrasil,
		Entrypoint:     core.EntrypointYggdrasil,
		ApplyBanPolicy: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !has(bits, "yggdrasil_server.join.bound_profile") {
		t.Fatal("expired ban should restore join permission")
	}
	if !has(bits, "yggdrasil_server.hasjoined.bound_profile") {
		t.Fatal("expired ban should keep hasjoined permission")
	}
}

func TestSessionPolicyUsesCacheNotDatabase(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "session-policy-cache@test.com", "pw", "SessionPolicyCache", false)

	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: core.SessionKindWeb,
		Entrypoint:  core.EntrypointDashboard,
	})
	if err != nil {
		t.Fatalf("session policy should use in-memory cache, not DB: %v", err)
	}
	if !has(bits, "profile.create.owned") {
		t.Fatal("cached web session policy should include profile.create.owned")
	}
}

func TestUncachedSessionPolicyAndErrorsExactly(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "uncached-session-policy@test.com", "pw", "UncachedSessionPolicy", false)

	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: "custom-session",
		Entrypoint:  "custom-entrypoint",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bits.Empty() {
		t.Fatalf("unknown session policy should intersect to empty bitset, got %#v", bits)
	}

	if _, err := db.Pool.Exec(ctx, `DROP TABLE session_permission_policies CASCADE`); err != nil {
		t.Fatal(err)
	}
	_, err = db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{
		SessionKind: "custom-session",
		Entrypoint:  "custom-entrypoint",
	})
	assertPostgresError(t, err, "42P01")
}
