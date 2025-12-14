package engine

import (
	"sync"

	"github.com/roach88/nysm/internal/ir"
)

// EventType distinguishes between event kinds.
type EventType int

const (
	// EventTypeInvocation represents an action invocation to process.
	EventTypeInvocation EventType = iota + 1
	// EventTypeCompletion represents an action completion to process.
	EventTypeCompletion
)

// Event wraps invocations and completions for the event queue.
type Event struct {
	Type       EventType
	Invocation *ir.Invocation
	Completion *ir.Completion
}

// eventQueue is a thread-safe FIFO queue for events.
//
// The queue is unbounded to allow cascading sync rule firings to enqueue
// arbitrarily many generated invocations without blocking.
//
// Thread-safety is provided for external enqueuing (e.g., HTTP handlers)
// while the Engine's Run loop dequeues. In practice, most usage is single-threaded.
//
// The queue uses a channel for signaling to enable context-aware waiting
// in the Run loop (prevents goroutine hangs on context cancellation).
type eventQueue struct {
	mu     sync.Mutex
	events []Event
	closed bool
	signal chan struct{} // Signals event availability (buffered, size 1)
}

// newEventQueue creates an empty event queue.
func newEventQueue() *eventQueue {
	return &eventQueue{
		events: make([]Event, 0, 64), // Pre-allocate for typical workloads
		signal: make(chan struct{}, 1),
	}
}

// Enqueue adds an event to the back of the queue.
// Thread-safe: may be called from any goroutine.
// Returns false if the queue is closed.
func (q *eventQueue) Enqueue(e Event) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return false
	}

	q.events = append(q.events, e)

	// Signal availability (non-blocking - buffer of 1 coalesces multiple signals)
	select {
	case q.signal <- struct{}{}:
	default:
	}

	return true
}

// Dequeue removes and returns the front event.
// Blocks until an event is available or the queue is closed.
// Returns (Event{}, false) if the queue is closed and empty.
//
// DEPRECATED: Use TryDequeue + Wait() for context-aware dequeuing.
// This method is retained for backward compatibility but may block
// indefinitely if context is not being checked.
func (q *eventQueue) Dequeue() (Event, bool) {
	for {
		// Try to dequeue first
		if e, ok := q.TryDequeue(); ok {
			return e, true
		}

		// Check if closed
		q.mu.Lock()
		if q.closed && len(q.events) == 0 {
			q.mu.Unlock()
			return Event{}, false
		}
		q.mu.Unlock()

		// Wait for signal
		<-q.signal
	}
}

// TryDequeue attempts to dequeue without blocking.
// Returns (Event{}, false) if queue is empty.
func (q *eventQueue) TryDequeue() (Event, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.events) == 0 {
		return Event{}, false
	}

	e := q.events[0]

	// CRITICAL: Nil out the slot to allow GC to collect the Event's pointers
	// (Invocation, Completion). Without this, the underlying array retains
	// references until reallocated, causing memory leaks under steady load.
	q.events[0] = Event{}

	// Fix memory retention: reset slice when empty
	if len(q.events) == 1 {
		// Last element - reset to empty slice with original capacity
		q.events = q.events[:0]
	} else {
		q.events = q.events[1:]
	}

	return e, true
}

// Wait returns a channel that signals when events may be available.
// Use with select for context-aware waiting:
//
//	select {
//	case <-ctx.Done():
//	    return ctx.Err()
//	case <-q.Wait():
//	    // Try TryDequeue
//	}
func (q *eventQueue) Wait() <-chan struct{} {
	return q.signal
}

// Len returns the current queue length.
func (q *eventQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.events)
}

// Close signals that no more events will be enqueued.
// Wakes any blocked waiters by closing the signal channel.
func (q *eventQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return // Already closed
	}

	q.closed = true
	close(q.signal) // Wakes all waiters
}
