package redisstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"element-skin/backend/internal/redisstore"
)

func TestMemoryStoreVerificationCodeLifecycle(t *testing.T) {
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

	store.Err = errors.New("forced verification memory error")
	set, err = store.SetVerificationCodeIfAbsent(ctx, "new@example.com", "reset", "NEWCODE2", time.Minute)
	if set || err == nil || err.Error() != "forced verification memory error" {
		t.Fatalf("set-if-absent backing error = %v, %v; want false and forced verification memory error", set, err)
	}
	if err := store.DeleteVerificationCode(ctx, "new@example.com", "reset"); err == nil || err.Error() != "forced verification memory error" {
		t.Fatalf("DeleteVerificationCode backing error=%v; want forced verification memory error", err)
	}
}
