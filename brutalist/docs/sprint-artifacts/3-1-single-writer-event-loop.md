# Story 3.1: Single-Writer Event Loop

Status: ready-for-dev

## Story

As a **developer building the engine**,
I want **a single-writer event loop for deterministic scheduling**,
So that **sync evaluation order is predictable and reproducible**.

## Acceptance Criteria

1. **Engine struct in `internal/engine/engine.go`**
   ```go
   type Engine struct {
       store    *store.Store
       compiler queryir.Compiler  // Placeholder interface for now
       flowGen  FlowTokenGenerator // Placeholder interface for now
       clock    *Clock
       specs    []ir.ConceptSpec
       syncs    []ir.SyncRule
       queue    *eventQueue
   }
   ```
   - All fields properly initialized
   - Clean separation of concerns

2. **Run() method implements single-writer event loop**
   ```go
   func (e *Engine) Run(ctx context.Context) error {
       for {
           select {
           case <-ctx.Done():
               return ctx.Err()
           default:
               event := e.queue.Dequeue()
               if event == nil {
                   // Wait for new events (blocking dequeue)
                   continue
               }
               if err := e.processEvent(ctx, event); err != nil {
                   return err
               }
           }
       }
   }
   ```
   - Context cancellation support
   - Graceful shutdown on ctx.Done()
   - Error propagation from processEvent

3. **Event queue implementation in `internal/engine/queue.go`**
   ```go
   type Event struct {
       Type EventType  // "invocation" or "completion"
       Data interface{} // *ir.Invocation or *ir.Completion
   }

   type EventType string
   const (
       EventTypeInvocation  EventType = "invocation"
       EventTypeCompletion  EventType = "completion"
   )

   type eventQueue struct {
       mu     sync.Mutex
       cond   *sync.Cond
       events []Event
   }

   func (q *eventQueue) Enqueue(event Event)
   func (q *eventQueue) Dequeue() *Event  // Blocks until event available or closed
   func (q *eventQueue) Close()
   ```
   - FIFO ordering guaranteed
   - Thread-safe for concurrent enqueue
   - Blocking dequeue with sync.Cond
   - Graceful close() support

4. **processEvent() stub in `internal/engine/engine.go`**
   ```go
   func (e *Engine) processEvent(ctx context.Context, event *Event) error {
       switch event.Type {
       case EventTypeInvocation:
           inv := event.Data.(*ir.Invocation)
           // TODO: Story 3.5 - Execute action
           _ = inv
           return nil
       case EventTypeCompletion:
           comp := event.Data.(*ir.Completion)
           // TODO: Story 3.3 - Match sync rules
           _ = comp
           return nil
       default:
           return fmt.Errorf("unknown event type: %v", event.Type)
       }
   }
   ```
   - Handles both invocation and completion events
   - Future story placeholders with TODO comments

5. **Clock implementation in `internal/engine/clock.go`**
   ```go
   type Clock struct {
       mu  sync.Mutex
       seq int64
   }

   func NewClock() *Clock {
       return &Clock{seq: 0}
   }

   func (c *Clock) Next() int64 {
       c.mu.Lock()
       defer c.mu.Unlock()
       c.seq++
       return c.seq
   }
   ```
   - Thread-safe monotonic counter
   - Starts at 1 (seq 0 reserved for initialization)

6. **Engine constructor in `internal/engine/engine.go`**
   ```go
   func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule) *Engine {
       return &Engine{
           store:  store,
           clock:  NewClock(),
           specs:  specs,
           syncs:  syncs,
           queue:  newEventQueue(),
       }
   }
   ```
   - Initializes all required fields
   - Creates fresh clock and queue

7. **Tests verify FIFO ordering and graceful shutdown**
   - Events processed in exact enqueue order
   - Context cancellation stops the loop cleanly
   - No goroutine leaks after shutdown

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **Concurrency Model** | Single writer, FIFO queue, declaration order sync eval |
| **CRITICAL-3** | Deterministic scheduling policy (declaration order) |
| **CP-2** | Logical clock (seq) for all event ordering |

## Tasks / Subtasks

- [ ] Task 1: Create engine package structure (AC: #1)
  - [ ] 1.1 Create `internal/engine/` directory
  - [ ] 1.2 Create `internal/engine/doc.go` with package documentation
  - [ ] 1.3 Create `internal/engine/engine.go` with Engine struct

- [ ] Task 2: Implement Clock (AC: #5)
  - [ ] 2.1 Create `internal/engine/clock.go`
  - [ ] 2.2 Implement Clock struct with Next() method
  - [ ] 2.3 Add thread-safety with mutex
  - [ ] 2.4 Write tests for Clock (concurrent access, monotonicity)

- [ ] Task 3: Implement event queue (AC: #3)
  - [ ] 3.1 Create `internal/engine/queue.go`
  - [ ] 3.2 Define Event and EventType types
  - [ ] 3.3 Implement eventQueue with FIFO semantics
  - [ ] 3.4 Implement Enqueue() (non-blocking, thread-safe)
  - [ ] 3.5 Implement Dequeue() (blocking with sync.Cond)
  - [ ] 3.6 Implement Close() for graceful shutdown
  - [ ] 3.7 Write tests for queue (FIFO order, blocking, close)

- [ ] Task 4: Implement Engine.New() constructor (AC: #6)
  - [ ] 4.1 Create New() function with parameters
  - [ ] 4.2 Initialize all Engine fields
  - [ ] 4.3 Return ready-to-use Engine instance
  - [ ] 4.4 Write tests for constructor (field initialization)

- [ ] Task 5: Implement Engine.Run() (AC: #2)
  - [ ] 5.1 Implement Run(ctx context.Context) error method
  - [ ] 5.2 Add select on ctx.Done() for graceful shutdown
  - [ ] 5.3 Implement event dequeue loop
  - [ ] 5.4 Call processEvent() for each event
  - [ ] 5.5 Propagate errors from processEvent
  - [ ] 5.6 Write tests for Run() (context cancellation, error propagation)

- [ ] Task 6: Implement processEvent() stub (AC: #4)
  - [ ] 6.1 Create processEvent(ctx, event) error method
  - [ ] 6.2 Add type switch on event.Type
  - [ ] 6.3 Handle EventTypeInvocation with TODO comment
  - [ ] 6.4 Handle EventTypeCompletion with TODO comment
  - [ ] 6.5 Return error for unknown event types
  - [ ] 6.6 Write tests for processEvent (type handling, unknown types)

- [ ] Task 7: Write comprehensive integration tests (AC: #7)
  - [ ] 7.1 Test FIFO ordering (enqueue 3 events, verify processing order)
  - [ ] 7.2 Test graceful shutdown (cancel context mid-processing)
  - [ ] 7.3 Test no goroutine leaks (use goleak)
  - [ ] 7.4 Test error propagation from processEvent
  - [ ] 7.5 Test empty queue behavior (blocking dequeue)

## Dev Notes

### Concurrency Model Architecture

```
                    ┌─────────────────────┐
   Invocations ───▶ │   Event Queue       │
                    │   (FIFO, ordered)   │
                    └──────────┬──────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │   Engine Loop       │
                    │   (single writer)   │
                    └──────────┬──────────┘
                               │
           ┌───────────────────┼───────────────────┐
           ▼                   ▼                   ▼
    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
    │ Process     │    │ Match       │    │ Execute     │
    │ Completion  │    │ Sync Rules  │    │ Where Query │
    └─────────────┘    └─────────────┘    └─────────────┘
           │                   │                   │
           └───────────────────┴───────────────────┘
                               │
                               ▼
                    ┌─────────────────────┐
                    │   SQLite Writer     │
                    │   (single conn)     │
                    └─────────────────────┘
```

**Critical Rules:**
1. **Single writer** - One goroutine writes to SQLite (Engine.Run())
2. **FIFO queue** - Invocations/completions processed in arrival order
3. **Sync evaluation order** - Rules evaluated in declaration order (Story 3.2)
4. **Binding order** - Query results processed in ORDER BY order (Story 4.x)
5. **No parallelism in core loop** - Determinism over throughput

**Action Execution (Future):**
- Actions may execute in parallel (I/O bound, external to engine)
- Completions enqueue back to the single-writer loop
- Engine sees completions in deterministic order

### Package Documentation

```go
// internal/engine/doc.go
// Package engine implements the NYSM reactive sync engine.
//
// The engine is the heart of NYSM - it receives invocations and completions,
// matches sync rules, executes queries, and generates follow-on invocations.
//
// ARCHITECTURE:
//
// Single-Writer Event Loop:
// The engine processes all events in a single goroutine for deterministic
// behavior. This ensures:
// - Predictable sync rule evaluation order
// - Reproducible event log on replay
// - Simple reasoning about causality
//
// Event Processing Flow:
// 1. Events enqueued to FIFO queue (invocations or completions)
// 2. Engine.Run() dequeues events one at a time
// 3. processEvent() routes to appropriate handler
// 4. Handler writes to SQLite (single writer)
// 5. Generated invocations enqueued back to queue
//
// The engine is designed for correctness and determinism, not throughput.
// External action execution may be parallelized, but the core evaluation
// loop is strictly single-threaded.
//
// CRITICAL PATTERNS:
//
// CP-2: Logical Clock
// All events stamped with monotonic seq counter from Clock.Next().
// NEVER use wall-clock timestamps for ordering.
//
// CRITICAL-3: Deterministic Scheduling
// Sync rules evaluated in declaration order.
// Query results processed in ORDER BY seq, id order.
// No randomness, no concurrency, no non-determinism.
package engine
```

### Engine Implementation

```go
// internal/engine/engine.go
package engine

import (
    "context"
    "fmt"

    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/queryir"
    "github.com/tyler/nysm/internal/store"
)

// Engine is the NYSM reactive sync engine.
// It processes invocations and completions in a deterministic event loop.
type Engine struct {
    store    *store.Store
    compiler queryir.Compiler      // TODO: Story 4.1 - Query IR compiler
    flowGen  FlowTokenGenerator    // TODO: Story 3.6 - Flow token generation
    clock    *Clock
    specs    []ir.ConceptSpec
    syncs    []ir.SyncRule
    queue    *eventQueue
}

// FlowTokenGenerator is a placeholder interface for flow token generation.
// TODO: Story 3.6 - Implement UUIDv7-based flow token generator
type FlowTokenGenerator interface {
    Generate() string
}

// New creates a new Engine instance.
//
// Parameters:
// - store: Durable SQLite store for event log
// - specs: Compiled concept specs (for validation)
// - syncs: Compiled sync rules (in declaration order)
//
// The engine is ready to use after construction. Call Run() to start
// the event processing loop.
func New(store *store.Store, specs []ir.ConceptSpec, syncs []ir.SyncRule) *Engine {
    return &Engine{
        store:  store,
        clock:  NewClock(),
        specs:  specs,
        syncs:  syncs,
        queue:  newEventQueue(),
    }
}

// Run starts the engine event processing loop.
//
// This is a blocking call that runs until:
// - The context is canceled (graceful shutdown)
// - processEvent returns an error (fatal error)
//
// The engine processes events in FIFO order. Only ONE goroutine writes
// to SQLite, ensuring deterministic replay.
//
// Usage:
//   ctx, cancel := context.WithCancel(context.Background())
//   defer cancel()
//   if err := engine.Run(ctx); err != nil && err != context.Canceled {
//       log.Fatalf("engine error: %v", err)
//   }
func (e *Engine) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            // Graceful shutdown - stop processing and return
            e.queue.Close()
            return ctx.Err()
        default:
            // Dequeue next event (blocks if queue empty)
            event := e.queue.Dequeue()
            if event == nil {
                // Queue closed, shutdown
                return nil
            }

            // Process event (writes to SQLite)
            if err := e.processEvent(ctx, event); err != nil {
                return fmt.Errorf("failed to process event: %w", err)
            }
        }
    }
}

// Enqueue adds an event to the processing queue.
// This is the external API for submitting invocations or completions.
//
// Thread-safe: Can be called from multiple goroutines.
func (e *Engine) Enqueue(event Event) {
    e.queue.Enqueue(event)
}

// processEvent routes events to the appropriate handler.
//
// This is where the engine decides what to do with each event:
// - Invocations: Execute action (Story 3.5)
// - Completions: Match sync rules and generate invocations (Story 3.3)
func (e *Engine) processEvent(ctx context.Context, event *Event) error {
    switch event.Type {
    case EventTypeInvocation:
        inv := event.Data.(*ir.Invocation)
        // TODO: Story 3.5 - Execute action via harness
        // This will call external action implementations and capture completions
        _ = inv
        return nil

    case EventTypeCompletion:
        comp := event.Data.(*ir.Completion)
        // TODO: Story 3.3 - Match sync rules via when-clause matcher
        // This will evaluate all sync rules in declaration order
        _ = comp
        return nil

    default:
        return fmt.Errorf("unknown event type: %v", event.Type)
    }
}
```

### Clock Implementation

```go
// internal/engine/clock.go
package engine

import "sync"

// Clock provides a thread-safe monotonic logical clock.
//
// Used to assign seq values to all invocations and completions.
// Implements CP-2: Logical clocks, never wall-clock timestamps.
//
// The clock is the source of truth for event ordering in NYSM.
// All replay, queries, and sync evaluation use seq values from this clock.
type Clock struct {
    mu  sync.Mutex
    seq int64
}

// NewClock creates a new logical clock starting at 0.
//
// The first call to Next() returns 1.
// Seq value 0 is reserved for "no event" or initialization.
func NewClock() *Clock {
    return &Clock{seq: 0}
}

// Next increments and returns the next sequence number.
//
// Thread-safe: Can be called from multiple goroutines.
// Monotonic: Always returns seq+1, never decreases.
//
// This is the ONLY way to generate seq values in NYSM.
func (c *Clock) Next() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.seq++
    return c.seq
}

// Current returns the current sequence number without incrementing.
//
// Thread-safe: Can be called from multiple goroutines.
// Used for testing and introspection.
func (c *Clock) Current() int64 {
    c.mu.Lock()
    defer c.mu.Unlock()
    return c.seq
}
```

### Event Queue Implementation

```go
// internal/engine/queue.go
package engine

import (
    "sync"

    "github.com/tyler/nysm/internal/ir"
)

// EventType represents the type of event in the queue.
type EventType string

const (
    // EventTypeInvocation represents an action invocation event
    EventTypeInvocation EventType = "invocation"

    // EventTypeCompletion represents an action completion event
    EventTypeCompletion EventType = "completion"
)

// Event represents a single event in the engine queue.
type Event struct {
    Type EventType   // "invocation" or "completion"
    Data interface{} // *ir.Invocation or *ir.Completion
}

// eventQueue is a thread-safe FIFO queue for engine events.
//
// The queue is the bridge between parallel action execution and the
// single-writer engine loop. Multiple goroutines may Enqueue(), but
// only the engine goroutine calls Dequeue().
//
// Dequeue() blocks when empty, using sync.Cond for efficient waiting.
type eventQueue struct {
    mu     sync.Mutex
    cond   *sync.Cond
    events []Event
    closed bool
}

// newEventQueue creates a new event queue.
func newEventQueue() *eventQueue {
    q := &eventQueue{
        events: make([]Event, 0, 16), // Pre-allocate small capacity
    }
    q.cond = sync.NewCond(&q.mu)
    return q
}

// Enqueue adds an event to the back of the queue.
//
// Thread-safe: Can be called from multiple goroutines.
// Non-blocking: Always returns immediately.
//
// If the queue is closed, this is a no-op (event discarded).
func (q *eventQueue) Enqueue(event Event) {
    q.mu.Lock()
    defer q.mu.Unlock()

    if q.closed {
        // Queue closed, discard event
        return
    }

    // Append to FIFO queue
    q.events = append(q.events, event)

    // Wake up any waiting Dequeue() call
    q.cond.Signal()
}

// Dequeue removes and returns the next event from the front of the queue.
//
// Blocks if the queue is empty, waiting for Enqueue() or Close().
// Returns nil if the queue is closed and empty.
//
// This should ONLY be called from the single engine goroutine.
func (q *eventQueue) Dequeue() *Event {
    q.mu.Lock()
    defer q.mu.Unlock()

    for {
        // If events available, return first one (FIFO)
        if len(q.events) > 0 {
            event := q.events[0]
            // NOTE: Nil out the slot to allow GC to reclaim memory.
            // Without this, the backing array retains references to dequeued events,
            // preventing garbage collection until the entire slice is replaced.
            q.events[0] = Event{} // Allow GC of dequeued event
            q.events = q.events[1:] // Shift queue
            return &event
        }

        // If closed and empty, return nil
        if q.closed {
            return nil
        }

        // Wait for Enqueue() or Close()
        q.cond.Wait()
    }
}

// Close marks the queue as closed.
//
// After Close():
// - Enqueue() becomes a no-op (events discarded)
// - Dequeue() returns nil after draining existing events
//
// This is used for graceful shutdown when context is canceled.
func (q *eventQueue) Close() {
    q.mu.Lock()
    defer q.mu.Unlock()

    q.closed = true
    q.cond.Broadcast() // Wake all waiting Dequeue() calls
}

// Len returns the current number of events in the queue.
//
// Thread-safe: Can be called from multiple goroutines.
// Used for testing and introspection.
func (q *eventQueue) Len() int {
    q.mu.Lock()
    defer q.mu.Unlock()
    return len(q.events)
}
```

### Test Examples

```go
// internal/engine/clock_test.go
package engine

import (
    "sync"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestClock_Next(t *testing.T) {
    clock := NewClock()

    // First call returns 1
    assert.Equal(t, int64(1), clock.Next())

    // Subsequent calls increment
    assert.Equal(t, int64(2), clock.Next())
    assert.Equal(t, int64(3), clock.Next())
}

func TestClock_Current(t *testing.T) {
    clock := NewClock()

    // Initial value is 0
    assert.Equal(t, int64(0), clock.Current())

    // After Next(), Current() reflects new value
    clock.Next()
    assert.Equal(t, int64(1), clock.Current())

    // Current() doesn't increment
    assert.Equal(t, int64(1), clock.Current())
}

func TestClock_Concurrent(t *testing.T) {
    clock := NewClock()
    const goroutines = 100
    const callsPerGoroutine = 100

    var wg sync.WaitGroup
    results := make([]int64, goroutines*callsPerGoroutine)

    // Launch goroutines
    for i := 0; i < goroutines; i++ {
        wg.Add(1)
        go func(offset int) {
            defer wg.Done()
            for j := 0; j < callsPerGoroutine; j++ {
                results[offset*callsPerGoroutine+j] = clock.Next()
            }
        }(i)
    }

    wg.Wait()

    // Verify all values are unique and in range [1, goroutines*callsPerGoroutine]
    seen := make(map[int64]bool)
    for _, seq := range results {
        require.False(t, seen[seq], "seq %d generated multiple times", seq)
        require.GreaterOrEqual(t, seq, int64(1))
        require.LessOrEqual(t, seq, int64(goroutines*callsPerGoroutine))
        seen[seq] = true
    }

    assert.Equal(t, goroutines*callsPerGoroutine, len(seen))
}
```

```go
// internal/engine/queue_test.go
package engine

import (
    "sync"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/ir"
)

func TestQueue_FIFO(t *testing.T) {
    q := newEventQueue()

    // Enqueue 3 events
    e1 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv1"}}
    e2 := Event{Type: EventTypeCompletion, Data: &ir.Completion{ID: "comp1"}}
    e3 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv2"}}

    q.Enqueue(e1)
    q.Enqueue(e2)
    q.Enqueue(e3)

    // Dequeue in FIFO order
    assert.Equal(t, EventTypeInvocation, q.Dequeue().Type)
    assert.Equal(t, EventTypeCompletion, q.Dequeue().Type)
    assert.Equal(t, EventTypeInvocation, q.Dequeue().Type)
}

func TestQueue_BlockingDequeue(t *testing.T) {
    q := newEventQueue()

    var wg sync.WaitGroup
    var event *Event

    // Start dequeue in goroutine (will block)
    wg.Add(1)
    go func() {
        defer wg.Done()
        event = q.Dequeue()
    }()

    // Give goroutine time to start waiting
    time.Sleep(10 * time.Millisecond)

    // Enqueue event - should wake up dequeue
    e1 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv1"}}
    q.Enqueue(e1)

    // Wait for dequeue to complete
    wg.Wait()

    require.NotNil(t, event)
    assert.Equal(t, EventTypeInvocation, event.Type)
}

func TestQueue_Close(t *testing.T) {
    q := newEventQueue()

    // Enqueue one event
    e1 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv1"}}
    q.Enqueue(e1)

    // Close queue
    q.Close()

    // Dequeue existing event (still works)
    event := q.Dequeue()
    require.NotNil(t, event)
    assert.Equal(t, EventTypeInvocation, event.Type)

    // Dequeue again returns nil (closed and empty)
    assert.Nil(t, q.Dequeue())

    // Enqueue after close is no-op
    e2 := Event{Type: EventTypeCompletion, Data: &ir.Completion{ID: "comp1"}}
    q.Enqueue(e2)
    assert.Equal(t, 0, q.Len())
}

func TestQueue_Len(t *testing.T) {
    q := newEventQueue()

    assert.Equal(t, 0, q.Len())

    e1 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv1"}}
    q.Enqueue(e1)
    assert.Equal(t, 1, q.Len())

    e2 := Event{Type: EventTypeCompletion, Data: &ir.Completion{ID: "comp1"}}
    q.Enqueue(e2)
    assert.Equal(t, 2, q.Len())

    q.Dequeue()
    assert.Equal(t, 1, q.Len())

    q.Dequeue()
    assert.Equal(t, 0, q.Len())
}

func TestQueue_Concurrent(t *testing.T) {
    q := newEventQueue()
    const producers = 10
    const eventsPerProducer = 100

    var wg sync.WaitGroup

    // Start producers
    for i := 0; i < producers; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            for j := 0; j < eventsPerProducer; j++ {
                e := Event{
                    Type: EventTypeInvocation,
                    Data: &ir.Invocation{ID: "inv"},
                }
                q.Enqueue(e)
            }
        }(i)
    }

    wg.Wait()

    // Verify all events enqueued
    assert.Equal(t, producers*eventsPerProducer, q.Len())

    // Dequeue all events
    for i := 0; i < producers*eventsPerProducer; i++ {
        event := q.Dequeue()
        require.NotNil(t, event)
        assert.Equal(t, EventTypeInvocation, event.Type)
    }

    assert.Equal(t, 0, q.Len())
}
```

```go
// internal/engine/engine_test.go
package engine

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/store"
    "go.uber.org/goleak"
)

func TestEngine_New(t *testing.T) {
    defer goleak.VerifyNone(t)

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    specs := []ir.ConceptSpec{
        {Name: "Cart", Purpose: "Shopping cart"},
    }
    syncs := []ir.SyncRule{
        {ID: "sync1", Scope: "flow"},
    }

    engine := New(st, specs, syncs)

    assert.NotNil(t, engine.store)
    assert.NotNil(t, engine.clock)
    assert.NotNil(t, engine.queue)
    assert.Equal(t, specs, engine.specs)
    assert.Equal(t, syncs, engine.syncs)
}

func TestEngine_Run_GracefulShutdown(t *testing.T) {
    defer goleak.VerifyNone(t)

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx, cancel := context.WithCancel(context.Background())

    // Start engine in goroutine
    errChan := make(chan error, 1)
    go func() {
        errChan <- engine.Run(ctx)
    }()

    // Give engine time to start
    time.Sleep(10 * time.Millisecond)

    // Cancel context
    cancel()

    // Wait for engine to stop
    err = <-errChan
    assert.ErrorIs(t, err, context.Canceled)
}

func TestEngine_Run_ProcessesEventsInOrder(t *testing.T) {
    defer goleak.VerifyNone(t)

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Enqueue 3 events
    e1 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv1"}}
    e2 := Event{Type: EventTypeCompletion, Data: &ir.Completion{ID: "comp1"}}
    e3 := Event{Type: EventTypeInvocation, Data: &ir.Invocation{ID: "inv2"}}

    engine.Enqueue(e1)
    engine.Enqueue(e2)
    engine.Enqueue(e3)

    // Start engine in goroutine
    go func() {
        _ = engine.Run(ctx)
    }()

    // Give engine time to process events
    time.Sleep(50 * time.Millisecond)

    // Verify queue drained
    assert.Equal(t, 0, engine.queue.Len())

    cancel()
}

func TestEngine_ProcessEvent_UnknownType(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx := context.Background()
    event := &Event{Type: "unknown", Data: nil}

    err = engine.processEvent(ctx, event)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "unknown event type")
}

func TestEngine_ProcessEvent_Invocation(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx := context.Background()
    inv := &ir.Invocation{ID: "inv1"}
    event := &Event{Type: EventTypeInvocation, Data: inv}

    // Should not error (stub implementation)
    err = engine.processEvent(ctx, event)
    assert.NoError(t, err)
}

func TestEngine_ProcessEvent_Completion(t *testing.T) {
    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil)

    ctx := context.Background()
    comp := &ir.Completion{ID: "comp1"}
    event := &Event{Type: EventTypeCompletion, Data: comp}

    // Should not error (stub implementation)
    err = engine.processEvent(ctx, event)
    assert.NoError(t, err)
}
```

### File List

Files to create:

1. `internal/engine/doc.go` - Package documentation
2. `internal/engine/engine.go` - Engine struct, New(), Run(), processEvent()
3. `internal/engine/clock.go` - Clock struct with Next() and Current()
4. `internal/engine/queue.go` - Event queue with FIFO semantics
5. `internal/engine/clock_test.go` - Clock tests (monotonicity, concurrency)
6. `internal/engine/queue_test.go` - Queue tests (FIFO, blocking, close)
7. `internal/engine/engine_test.go` - Engine tests (shutdown, ordering, event processing)

### Placeholder Interfaces

These interfaces are defined as placeholders and will be implemented in future stories:

```go
// queryir.Compiler - Story 4.1
// Compiles where-clauses to SQL queries
type Compiler interface {
    CompileWhere(where ir.WhereClause) (string, error)
}

// FlowTokenGenerator - Story 3.6
// Generates UUIDv7 flow tokens
type FlowTokenGenerator interface {
    Generate() string
}
```

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for ir.* types
- Story 2.1 (SQLite Store Initialization) - Required for store.Store
- Story 2.2 (Event Log Schema) - Required for store operations

**Enables:**
- Story 3.2 (Sync Rule Registration and Declaration Order) - Uses Engine.syncs field
- Story 3.3 (When-Clause Matching) - Implements processEvent for completions
- Story 3.4 (Output Case Matching) - Extends when-clause matching
- Story 3.5 (Invocation Execution) - Implements processEvent for invocations
- Story 3.6 (Flow Token Generation) - Implements FlowTokenGenerator interface

**Note:** This story establishes the single-writer event loop foundation. Future stories will fill in the TODO stubs in processEvent().

### Story Completion Checklist

- [ ] `internal/engine/` directory created
- [ ] `internal/engine/doc.go` written with package documentation
- [ ] `internal/engine/clock.go` implements Clock with Next() and Current()
- [ ] `internal/engine/queue.go` implements eventQueue with FIFO semantics
- [ ] `internal/engine/engine.go` implements Engine struct and methods
- [ ] Engine.New() initializes all fields correctly
- [ ] Engine.Run() implements single-writer event loop with context cancellation
- [ ] processEvent() stub handles both invocation and completion events
- [ ] All tests pass (`go test ./internal/engine/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/engine` passes
- [ ] Clock tests verify monotonicity and thread-safety
- [ ] Queue tests verify FIFO ordering and blocking dequeue
- [ ] Engine tests verify graceful shutdown and no goroutine leaks (goleak)
- [ ] Integration test verifies events processed in exact enqueue order

### References

- [Source: docs/architecture.md#Concurrency Model] - Single-writer event loop design
- [Source: docs/architecture.md#Technology Stack] - Go 1.25, SQLite
- [Source: docs/architecture.md#CP-2] - Logical clocks (seq), not timestamps
- [Source: docs/architecture.md#CRITICAL-3] - Deterministic scheduling policy
- [Source: docs/epics.md#Story 3.1] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-5.1] - Durable engine requirements
- [Source: docs/prd.md#NFR-2.1] - Deterministic replay requirement

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow

### Completion Notes

- Foundation story for Epic 3 - all sync engine stories depend on this event loop
- Single-writer design ensures deterministic replay (CRITICAL-3)
- Clock provides monotonic seq values for all events (CP-2)
- Event queue uses sync.Cond for efficient blocking dequeue
- processEvent() stub has TODO comments for future stories
- Graceful shutdown via context cancellation
- Tests use goleak to verify no goroutine leaks
- FlowTokenGenerator and queryir.Compiler are placeholder interfaces
- Engine.Run() blocks until context canceled or fatal error
- Next stories (3.2-3.6) will implement TODO stubs and add functionality
