package engine

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClock_NewClock(t *testing.T) {
	c := NewClock()
	assert.Equal(t, int64(0), c.Current(), "new clock should start at 0")
}

func TestClock_NewClockAt(t *testing.T) {
	c := NewClockAt(100)
	assert.Equal(t, int64(100), c.Current(), "clock should start at specified value")
}

func TestClock_Next_Incrementing(t *testing.T) {
	c := NewClock()

	// First call returns 1 (increments then returns)
	assert.Equal(t, int64(1), c.Next())
	assert.Equal(t, int64(2), c.Next())
	assert.Equal(t, int64(3), c.Next())

	// Current should reflect increments
	assert.Equal(t, int64(3), c.Current())
}

func TestClock_Next_Unique(t *testing.T) {
	c := NewClock()
	const iterations = 1000

	seen := make(map[int64]bool)
	for i := 0; i < iterations; i++ {
		seq := c.Next()
		assert.False(t, seen[seq], "seq %d generated twice", seq)
		seen[seq] = true
	}

	assert.Len(t, seen, iterations, "all seqs should be unique")
}

func TestClock_ThreadSafe(t *testing.T) {
	c := NewClock()
	const goroutines = 100
	const callsPerGoroutine = 100

	var wg sync.WaitGroup
	seqs := make(chan int64, goroutines*callsPerGoroutine)

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				seqs <- c.Next()
			}
		}()
	}

	wg.Wait()
	close(seqs)

	// Verify all seqs are unique
	seen := make(map[int64]bool)
	for seq := range seqs {
		assert.False(t, seen[seq], "seq %d generated twice", seq)
		seen[seq] = true
	}

	expected := goroutines * callsPerGoroutine
	assert.Len(t, seen, expected, "should have %d unique seqs", expected)
}

func TestClock_Current_DoesNotIncrement(t *testing.T) {
	c := NewClock()

	c.Next() // 1
	c.Next() // 2

	// Current should not change the value
	assert.Equal(t, int64(2), c.Current())
	assert.Equal(t, int64(2), c.Current())
	assert.Equal(t, int64(2), c.Current())
}
