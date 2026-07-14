package app_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/app"
	"element-skin/backend/internal/database"
	"element-skin/backend/internal/model"
	"element-skin/backend/internal/permission"
	oauthsvc "element-skin/backend/internal/service/oauth"
	"element-skin/backend/internal/testutil"
)

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

func (r *recordingOAuthGrantCleaner) CleanupGrants(_ context.Context, actor permission.Actor, now int64) (oauthsvc.GrantCleanupResult, error) {
	r.calls.Add(1)
	r.actor.Store(actor)
	if now <= 0 ||
		!actor.Has(permission.MustDefinitionByCode("oauth_grant.revoke.system")) ||
		!actor.Has(permission.MustDefinitionByCode("oauth_grant.delete.system")) {
		r.invalidCall.Store(true)
	}
	return oauthsvc.GrantCleanupResult{}, errors.New("oauth grant boom")
}

func TestSchedulerOAuthGrantCleanupTaskUsesSystemMaintenanceActorAndSurvivesCleanupError(t *testing.T) {
	cleaner := &recordingOAuthGrantCleaner{}
	ctx, cancel := context.WithCancel(context.Background())
	done := app.StartScheduler(ctx, app.ScheduledTask{
		Name:     "oauth_grant_cleanup_test",
		Interval: fixedTestInterval(10 * time.Millisecond),
		Run: func(ctx context.Context) error {
			_, err := cleaner.CleanupGrants(ctx, permission.SystemMaintenanceActor(), database.NowMS())
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
