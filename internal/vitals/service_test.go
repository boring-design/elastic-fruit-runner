package vitals_test

import (
	"testing"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/vitals"
)

func TestNew_SetsStartedAt(t *testing.T) {
	t.Parallel()
	now := time.Now()
	svc := vitals.New(now)
	if !svc.StartedAt().Equal(now) {
		t.Fatalf("StartedAt() = %v, want %v", svc.StartedAt(), now)
	}
}

func TestGetVitals_InitiallyZero(t *testing.T) {
	t.Parallel()
	svc := vitals.New(time.Now())
	v := svc.GetVitals()
	if v.CPUUsagePercent != 0 || v.MemoryUsagePercent != 0 || v.DiskUsagePercent != 0 {
		t.Fatalf("expected zero vitals before Start(), got %+v", v)
	}
}

func TestService_CollectsAfterStart(t *testing.T) {
	t.Parallel()
	svc := vitals.New(time.Now())
	go svc.Start(t.Context(), 50*time.Millisecond)

	// Wait for at least one collection cycle
	deadline := time.After(5 * time.Second)
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("vitals never became non-zero after Start()")
		case <-ticker.C:
			v := svc.GetVitals()
			// Memory and disk usage should always be > 0 on a real system
			if v.MemoryUsagePercent > 0 && v.DiskUsagePercent > 0 {
				return
			}
		}
	}
}
