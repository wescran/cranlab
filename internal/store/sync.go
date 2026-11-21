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

// StartSync starts the subscription and serialized refresh loop in the background. Its
// stop function cancels and joins all work before the caller closes the store.
func StartSync(parent context.Context, subscribe func(context.Context, func()) error, refresh func(context.Context, bool) error) (stop func()) {
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
	run := func(full bool) {
		if err := refresh(ctx, full); err != nil && ctx.Err() == nil {
			slog.Warn("snapshot refresh failed", "err", err)
		}
	}
	wg.Go(func() {
		run(true)
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for {
			full := false
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				full = true
			case <-pending:
				// Bound the coalescing window from the first signal so a busy
				// stream cannot starve the refresh indefinitely.
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Second):
				case <-ticker.C:
					full = true
				}
			}
			// Signals already received are covered by the scan about to start.
			select {
			case <-pending:
			default:
			}
			if ctx.Err() == nil {
				run(full)
			}
		}
	})
	return func() { cancel(); wg.Wait() }
}
