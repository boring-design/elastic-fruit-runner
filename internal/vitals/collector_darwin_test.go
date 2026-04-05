//go:build darwin

package vitals

import (
	"testing"
	"time"
)

func TestCollect_Darwin_ReturnsNonZero(t *testing.T) {
	// First call seeds the CPU counters; second call computes a delta.
	Collect()
	time.Sleep(100 * time.Millisecond)
	v := Collect()

	if v.CPUUsagePercent < 0 || v.CPUUsagePercent > 100 {
		t.Errorf("CPUUsagePercent out of range: %f", v.CPUUsagePercent)
	}
	if v.MemoryUsagePercent <= 0 || v.MemoryUsagePercent > 100 {
		t.Errorf("MemoryUsagePercent out of range: %f", v.MemoryUsagePercent)
	}
	if v.DiskUsagePercent <= 0 || v.DiskUsagePercent > 100 {
		t.Errorf("DiskUsagePercent out of range: %f", v.DiskUsagePercent)
	}
	// Temperature may be 0 on darwin (IOKit not implemented yet), so no assertion.
}
