package app_test

import (
	"context"
	"time"
)

func ptrInt64(v int64) *int64 {
	return &v
}

func fixedTestInterval(interval time.Duration) func(context.Context) time.Duration {
	return func(context.Context) time.Duration {
		return interval
	}
}
