package vitals_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

func TestRunCollector_CallsOnUpdateImmediately(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	var called atomic.Bool
	go vitals.RunCollector(ctx, 10*time.Second, func(_ vitals.Vitals) {
		called.Store(true)
	})

	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			cancel()
			t.Fatal("onUpdate was not called within 5 seconds")
		case <-ticker.C:
			if called.Load() {
				cancel()
				return
			}
		}
	}
}

func TestRunCollector_StopsOnContextCancel(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		vitals.RunCollector(ctx, 50*time.Millisecond, func(v vitals.Vitals) {})
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// RunCollector returned after cancellation
	case <-time.After(5 * time.Second):
		t.Fatal("RunCollector did not stop after context cancellation")
	}
}

func TestRunCollector_CollectsMultipleTimes(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	var count atomic.Int32
	go vitals.RunCollector(ctx, 50*time.Millisecond, func(_ vitals.Vitals) {
		count.Add(1)
	})

	// Wait for at least 3 collections (1 immediate + 2 from ticker)
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatalf("expected at least 3 collections, got %d", count.Load())
		case <-ticker.C:
			if count.Load() >= 3 {
				return
			}
		}
	}
}
