package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"element-skin/backend/internal/permission"
	oauthsvc "element-skin/backend/internal/service/oauth"
)

type taskRefreshCleaner struct {
	cutoff int64
	err    error
}

func (c *taskRefreshCleaner) DeleteExpiredRefresh(_ context.Context, cutoff int64) error {
	c.cutoff = cutoff
	return c.err
}

type taskNoticeCleaner struct {
	cutoff int64
	actor  permission.Actor
	err    error
}

func (c *taskNoticeCleaner) DeleteExpired(_ context.Context, actor permission.Actor, cutoff int64) error {
	c.actor = actor
	c.cutoff = cutoff
	return c.err
}

type taskOAuthGrantCleaner struct {
	actor permission.Actor
	now   int64
	err   error
}

func (c *taskOAuthGrantCleaner) CleanupGrants(_ context.Context, actor permission.Actor, now int64) (oauthsvc.GrantCleanupResult, error) {
	c.actor = actor
	c.now = now
	return oauthsvc.GrantCleanupResult{Revoked: 3, Deleted: 7}, c.err
}

func TestCleanupTaskConstructorsRunExactCleaners(t *testing.T) {
	ctx := context.Background()
	boom := errors.New("cleanup boom")

	refresh := &taskRefreshCleaner{err: boom}
	refreshTask := refreshCleanupTask(refresh, 15*time.Minute)
	if refreshTask.Name != "refresh_token_cleanup" || refreshTask.RunImmediately || refreshTask.Interval(ctx) != 15*time.Minute {
		t.Fatalf("refresh cleanup task metadata mismatch: %#v", refreshTask)
	}
	if err := refreshTask.Run(ctx); !errors.Is(err, boom) || refresh.cutoff <= 0 {
		t.Fatalf("refresh cleanup run mismatch: cutoff=%d err=%v", refresh.cutoff, err)
	}

	notice := &taskNoticeCleaner{err: boom}
	noticeTask := noticeCleanupTask(notice, 30*time.Minute)
	if noticeTask.Name != "notice_cleanup" || noticeTask.RunImmediately || noticeTask.Interval(ctx) != 30*time.Minute {
		t.Fatalf("notice cleanup task metadata mismatch: %#v", noticeTask)
	}
	if err := noticeTask.Run(ctx); !errors.Is(err, boom) || notice.cutoff <= 0 ||
		notice.actor.SubjectID != "system:maintenance" ||
		notice.actor.SessionKind != permission.SessionKindSystem ||
		notice.actor.Entrypoint != permission.EntrypointMaintenance ||
		!notice.actor.Has(permission.MustDefinitionByCode("notice.delete.system")) {
		t.Fatalf("notice cleanup run mismatch: cutoff=%d actor=%#v err=%v", notice.cutoff, notice.actor, err)
	}

	oauth := &taskOAuthGrantCleaner{err: boom}
	oauthTask := oauthGrantCleanupTask(oauth, time.Hour)
	if oauthTask.Name != "oauth_grant_cleanup" || oauthTask.RunImmediately || oauthTask.Interval(ctx) != time.Hour {
		t.Fatalf("oauth cleanup task metadata mismatch: %#v", oauthTask)
	}
	if err := oauthTask.Run(ctx); !errors.Is(err, boom) || oauth.now <= 0 ||
		oauth.actor.SubjectID != "system:maintenance" ||
		oauth.actor.SessionKind != permission.SessionKindSystem ||
		oauth.actor.Entrypoint != permission.EntrypointMaintenance ||
		!oauth.actor.Has(permission.MustDefinitionByCode("oauth_grant.revoke.system")) ||
		!oauth.actor.Has(permission.MustDefinitionByCode("oauth_grant.delete.system")) {
		t.Fatalf("oauth cleanup run mismatch: now=%d actor=%#v err=%v", oauth.now, oauth.actor, err)
	}
}

func TestProbeStatusTaskWithNilServiceReturnsNilAndUsesIntervalReader(t *testing.T) {
	task := probeStatusTask(nil, taskIntervalReader{value: "120"})
	if task.Name != "probe_status" || !task.RunImmediately {
		t.Fatalf("probe task metadata mismatch: %#v", task)
	}
	if got := task.Interval(context.Background()); got != 2*time.Minute {
		t.Fatalf("probe interval=%s want 2m", got)
	}
	if err := task.Run(context.Background()); err != nil {
		t.Fatalf("nil probe service should return nil, got %v", err)
	}
}

type taskIntervalReader struct {
	value string
}

func (r taskIntervalReader) Get(context.Context, string, string) (string, error) {
	return r.value, nil
}
