package app

import (
	"context"
	"time"
)

type ScheduledTask struct {
	Name           string
	Interval       func(context.Context) time.Duration
	RunImmediately bool
	Run            func(context.Context) error
}

func StartScheduler(ctx context.Context, tasks ...ScheduledTask) []<-chan struct{} {
	done := make([]<-chan struct{}, 0, len(tasks))
	for _, task := range tasks {
		done = append(done, startScheduledTask(ctx, task))
	}
	return done
}

func startScheduledTask(ctx context.Context, task ScheduledTask) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if task.Run == nil {
			return
		}
		if task.RunImmediately {
			if ctx.Err() != nil {
				return
			}
			_ = task.Run(ctx)
		}
		for {
			interval := taskInterval(ctx, task)
			if interval <= 0 {
				return
			}
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				return
			case <-timer.C:
			}
			if ctx.Err() != nil {
				return
			}
			_ = task.Run(ctx)
		}
	}()
	return done
}

func taskInterval(ctx context.Context, task ScheduledTask) time.Duration {
	if task.Interval == nil {
		return 0
	}
	return task.Interval(ctx)
}

func fixedInterval(interval time.Duration) func(context.Context) time.Duration {
	return func(context.Context) time.Duration {
		return interval
	}
}
