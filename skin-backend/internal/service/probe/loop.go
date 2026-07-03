package probe

import (
	"context"
	"strconv"
	"time"
)

const (
	defaultIntervalKey = "fallback_probe_interval"
	defaultInterval    = 10 * time.Minute
	minInterval        = time.Minute
)

type IntervalReader interface {
	Get(ctx context.Context, key, fallback string) (string, error)
}

func ReadInterval(ctx context.Context, reader IntervalReader) time.Duration {
	if reader == nil {
		return defaultInterval
	}
	raw, err := reader.Get(ctx, defaultIntervalKey, strconv.Itoa(int(defaultInterval/time.Second)))
	if err != nil {
		return defaultInterval
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return defaultInterval
	}
	d := time.Duration(n) * time.Second
	if d < minInterval {
		return minInterval
	}
	return d
}
