package resilience

import (
	"context"
	"errors"
	"time"
)

type Backoff struct {
	Attempts int
	Initial  time.Duration
	Max      time.Duration
}

func Retry(ctx context.Context, cfg Backoff, fn func(context.Context) error) error {
	if cfg.Attempts <= 0 {
		cfg.Attempts = 3
	}
	if cfg.Initial <= 0 {
		cfg.Initial = 200 * time.Millisecond
	}
	if cfg.Max <= 0 {
		cfg.Max = 2 * time.Second
	}

	var lastErr error
	delay := cfg.Initial
	for attempt := 1; attempt <= cfg.Attempts; attempt++ {
		if err := fn(ctx); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt == cfg.Attempts {
			break
		}
		select {
		case <-ctx.Done():
			return errors.Join(ctx.Err(), lastErr)
		case <-time.After(delay):
		}
		delay *= 2
		if delay > cfg.Max {
			delay = cfg.Max
		}
	}
	return lastErr
}
