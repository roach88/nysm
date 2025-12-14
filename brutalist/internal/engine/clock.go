package engine

import "sync/atomic"

// Clock implements CP-2: monotonic logical clock for event ordering.
//
// All events are stamped with a strictly increasing seq number from this clock.
// This ensures:
// - Deterministic ordering (no wall-clock race conditions)
// - Replay produces identical order
// - Causal relationships are explicit
//
// Thread-safety: Clock is safe for concurrent use (atomic operations).
// However, the Engine's single-writer design means only one goroutine
// typically calls Next().
type Clock struct {
	seq atomic.Int64
}

// NewClock creates a new clock starting at 0.
func NewClock() *Clock {
	return &Clock{}
}

// NewClockAt creates a new clock starting at a specific sequence number.
// Used for replay to resume from last known position.
func NewClockAt(start int64) *Clock {
	c := &Clock{}
	c.seq.Store(start)
	return c
}

// Next returns the next sequence number and increments the clock.
// Calls are linearizable - each call returns a unique, increasing value.
func (c *Clock) Next() int64 {
	return c.seq.Add(1)
}

// Current returns the current sequence number without incrementing.
// Useful for querying the clock's position (e.g., for checkpointing).
func (c *Clock) Current() int64 {
	return c.seq.Load()
}
