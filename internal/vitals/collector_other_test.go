//go:build !darwin && !linux

package vitals

import "testing"

func TestCollect_Other_ReturnsZero(t *testing.T) {
	v := Collect()

	if v.CPUUsagePercent != 0 {
		t.Errorf("expected CPUUsagePercent 0, got %f", v.CPUUsagePercent)
	}
	if v.MemoryUsagePercent != 0 {
		t.Errorf("expected MemoryUsagePercent 0, got %f", v.MemoryUsagePercent)
	}
	if v.DiskUsagePercent != 0 {
		t.Errorf("expected DiskUsagePercent 0, got %f", v.DiskUsagePercent)
	}
	if v.TemperatureCelsius != 0 {
		t.Errorf("expected TemperatureCelsius 0, got %f", v.TemperatureCelsius)
	}
}
