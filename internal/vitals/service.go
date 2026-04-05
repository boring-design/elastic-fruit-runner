package vitals

import (
	"context"
	"sync"
	"time"
)

// Service provides process-level information (startedAt) and periodically
// collected host resource metrics.
type Service struct {
	startedAt time.Time

	mu      sync.RWMutex
	current Vitals
}

// New creates a Service that records the given start time.
func New(startedAt time.Time) *Service {
	return &Service{startedAt: startedAt}
}

// Start begins periodic host metrics collection in a background goroutine.
// Blocks until ctx is cancelled.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	RunCollector(ctx, interval, func(v Vitals) {
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
func (s *Service) GetVitals() Vitals {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}
