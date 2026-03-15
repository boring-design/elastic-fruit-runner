package runnerpool

import "fmt"

// SlotState represents the current lifecycle phase of a runner slot.
type SlotState int

const (
	SlotIdle     SlotState = iota
	SlotCreating
	SlotReady
	SlotAssigned
	SlotRunning
	SlotCleaning
	SlotFailed
)

func (s SlotState) String() string {
	switch s {
	case SlotIdle:
		return "idle"
	case SlotCreating:
		return "creating"
	case SlotReady:
		return "ready"
	case SlotAssigned:
		return "assigned"
	case SlotRunning:
		return "running"
	case SlotCleaning:
		return "cleaning"
	case SlotFailed:
		return "failed"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// Slot represents a single runner execution slot in the pool.
type Slot struct {
	ID    int
	Name  string
	State SlotState
}
