package store

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Reconnect independently of refresh/fallback. A stable session should not
// inherit a long backoff from a previous outage.
func listen(ctx context.Context, subscribe func(context.Context, func()) error, wake func()) {
	const maxDelay = time.Minute
	delay := time.Second
	for ctx.Err() == nil {
		start := time.Now()
		err := subscribe(ctx, wake)
		if ctx.Err() != nil {
			return
		}
		if time.Since(start) >= maxDelay {
			delay = time.Second
		}
		slog.Warn("repo subscription interrupted", "err", err, "retry_in", delay)
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = min(2*delay, maxDelay)
	}
}

// StartSync subscribes while performing one synchronous startup refresh. Its
// stop function cancels and joins all work before the caller closes the store.
func StartSync(parent context.Context, subscribe func(context.Context, func()) error, refresh func(context.Context) error) (stop func()) {
	ctx, cancel := context.WithCancel(parent)
	var wg sync.WaitGroup
	// A single buffered bell retains one follow-up during a slow refresh.
	pending := make(chan struct{}, 1)
	wake := func() {
		select {
		case pending <- struct{}{}:
		default:
		}
	}
	wg.Go(func() { listen(ctx, subscribe, wake) })
	run := func() {
		if err := refresh(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("article refresh failed", "err", err)
		}
	}
	run()
	wg.Go(func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-pending:
				// Bound the coalescing window from the first signal so a busy
				// stream cannot starve the refresh indefinitely.
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				case <-ticker.C:
				}
			}
			// Signals already received are covered by the scan about to start.
			select {
			case <-pending:
			default:
			}
			if ctx.Err() == nil {
				run()
			}
		}
	})
	return func() { cancel(); wg.Wait() }
}
