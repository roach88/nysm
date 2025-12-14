package testutil

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicClock_StartsAtZero(t *testing.T) {
	clock := NewDeterministicClock()
	assert.Equal(t, int64(0), clock.Current())
}

func TestDeterministicClock_NextIncrementsMonotonically(t *testing.T) {
	clock := NewDeterministicClock()

	// First call returns 1
	assert.Equal(t, int64(1), clock.Next())
	assert.Equal(t, int64(1), clock.Current())

	// Subsequent calls increment
	assert.Equal(t, int64(2), clock.Next())
	assert.Equal(t, int64(3), clock.Next())
	assert.Equal(t, int64(4), clock.Next())
	assert.Equal(t, int64(4), clock.Current())
}

func TestDeterministicClock_Reset(t *testing.T) {
	clock := NewDeterministicClock()

	// Advance clock
	clock.Next()
	clock.Next()
	clock.Next()
	assert.Equal(t, int64(3), clock.Current())

	// Reset
	clock.Reset()
	assert.Equal(t, int64(0), clock.Current())

	// First call after reset returns 1
	assert.Equal(t, int64(1), clock.Next())
}

func TestDeterministicClock_ThreadSafe(t *testing.T) {
	clock := NewDeterministicClock()
	const numGoroutines = 100
	const callsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make([][]int64, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		results[i] = make([]int64, callsPerGoroutine)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				results[idx][j] = clock.Next()
			}
		}(i)
	}

	wg.Wait()

	// Collect all values
	allValues := make(map[int64]bool)
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < callsPerGoroutine; j++ {
			val := results[i][j]
			require.False(t, allValues[val], "duplicate value %d", val)
			allValues[val] = true
		}
	}

	// Verify all values from 1 to numGoroutines*callsPerGoroutine are present
	expectedTotal := numGoroutines * callsPerGoroutine
	assert.Len(t, allValues, expectedTotal)
	for i := int64(1); i <= int64(expectedTotal); i++ {
		assert.True(t, allValues[i], "missing value %d", i)
	}
}

func TestDeterministicClock_Deterministic(t *testing.T) {
	// Run twice and verify same sequence
	clock1 := NewDeterministicClock()
	clock2 := NewDeterministicClock()

	for i := 0; i < 100; i++ {
		assert.Equal(t, clock1.Next(), clock2.Next())
	}
}
