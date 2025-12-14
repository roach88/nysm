package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFixedFlowGenerator_ReturnsSameToken(t *testing.T) {
	gen := NewFixedFlowGenerator("test-flow-123")

	// Multiple calls return same token
	assert.Equal(t, "test-flow-123", gen.Generate())
	assert.Equal(t, "test-flow-123", gen.Generate())
	assert.Equal(t, "test-flow-123", gen.Generate())
}

func TestFixedFlowGenerator_EmptyTokenDefault(t *testing.T) {
	gen := NewFixedFlowGenerator("")

	// Empty token uses default
	assert.Equal(t, "test-flow-default", gen.Generate())
}

func TestFixedFlowGenerator_CustomToken(t *testing.T) {
	gen := NewFixedFlowGenerator("01234567-89ab-cdef-0123-456789abcdef")

	// Returns custom token
	assert.Equal(t, "01234567-89ab-cdef-0123-456789abcdef", gen.Generate())
}

func TestFixedFlowGenerator_ThreadSafe(t *testing.T) {
	gen := NewFixedFlowGenerator("thread-safe-token")

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				token := gen.Generate()
				assert.Equal(t, "thread-safe-token", token)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
