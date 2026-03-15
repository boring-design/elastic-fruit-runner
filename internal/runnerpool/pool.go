package runnerpool

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/boring-design/elastic-fruit-runner/internal/backend"
)

const (
	maxBackoff     = 30 * time.Second
	initialBackoff = 1 * time.Second
)

// RunnerPool manages a fixed set of runner slots with background warm-up.
// Slots are pre-warmed (backend.Prepare) so they are ready when jobs arrive.
type RunnerPool struct {
	size    int
	backend backend.Backend
	logger  *slog.Logger

	slots   []*Slot
	readyCh chan *Slot

	mu sync.Mutex
}

// New creates a RunnerPool with the given number of slots.
func New(size int, b backend.Backend, logger *slog.Logger) *RunnerPool {
	slots := make([]*Slot, size)
	for i := range size {
		slots[i] = &Slot{
			ID:    i,
			Name:  generateSlotName(i),
			State: SlotIdle,
		}
	}
	return &RunnerPool{
		size:    size,
		backend: b,
		logger:  logger,
		slots:   slots,
		readyCh: make(chan *Slot, size),
	}
}

// Start launches background warm-up goroutines for each slot.
// Blocks until ctx is cancelled.
func (p *RunnerPool) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for _, s := range p.slots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.warmUpLoop(ctx, s)
		}()
	}
	wg.Wait()
}

// Acquire blocks until a ready slot is available, then returns it.
func (p *RunnerPool) Acquire(ctx context.Context) (*Slot, error) {
	select {
	case slot := <-p.readyCh:
		p.mu.Lock()
		slot.State = SlotAssigned
		p.mu.Unlock()
		p.logger.Info("slot acquired", "slot", slot.ID, "name", slot.Name)
		return slot, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Release marks a slot for cleanup and re-warming. Non-blocking.
// Uses context.Background() internally so cleanup completes even on shutdown.
func (p *RunnerPool) Release(slot *Slot) {
	p.mu.Lock()
	slot.State = SlotCleaning
	p.mu.Unlock()

	log := p.logger.With("slot", slot.ID, "name", slot.Name)
	log.Info("releasing slot")

	// Cleanup with a background context so it completes even if parent cancelled.
	p.backend.Cleanup(context.Background(), slot.Name)

	p.mu.Lock()
	slot.Name = generateSlotName(slot.ID)
	slot.State = SlotIdle
	p.mu.Unlock()

	log.Info("slot released and ready for re-warm", "newName", slot.Name)
}

// Backend returns the underlying backend for callers that need direct access
// (e.g., the scheduler calling RunRunner).
func (p *RunnerPool) Backend() backend.Backend {
	return p.backend
}

func (p *RunnerPool) warmUpLoop(ctx context.Context, slot *Slot) {
	backoff := initialBackoff

	for {
		if ctx.Err() != nil {
			return
		}

		p.mu.Lock()
		if slot.State != SlotIdle {
			p.mu.Unlock()
			// Slot is in use or being cleaned up; wait and check again.
			select {
			case <-time.After(500 * time.Millisecond):
				continue
			case <-ctx.Done():
				return
			}
		}
		slot.State = SlotCreating
		name := slot.Name
		p.mu.Unlock()

		log := p.logger.With("slot", slot.ID, "name", name)
		log.Info("warming up slot")

		if err := p.backend.Prepare(ctx, name); err != nil {
			log.Error("warm-up failed", "err", err, "backoff", backoff)

			// Clean up any partial state.
			p.backend.Cleanup(context.Background(), name)

			p.mu.Lock()
			slot.State = SlotFailed
			p.mu.Unlock()

			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}

			backoff = min(backoff*2, maxBackoff)

			p.mu.Lock()
			slot.Name = generateSlotName(slot.ID)
			slot.State = SlotIdle
			p.mu.Unlock()
			continue
		}

		// Warm-up succeeded.
		backoff = initialBackoff

		p.mu.Lock()
		slot.State = SlotReady
		p.mu.Unlock()

		log.Info("slot ready")

		select {
		case p.readyCh <- slot:
			// Slot handed off to Acquire.
		case <-ctx.Done():
			// Shutting down; clean up the just-prepared slot.
			p.backend.Cleanup(context.Background(), name)
			return
		}
	}
}

func generateSlotName(slotID int) string {
	return fmt.Sprintf("efr-%d-%d", time.Now().Unix(), slotID)
}
