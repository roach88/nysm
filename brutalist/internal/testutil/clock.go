package testutil

import "sync"

// DeterministicClock provides a thread-safe monotonic logical clock for tests.
//
// Unlike engine.Clock, DeterministicClock can be reset for test reuse.
// This enables the same test scenario to run multiple times with identical seq values.
//
// Thread-safety: All methods are safe for concurrent use via internal mutex.
type DeterministicClock struct {
	mu  sync.Mutex
	seq int64
}

// NewDeterministicClock creates a new deterministic clock starting at 0.
//
// The first call to Next() returns 1.
func NewDeterministicClock() *DeterministicClock {
	return &DeterministicClock{seq: 0}
}

// Next increments and returns the next sequence number.
//
// Thread-safe: uses mutex to protect seq access.
// Monotonic: always returns seq+1, never decreases.
func (c *DeterministicClock) Next() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq++
	return c.seq
}

// Current returns the current sequence number without incrementing.
//
// Thread-safe: uses mutex to protect seq access.
func (c *DeterministicClock) Current() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.seq
}

// Reset resets the clock to 0.
//
// Used for test reuse. After Reset(), the next call to Next() returns 1.
func (c *DeterministicClock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seq = 0
}
