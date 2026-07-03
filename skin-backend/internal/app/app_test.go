package app_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/redisstore"
	"element-skin/backend/internal/testutil"

	"github.com/jackc/pgx/v5"
)

var appNewDatabaseCounter atomic.Uint64

func TestNewRejectsWeakJWTSecret(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.JWTSecret = "short"
	if _, err := app.New(context.Background(), cfg); err == nil {
		t.Fatal("weak JWT secret should reject startup")
	}
}

func TestNewOpensDependenciesBuildsRouterAndClosesExactly(t *testing.T) {
	ctx := context.Background()
	cfg := testutil.TestConfig()
	dbName := fmt.Sprintf("elementskin_app_new_%d_%d", os.Getpid(), appNewDatabaseCounter.Add(1))
	cfg.DatabaseDSN = createTemporaryDatabaseForAppNew(t, dbName)
	cfg.TexturesDir = t.TempDir()
	cfg.CarouselDir = t.TempDir()
	cfg.RedisKeyPrefix = cfg.RedisKeyPrefix + dbName + ":"

	application, err := app.New(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) {
		t.Fatalf("app.New router response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}

	application.Close()
	dropTemporaryDatabaseForAppNew(t, dbName)
}

func TestSchedulerRefreshCleanupTaskRemovesExpiredThenCancels(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	user := testutil.CreateUser(t, db, "cleanup@example.com", "Password123!", "CleanupUser", false)
	now := database.NowMS()
	if err := db.Tokens.AddRefresh(context.Background(), "hash_old", user.ID, now-10_000, now); err != nil {
		t.Fatal(err)
	}
	if err := db.Tokens.AddRefresh(context.Background(), "hash_new", user.ID, now+7*24*3600*1000, now); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "refresh_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			return db.Tokens.DeleteExpiredRefresh(ctx, database.NowMS())
		},
	})[0]

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		row, err := db.Tokens.GetRefresh(context.Background(), "hash_old")
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup loop did not stop after cancellation")
	}
	if row, err := db.Tokens.GetRefresh(context.Background(), "hash_old"); err != nil || row != nil {
		t.Fatalf("expired refresh token should be removed: row=%#v err=%v", row, err)
	}
	if row, err := db.Tokens.GetRefresh(context.Background(), "hash_new"); err != nil || row == nil {
		t.Fatalf("future refresh token should be retained: row=%#v err=%v", row, err)
	}
}

type flakyRefreshCleaner struct {
	calls atomic.Int64
}

func (f *flakyRefreshCleaner) DeleteExpiredRefresh(context.Context, int64) error {
	f.calls.Add(1)
	return errors.New("boom")
}

func TestSchedulerSurvivesCleanupError(t *testing.T) {
	cleaner := &flakyRefreshCleaner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "flaky_refresh_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			return cleaner.DeleteExpiredRefresh(ctx, database.NowMS())
		},
	})[0]

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && cleaner.calls.Load() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cleanup loop did not stop after cancellation")
	}
	if cleaner.calls.Load() < 2 {
		t.Fatalf("cleanup loop should continue after errors, calls=%d", cleaner.calls.Load())
	}
}

func TestSchedulerNoticeCleanupTaskRemovesExpiredThenCancels(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	now := database.NowMS()
	expiredID := "expired_notice_cleanup"
	if err := db.Notices.Create(context.Background(), model.Notice{
		ID:              expiredID,
		Type:            "announcement",
		Title:           "Expired",
		Summary:         "Expired summary",
		ContentMarkdown: "",
		DisplayMode:     "inline",
		Level:           "info",
		Audience:        "users",
		Enabled:         true,
		Dismissible:     true,
		EndsAt:          ptrInt64(now - 1000),
		CreatedAt:       now - 2000,
		UpdatedAt:       now - 2000,
	}); err != nil {
		t.Fatal(err)
	}
	activeID := "active_notice_cleanup"
	if err := db.Notices.Create(context.Background(), model.Notice{
		ID:              activeID,
		Type:            "announcement",
		Title:           "Active",
		Summary:         "Active summary",
		ContentMarkdown: "",
		DisplayMode:     "inline",
		Level:           "info",
		Audience:        "users",
		Enabled:         true,
		Dismissible:     true,
		EndsAt:          ptrInt64(now + 60_000),
		CreatedAt:       now,
		UpdatedAt:       now,
	}); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "notice_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			return db.Notices.DeleteExpired(ctx, database.NowMS())
		},
	})[0]
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		row, err := db.Notices.Get(context.Background(), expiredID)
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notice cleanup loop did not stop after cancellation")
	}
	if row, err := db.Notices.Get(context.Background(), expiredID); err != nil || row != nil {
		t.Fatalf("expired notice should be removed: row=%#v err=%v", row, err)
	}
	if row, err := db.Notices.Get(context.Background(), activeID); err != nil || row == nil || row.Title != "Active" {
		t.Fatalf("active notice should be retained: row=%#v err=%v", row, err)
	}
}

type flakyNoticeCleaner struct {
	calls atomic.Int64
}

func (f *flakyNoticeCleaner) DeleteExpired(context.Context, int64) error {
	f.calls.Add(1)
	return errors.New("notice boom")
}

func TestSchedulerNoticeCleanupTaskSurvivesCleanupError(t *testing.T) {
	cleaner := &flakyNoticeCleaner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "flaky_notice_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			return cleaner.DeleteExpired(ctx, database.NowMS())
		},
	})[0]

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && cleaner.calls.Load() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("notice cleanup loop did not stop after cancellation")
	}
	if cleaner.calls.Load() < 2 {
		t.Fatalf("notice cleanup loop should continue after errors, calls=%d", cleaner.calls.Load())
	}
}

type recordingOAuthGrantCleaner struct {
	calls       atomic.Int64
	actor       atomic.Value
	invalidCall atomic.Bool
}

func (r *recordingOAuthGrantCleaner) DeleteExpiredRevokedGrants(_ context.Context, actor permission.Actor, now int64) (int64, error) {
	r.calls.Add(1)
	r.actor.Store(actor)
	if now <= 0 || !actor.Has(permission.MustDefinitionByCode("oauth_grant.delete.system")) {
		r.invalidCall.Store(true)
	}
	return 0, errors.New("oauth grant boom")
}

func TestSchedulerOAuthGrantCleanupTaskUsesSystemMaintenanceActorAndSurvivesCleanupError(t *testing.T) {
	cleaner := &recordingOAuthGrantCleaner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "oauth_grant_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			_, err := cleaner.DeleteExpiredRevokedGrants(ctx, permission.SystemMaintenanceActor(), database.NowMS())
			return err
		},
	})[0]

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && cleaner.calls.Load() < 2 {
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("oauth grant cleanup loop did not stop after cancellation")
	}
	if cleaner.calls.Load() < 2 {
		t.Fatalf("oauth grant cleanup loop should continue after errors, calls=%d", cleaner.calls.Load())
	}
	if cleaner.invalidCall.Load() {
		t.Fatal("oauth grant cleanup loop should pass system maintenance actor and positive timestamp")
	}
	got, ok := cleaner.actor.Load().(permission.Actor)
	if !ok {
		t.Fatal("oauth grant cleanup loop did not pass an actor")
	}
	if got.SubjectID != "system:maintenance" || got.SessionKind != permission.SessionKindSystem || got.Entrypoint != permission.EntrypointMaintenance {
		t.Fatalf("oauth grant cleanup actor mismatch: %#v", got)
	}
}

func TestSchedulerRunsImmediateTaskOnceBeforeFirstInterval(t *testing.T) {
	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:           "immediate_test",
		RunImmediately: true,
		Interval:       fixedTestInterval(time.Hour),
		Run: func(context.Context) error {
			calls.Add(1)
			cancel()
			return nil
		},
	})[0]
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("immediate scheduler task did not stop after canceling its context")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("immediate task should run exactly once before first interval, got %d", got)
	}
}

func TestSchedulerSkipsImmediateTaskWhenAlreadyCanceled(t *testing.T) {
	var calls atomic.Int64
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:           "already_canceled_test",
		RunImmediately: true,
		Interval:       fixedTestInterval(time.Hour),
		Run: func(context.Context) error {
			calls.Add(1)
			return nil
		},
	})[0]
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("already-canceled scheduler task did not exit")
	}
	if got := calls.Load(); got != 0 {
		t.Fatalf("already-canceled immediate task should not run, got %d", got)
	}
}

func TestSchedulerExitsWithoutWorkForNilRunOrMissingInterval(t *testing.T) {
	ctx := context.Background()
	done := app.StartScheduler(ctx,
		app.ScheduledTask{Name: "nil_run", Interval: fixedTestInterval(time.Millisecond)},
		app.ScheduledTask{Name: "nil_interval", Run: func(context.Context) error { return nil }},
	)
	for i, ch := range done {
		select {
		case <-ch:
		case <-time.After(time.Second):
			t.Fatalf("scheduler task %d should exit without work", i)
		}
	}
}

func TestNewWithDBAndRedisClosesRedisWhenSignerInitializationFails(t *testing.T) {
	cfg := testutil.TestConfig()
	cfg.PrivateKeyPath = t.TempDir() + "/missing-private.pem"
	cache := &closeTrackingStore{Store: redisstore.NewMemoryStore()}

	application, err := app.NewWithDBAndRedis(cfg, nil, cache)
	if err == nil || application != nil {
		t.Fatalf("missing signing key should fail app construction: app=%#v err=%v", application, err)
	}
	if !cache.closed {
		t.Fatal("failed app construction must close the already-open Redis store")
	}
}

func TestNewWithDBBuildsWorkingRouterAndCloseReleasesRedis(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.RedisKeyPrefix = cfg.RedisKeyPrefix + "app-new-with-db:"
	application, err := app.NewWithDB(cfg, db)
	if err != nil {
		t.Fatal(err)
	}
	defer application.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/public/settings", nil)
	rec := httptest.NewRecorder()
	application.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"site_name"`) {
		t.Fatalf("NewWithDB router response mismatch: status=%d body=%q", rec.Code, rec.Body.String())
	}
}

func TestNewWithDBReturnsExactRedisConnectionError(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cfg.RedisAddr = "127.0.0.1:1"
	application, err := app.NewWithDB(cfg, db)
	if application != nil || err == nil || !strings.Contains(err.Error(), "connect redis 127.0.0.1:1") {
		t.Fatalf("NewWithDB redis error mismatch: app=%#v err=%v", application, err)
	}
}

func TestAppCloseReleasesDatabaseAndRedis(t *testing.T) {
	db, _ := testutil.NewTestApp(t)
	cfg := testutil.TestConfig()
	cache := &closeTrackingStore{Store: redisstore.NewMemoryStore()}
	application, err := app.NewWithDBAndRedis(cfg, db, cache)
	if err != nil {
		t.Fatal(err)
	}

	application.Close()

	if !cache.closed {
		t.Fatal("App.Close must close Redis")
	}
	if err := db.Pool.Ping(context.Background()); err == nil {
		t.Fatal("App.Close must close the database pool")
	}
}

type closeTrackingStore struct {
	redisstore.Store
	closed bool
}

func ptrInt64(v int64) *int64 {
	return &v
}

func fixedTestInterval(interval time.Duration) func(context.Context) time.Duration {
	return func(context.Context) time.Duration {
		return interval
	}
}

func (s *closeTrackingStore) Close() error {
	s.closed = true
	return s.Store.Close()
}

func createTemporaryDatabaseForAppNew(t *testing.T, dbName string) string {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		t.Fatalf("connect admin database: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), fmt.Sprintf(`CREATE DATABASE "%s"`, dbName)); err != nil {
		t.Fatalf("create app.New test database: %v", err)
	}
	t.Cleanup(func() {
		dropTemporaryDatabaseForAppNew(t, dbName)
	})
	return "postgresql://postgres:12345678@localhost:5432/" + dbName + "?sslmode=disable"
}

func dropTemporaryDatabaseForAppNew(t *testing.T, dbName string) {
	t.Helper()
	adminDSN := os.Getenv("ADMIN_DATABASE_DSN")
	if adminDSN == "" {
		adminDSN = "postgresql://postgres:12345678@localhost:5432/postgres?sslmode=disable"
	}
	conn, err := pgx.Connect(context.Background(), adminDSN)
	if err != nil {
		t.Fatalf("connect admin database for app.New cleanup: %v", err)
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(context.Background(), fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
		t.Fatalf("drop app.New test database: %v", err)
	}
}
