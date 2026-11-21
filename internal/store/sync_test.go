package store_test

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/wescran/cranlab/internal/store"
)

func TestBurstAndWakeDuringRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var ring func()
		var calls atomic.Int32
		release := make(chan struct{})
		stop := store.StartSync(t.Context(), func(ctx context.Context, wake func()) error {
			ring = wake
			<-ctx.Done()
			return ctx.Err()
		}, func(ctx context.Context, full bool) error {
			n := calls.Add(1)
			if full != (n == 1) {
				t.Errorf("refresh %d: full=%v", n, full)
			}
			if n == 2 {
				select {
				case <-release:
				case <-ctx.Done():
				}
			}
			return nil
		})
		defer stop()
		synctest.Wait()
		burst := func() {
			for range 5 {
				ring()
				synctest.Wait()
			}
		}
		burst()
		time.Sleep(999 * time.Millisecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("burst was not debounced: %d calls", calls.Load())
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatalf("burst did not refresh: %d calls", calls.Load())
		}
		burst()
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatalf("refreshes overlapped: %d calls", calls.Load())
		}
		close(release)
		time.Sleep(2 * time.Second)
		synctest.Wait()
		if calls.Load() != 3 {
			t.Fatalf("pending burst: got %d calls, want 3", calls.Load())
		}
	})
}

func TestRetryInterruptedSubscription(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32
		subscribe := func(context.Context, func()) error {
			attempts.Add(1)
			return io.EOF
		}
		stop := store.StartSync(t.Context(), subscribe, func(context.Context, bool) error { return nil })
		defer stop()
		for i, delay := range []time.Duration{time.Second, 2 * time.Second, 4 * time.Second} {
			synctest.Wait()
			if attempts.Load() != int32(i+1) {
				t.Fatalf("attempts before backoff: %d, want %d", attempts.Load(), i+1)
			}
			time.Sleep(delay)
		}
		synctest.Wait()
		if attempts.Load() != 4 {
			t.Fatalf("attempts: %d, want 4", attempts.Load())
		}
		now := time.Now()
		stop()
		if !time.Now().Equal(now) {
			t.Fatal("shutdown waited for reconnect backoff")
		}
	})
}

func TestFallbackWithoutSignals(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var calls atomic.Int32
		stop := store.StartSync(ctx, func(ctx context.Context, wake func()) error {
			<-ctx.Done()
			return ctx.Err()
		}, func(ctx context.Context, full bool) error {
			if !full {
				t.Error("startup/fallback must reconcile all posts")
			}
			if calls.Add(1) == 2 {
				<-ctx.Done() // cancel while the fallback is in flight
				return ctx.Err()
			}
			return nil
		})
		defer stop()
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("startup: got %d refreshes", calls.Load())
		}
		time.Sleep(15*time.Minute - time.Millisecond)
		synctest.Wait()
		if calls.Load() != 1 {
			t.Fatalf("premature fallback: %d refreshes", calls.Load())
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if calls.Load() != 2 {
			t.Fatalf("fallback: got %d refreshes", calls.Load())
		}
		now := time.Now()
		cancel()
		stop()
		if !time.Now().Equal(now) {
			t.Fatal("shutdown waited for timer")
		}
	})
}
