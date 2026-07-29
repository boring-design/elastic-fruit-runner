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

	mu       sync.RWMutex
	current  Vitals
	onUpdate func(Vitals)
}

// New creates a Service that records the given start time.
func New(startedAt time.Time) *Service {
	return &Service{startedAt: startedAt}
}

// Start begins periodic host metrics collection in a background goroutine.
// Blocks until ctx is cancelled.
func (s *Service) Start(ctx context.Context, interval time.Duration) {
	// Prime CPU counters so the first real sample has a delta to compare against.
	Collect()

	RunCollector(ctx, interval, func(v Vitals) {
		s.mu.Lock()
		s.current = v
		onUpdate := s.onUpdate
		s.mu.Unlock()
		if onUpdate != nil {
			onUpdate(v)
		}
	})
}

// SetOnUpdate sets the function called after each resource sample.
func (s *Service) SetOnUpdate(onUpdate func(Vitals)) {
	s.mu.Lock()
	s.onUpdate = onUpdate
	s.mu.Unlock()
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
