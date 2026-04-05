package vitals

import (
	"context"
	"sync"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/hostmetrics"
)

// Service provides process-level information (startedAt) and periodically
// collected host resource metrics.
type Service struct {
	startedAt time.Time

	mu      sync.RWMutex
	current hostmetrics.Vitals
}

// New creates a VitalsService that records the given start time.
func New(startedAt time.Time) *Service {
	return &Service{startedAt: startedAt}
}

// Start begins periodic host metrics collection in a background goroutine.
// Blocks until ctx is cancelled.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	hostmetrics.RunCollector(ctx, interval, func(v hostmetrics.Vitals) {
		s.mu.Lock()
		s.current = v
		s.mu.Unlock()
	})
}

// StartedAt returns when the daemon process started.
func (s *Service) StartedAt() time.Time {
	return s.startedAt
}

// GetVitals returns the latest host metrics snapshot.
func (s *Service) GetVitals() hostmetrics.Vitals {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}
