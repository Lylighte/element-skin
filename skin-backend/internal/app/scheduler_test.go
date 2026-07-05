package app_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"element-skin/backend/internal/app"
)

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
