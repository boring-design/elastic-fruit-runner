package vitals

import (
	"context"
	"time"
)

// Vitals holds a point-in-time snapshot of host resource metrics.
type Vitals struct {
	CPUUsagePercent    float32
	MemoryUsagePercent float32
	DiskUsagePercent   float32
	TemperatureCelsius float32
}

// CollectFunc is the signature for a function that collects host metrics.
type CollectFunc func() Vitals

// RunCollector periodically collects host metrics and calls onUpdate.
// Blocks until ctx is cancelled.
func RunCollector(ctx context.Context, interval time.Duration, onUpdate func(Vitals)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Collect immediately on start.
	onUpdate(Collect())

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			onUpdate(Collect())
		}
	}
}
