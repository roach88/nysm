package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

func TestEventQueue_EnqueueDequeue(t *testing.T) {
	q := newEventQueue()

	inv := &ir.Invocation{ID: "inv-1"}
	event := Event{Type: EventTypeInvocation, Invocation: inv}

	ok := q.Enqueue(event)
	require.True(t, ok, "enqueue should succeed")

	got, ok := q.TryDequeue()
	require.True(t, ok, "dequeue should succeed")
	assert.Equal(t, EventTypeInvocation, got.Type)
	assert.Equal(t, "inv-1", got.Invocation.ID)
}

func TestEventQueue_FIFO(t *testing.T) {
	q := newEventQueue()

	// Enqueue 3 events
	for i := 1; i <= 3; i++ {
		inv := &ir.Invocation{ID: string(rune('A' + i - 1))}
		q.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})
	}

	// Dequeue in order
	e1, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "A", e1.Invocation.ID)

	e2, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "B", e2.Invocation.ID)

	e3, ok := q.TryDequeue()
	require.True(t, ok)
	assert.Equal(t, "C", e3.Invocation.ID)
}

func TestEventQueue_TryDequeue_Empty(t *testing.T) {
	q := newEventQueue()

	_, ok := q.TryDequeue()
	assert.False(t, ok, "dequeue from empty queue should return false")
}

func TestEventQueue_Dequeue_BlocksUntilAvailable(t *testing.T) {
	q := newEventQueue()

	done := make(chan Event)

	go func() {
		e, ok := q.Dequeue()
		if ok {
			done <- e
		}
	}()

	// Give goroutine time to block
	time.Sleep(10 * time.Millisecond)

	// Enqueue an event
	inv := &ir.Invocation{ID: "inv-blocking"}
	q.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})

	// Should receive the event
	select {
	case e := <-done:
		assert.Equal(t, "inv-blocking", e.Invocation.ID)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("dequeue did not unblock")
	}
}

func TestEventQueue_Close_UnblocksDequeue(t *testing.T) {
	q := newEventQueue()

	done := make(chan bool)

	go func() {
		_, ok := q.Dequeue()
		done <- ok
	}()

	// Give goroutine time to block
	time.Sleep(10 * time.Millisecond)

	// Close the queue
	q.Close()

	// Should unblock with false
	select {
	case ok := <-done:
		assert.False(t, ok, "dequeue after close should return false")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("dequeue did not unblock after close")
	}
}

func TestEventQueue_Enqueue_AfterClose(t *testing.T) {
	q := newEventQueue()
	q.Close()

	inv := &ir.Invocation{ID: "inv-after-close"}
	ok := q.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})
	assert.False(t, ok, "enqueue after close should return false")
}

func TestEventQueue_Len(t *testing.T) {
	q := newEventQueue()

	assert.Equal(t, 0, q.Len())

	q.Enqueue(Event{Type: EventTypeInvocation, Invocation: &ir.Invocation{ID: "1"}})
	assert.Equal(t, 1, q.Len())

	q.Enqueue(Event{Type: EventTypeInvocation, Invocation: &ir.Invocation{ID: "2"}})
	assert.Equal(t, 2, q.Len())

	q.TryDequeue()
	assert.Equal(t, 1, q.Len())

	q.TryDequeue()
	assert.Equal(t, 0, q.Len())
}

func TestEventQueue_ThreadSafe(t *testing.T) {
	q := newEventQueue()

	const producers = 10
	const eventsPerProducer = 100

	var wg sync.WaitGroup

	// Start producers
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for i := 0; i < eventsPerProducer; i++ {
				inv := &ir.Invocation{ID: string(rune(producerID*1000 + i))}
				q.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})
			}
		}(p)
	}

	// Start consumer
	received := make([]Event, 0, producers*eventsPerProducer)
	var mu sync.Mutex

	consumerDone := make(chan struct{})
	go func() {
		for {
			e, ok := q.TryDequeue()
			if !ok {
				// Queue might be temporarily empty
				time.Sleep(1 * time.Millisecond)
				continue
			}
			mu.Lock()
			received = append(received, e)
			if len(received) >= producers*eventsPerProducer {
				mu.Unlock()
				break
			}
			mu.Unlock()
		}
		close(consumerDone)
	}()

	// Wait for all producers
	wg.Wait()

	// Wait for consumer to finish
	select {
	case <-consumerDone:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatalf("consumer timeout: received %d events", len(received))
	}

	assert.Len(t, received, producers*eventsPerProducer)
}
