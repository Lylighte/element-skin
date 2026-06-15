package redisstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/model"
	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreCachesAndInvalidatesPublicData(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty setting should miss, got %v", err)
	}
	if err := store.SetSetting(ctx, "site_name", "Cached Setting", time.Minute); err != nil {
		t.Fatal(err)
	}
	setting, err := store.GetSetting(ctx, "site_name")
	if err != nil || setting != "Cached Setting" {
		t.Fatalf("setting cache mismatch: %q err=%v", setting, err)
	}
	if err := store.InvalidateSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetSetting(ctx, "site_name"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated setting should miss, got %v", err)
	}

	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("empty settings should miss, got %v", err)
	}
	if err := store.SetPublicSettings(ctx, map[string]any{"site_name": "Cached"}, time.Minute); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPublicSettings(ctx)
	if err != nil || got["site_name"] != "Cached" {
		t.Fatalf("settings cache mismatch: %#v err=%v", got, err)
	}
	got["site_name"] = "mutated"
	again, _ := store.GetPublicSettings(ctx)
	if again["site_name"] != "Cached" {
		t.Fatalf("cache should return cloned data, got %#v", again)
	}
	if err := store.SetPublicHomepageMedia(ctx, []model.HomepageMedia{{ID: "a", Type: "image", StoragePath: "a.png"}}, time.Minute); err != nil {
		t.Fatal(err)
	}
	homepageMedia, err := store.GetPublicHomepageMedia(ctx)
	if err != nil || len(homepageMedia) != 1 || homepageMedia[0].StoragePath != "a.png" {
		t.Fatalf("homepage media cache mismatch: %#v err=%v", homepageMedia, err)
	}
	if err := store.InvalidatePublicSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicSettings(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated settings should miss, got %v", err)
	}
	if err := store.InvalidatePublicHomepageMedia(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetPublicHomepageMedia(ctx); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated homepage media should miss, got %v", err)
	}
}

func TestMemoryStoreVerificationRateLimitAndAuthCache(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	if err := store.SetVerificationCode(ctx, "User@Example.com", "register", "ABC12345", time.Minute); err != nil {
		t.Fatal(err)
	}
	code, err := store.GetVerificationCode(ctx, "user@example.com", "register")
	if err != nil || code != "ABC12345" {
		t.Fatalf("verification code mismatch: %q err=%v", code, err)
	}
	if err := store.DeleteVerificationCode(ctx, "user@example.com", "register"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "register"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("deleted code should miss, got %v", err)
	}

	for i := 0; i < 2; i++ {
		res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
		if err != nil || !res.Allowed {
			t.Fatalf("hit %d should be allowed: %#v err=%v", i+1, res, err)
		}
	}
	res, err := store.HitRateLimit(ctx, "login:ip:192.0.2.1", 2, time.Minute)
	if err != nil || res.Allowed || res.Remaining != 0 {
		t.Fatalf("third hit should be rejected: %#v err=%v", res, err)
	}

	until := time.Now().Add(time.Hour).UnixMilli()
	auth := redisstore.AuthUser{ID: "u1", IsAdmin: true, BannedUntil: &until}
	if err := store.SetAuthUser(ctx, auth, time.Minute); err != nil {
		t.Fatal(err)
	}
	cached, err := store.GetAuthUser(ctx, "u1")
	if err != nil || !cached.IsAdmin || !cached.Banned(time.Now()) {
		t.Fatalf("auth cache mismatch: %#v err=%v", cached, err)
	}
	if err := store.InvalidateAuthUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetAuthUser(ctx, "u1"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("invalidated auth cache should miss, got %v", err)
	}
}

func TestMemoryStoreConcurrentRateLimitAllowsExactThreshold(t *testing.T) {
	store := redisstore.NewMemoryStore()
	const attempts = 25
	const limit = 7
	type result struct {
		value redisstore.RateLimitResult
		err   error
	}
	start := make(chan struct{})
	results := make(chan result, attempts)
	var wg sync.WaitGroup
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := store.HitRateLimit(context.Background(), "concurrent-memory", limit, time.Minute)
			results <- result{value: value, err: err}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	allowed := 0
	rejected := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent memory rate limit failed: %v", result.err)
		}
		if result.value.Allowed {
			allowed++
		} else {
			rejected++
		}
		if result.value.Remaining < 0 || result.value.Remaining > limit-1 || result.value.RetryAfter <= 0 {
			t.Fatalf("concurrent memory rate limit returned invalid metadata: %#v", result.value)
		}
	}
	if allowed != limit || rejected != attempts-limit {
		t.Fatalf("concurrent memory rate limit allowed=%d rejected=%d; want %d and %d", allowed, rejected, limit, attempts-limit)
	}
	final, err := store.HitRateLimit(context.Background(), "concurrent-memory", limit, time.Minute)
	if err != nil || final.Allowed || final.Remaining != 0 {
		t.Fatalf("final memory rate-limit state=%#v err=%v; want rejected with zero remaining", final, err)
	}
}

func TestMemoryStoreConsumesVerificationCodeExactlyOnce(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	const code = "ABC12345"
	if err := store.SetVerificationCode(ctx, "User@Example.com", "reset", code, time.Minute); err != nil {
		t.Fatal(err)
	}
	if consumed, err := store.ConsumeVerificationCode(ctx, "user@example.com", "RESET", "wrong"); err != nil || consumed {
		t.Fatalf("wrong code consumption = %v, %v; want false, nil", consumed, err)
	}
	if stored, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || stored != code {
		t.Fatalf("wrong code must remain stored: code=%q err=%v", stored, err)
	}

	var wg sync.WaitGroup
	results := make(chan bool, 2)
	errs := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			consumed, err := store.ConsumeVerificationCode(ctx, "USER@example.com", "reset", "abc12345")
			results <- consumed
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	successes := 0
	for consumed := range results {
		if consumed {
			successes++
		}
	}
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent consumption failed: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful consumptions=%d, want exactly 1", successes)
	}
	if _, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("consumed code should miss, got %v", err)
	}
}

func TestMemoryStoreSetsVerificationCodeOnlyWhenAbsent(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	if err := store.SetVerificationCode(ctx, "user@example.com", "reset", "NEWCODE1", time.Minute); err != nil {
		t.Fatal(err)
	}
	set, err := store.SetVerificationCodeIfAbsent(ctx, "user@example.com", "reset", "OLDCODE1", time.Minute)
	if err != nil || set {
		t.Fatalf("set-if-absent with existing code = %v, %v; want false, nil", set, err)
	}
	if stored, err := store.GetVerificationCode(ctx, "user@example.com", "reset"); err != nil || stored != "NEWCODE1" {
		t.Fatalf("existing code was overwritten: code=%q err=%v", stored, err)
	}
}

func TestMemoryStoreYggTokenLifecycleAndTrim(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	profileID := "p1"

	for i := 1; i <= 4; i++ {
		if err := store.SetYggToken(ctx, model.Token{
			AccessToken: "access_" + string(rune('0'+i)),
			ClientToken: "client",
			UserID:      "u1",
			ProfileID:   &profileID,
			CreatedAt:   int64(i),
		}, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.TrimYggTokensByUser(ctx, "u1", 2); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_1", "access_2"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be trimmed, got %v", access, err)
		}
	}
	for _, access := range []string{"access_3", "access_4"} {
		token, err := store.GetYggToken(ctx, access)
		if err != nil || token.UserID != "u1" || token.ProfileID == nil || *token.ProfileID != profileID {
			t.Fatalf("%s should remain: %#v err=%v", access, token, err)
		}
	}

	replaced, err := store.ReplaceYggToken(ctx, "access_3", model.Token{
		AccessToken: "access_new",
		ClientToken: "client",
		UserID:      "u1",
		ProfileID:   &profileID,
		CreatedAt:   5,
	}, time.Minute)
	if err != nil || !replaced {
		t.Fatalf("replace should succeed: replaced=%v err=%v", replaced, err)
	}
	if _, err := store.GetYggToken(ctx, "access_3"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("old token should miss after replace, got %v", err)
	}
	if token, err := store.GetYggToken(ctx, "access_new"); err != nil || token.UserID != "u1" {
		t.Fatalf("new token mismatch: %#v err=%v", token, err)
	}

	if err := store.DeleteYggTokensByUser(ctx, "u1"); err != nil {
		t.Fatal(err)
	}
	for _, access := range []string{"access_4", "access_new"} {
		if _, err := store.GetYggToken(ctx, access); !errors.Is(err, redisstore.ErrCacheMiss) {
			t.Fatalf("%s should be deleted by user, got %v", access, err)
		}
	}
}

func TestMemoryStoreYggSessionTTL(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()
	if err := store.SetYggSession(ctx, model.Session{ServerID: "server", AccessToken: "access", CreatedAt: 1}, time.Nanosecond); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	if _, err := store.GetYggSession(ctx, "server"); !errors.Is(err, redisstore.ErrCacheMiss) {
		t.Fatalf("expired ygg session should miss, got %v", err)
	}
}

func TestMemoryStoreFallbackRequestLoopGuardLifecycle(t *testing.T) {
	store := redisstore.NewMemoryStore()
	ctx := context.Background()

	first, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !first {
		t.Fatalf("first fallback request should be marked as new: first=%v err=%v", first, err)
	}
	duplicate, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || duplicate {
		t.Fatalf("duplicate fallback request should be rejected until cleared: duplicate=%v err=%v", duplicate, err)
	}
	otherEndpoint, err := store.MarkFallbackRequest(ctx, "https://other.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !otherEndpoint {
		t.Fatalf("same request on different endpoint should have independent guard: first=%v err=%v", otherEndpoint, err)
	}
	if err := store.DeleteFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server"); err != nil {
		t.Fatal(err)
	}
	afterDelete, err := store.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "hasJoined:Player:server", time.Minute)
	if err != nil || !afterDelete {
		t.Fatalf("deleted fallback guard should allow request again: first=%v err=%v", afterDelete, err)
	}

	expiringStore := redisstore.NewMemoryStore()
	expiringFirst, err := expiringStore.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:uuid", time.Nanosecond)
	if err != nil || !expiringFirst {
		t.Fatalf("first expiring fallback mark mismatch: first=%v err=%v", expiringFirst, err)
	}
	time.Sleep(time.Millisecond)
	afterTTL, err := expiringStore.MarkFallbackRequest(ctx, "https://fallback.example/ygg", "profile:uuid", time.Minute)
	if err != nil || !afterTTL {
		t.Fatalf("expired fallback guard should allow request again: first=%v err=%v", afterTTL, err)
	}
}
