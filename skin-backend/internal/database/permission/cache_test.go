package permission_test

import (
	"context"
	"testing"

	"element-skin/backend/internal/database"
	permissiondb "element-skin/backend/internal/database/permission"
	core "element-skin/backend/internal/permission"
	"element-skin/backend/internal/testutil"
)

func setupCacheStore(t *testing.T) (*database.DB, *permissiondb.RedisPermCache) {
	t.Helper()
	db, _ := testutil.NewTestAppTB(t)
	cache := &permissiondb.RedisPermCache{Store: testutil.NewMemoryRedis()}
	db.Permissions.Cache = cache
	return db, cache
}

func TestEffectivePermissionsCacheHitExactMatch(t *testing.T) {
	db, cache := setupCacheStore(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ch-hit@test.com", "pw", "CHHit", false)
	subjectID := permissiondb.SubjectIDForUser(user.ID)

	first, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	cached, ok, err := cache.GetEffective(ctx, subjectID)
	if err != nil || !ok {
		t.Fatalf("cache populated after first call: ok=%v err=%v", ok, err)
	}
	if len(first) != len(cached) {
		t.Fatalf("bitset length mismatch: %d vs %d", len(first), len(cached))
	}
	for i := range first {
		if first[i] != cached[i] {
			t.Fatalf("word[%d] mismatch: %#x vs %#x", i, first[i], cached[i])
		}
	}
	second, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range first {
		if second[i] != first[i] {
			t.Fatalf("cache-hit word[%d] mismatch: %#x vs %#x", i, first[i], second[i])
		}
	}
}

func TestEffectivePermissionsCacheMissOnColdStart(t *testing.T) {
	db, cache := setupCacheStore(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ch-miss@test.com", "pw", "CHMiss", false)
	subjectID := permissiondb.SubjectIDForUser(user.ID)

	_, ok, err := cache.GetEffective(ctx, subjectID)
	if err != nil || ok {
		t.Fatalf("cache empty before first call: ok=%v err=%v", ok, err)
	}
	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if bits.Empty() {
		t.Fatal("permissions should not be empty for a user")
	}
	_, ok, err = cache.GetEffective(ctx, subjectID)
	if err != nil || !ok {
		t.Fatalf("cache populated after call: ok=%v err=%v", ok, err)
	}
}

func TestGrantRoleInvalidatesCache(t *testing.T) {
	db, cache := setupCacheStore(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ch-grant@test.com", "pw", "CHGrant", false)
	subjectID := permissiondb.SubjectIDForUser(user.ID)

	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	def := core.MustDefinitionByCode("texture.review.assigned")
	if before.Has(def.BitIndex) {
		t.Fatal("user should not have moderator review before grant")
	}
	if err := db.Permissions.GrantRole(ctx, user.ID, core.RoleModerator, ""); err != nil {
		t.Fatal(err)
	}
	_, ok, err := cache.GetEffective(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("grant must invalidate cache")
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !after.Has(def.BitIndex) {
		t.Fatal("after grant, user must have moderator review")
	}
}

func TestSetOverrideInvalidatesCache(t *testing.T) {
	db, cache := setupCacheStore(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ch-over@test.com", "pw", "CHOver", false)
	subjectID := permissiondb.SubjectIDForUser(user.ID)
	def := core.MustDefinitionByCode("texture.update_visibility.owned")

	before, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !before.Has(def.BitIndex) {
		t.Fatal("user must have texture.update_visibility.owned before override")
	}
	if err := db.Permissions.SetSubjectPermissionOverride(ctx, user.ID, def, "deny", ""); err != nil {
		t.Fatal(err)
	}
	_, ok, err := cache.GetEffective(ctx, subjectID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("deny override must invalidate cache")
	}
	after, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if after.Has(def.BitIndex) {
		t.Fatal("after deny override, user must not have the permission")
	}
}

func TestNilCacheIsNoop(t *testing.T) {
	db, _ := testutil.NewTestAppTB(t)
	ctx := context.Background()
	user := testutil.CreateUser(t, db, "ch-nil@test.com", "pw", "CHNil", false)
	db.Permissions.Cache = nil

	bits, err := db.Permissions.EffectivePermissionsForUser(ctx, user.ID, permissiondb.EffectiveOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if bits.Empty() {
		t.Fatal("nil cache must not block resolution")
	}
	if err := db.Permissions.GrantRole(ctx, user.ID, core.RoleModerator, ""); err != nil {
		t.Fatal(err)
	}
}
