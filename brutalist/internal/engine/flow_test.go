package engine

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDv7Generator_ValidFormat(t *testing.T) {
	gen := UUIDv7Generator{}
	token := gen.Generate()

	// Verify 36 characters (hyphenated UUID format)
	assert.Equal(t, 36, len(token), "UUID should be 36 characters")

	// Verify it's a valid UUID
	parsed, err := uuid.Parse(token)
	require.NoError(t, err, "token should be valid UUID")

	// Verify it's UUIDv7 (version 7)
	assert.Equal(t, uuid.Version(7), parsed.Version())
}

func TestUUIDv7Generator_Uniqueness(t *testing.T) {
	gen := UUIDv7Generator{}
	const iterations = 1000

	tokens := make(map[string]bool, iterations)

	// Generate many tokens
	for i := 0; i < iterations; i++ {
		token := gen.Generate()
		require.False(t, tokens[token], "token %s generated twice", token)
		tokens[token] = true
	}

	assert.Equal(t, iterations, len(tokens), "all tokens should be unique")
}

func TestUUIDv7Generator_HyphenatedFormat(t *testing.T) {
	gen := UUIDv7Generator{}
	token := gen.Generate()

	// Verify hyphenated format: 8-4-4-4-12
	// Example: "550e8400-e29b-41d4-a716-446655440000"
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`, token)
}

func TestUUIDv7Generator_Concurrent(t *testing.T) {
	gen := UUIDv7Generator{}
	const goroutines = 100

	tokens := make(chan string, goroutines)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tokens <- gen.Generate()
		}()
	}

	wg.Wait()
	close(tokens)

	// Verify all tokens are unique
	seen := make(map[string]bool)
	for token := range tokens {
		require.False(t, seen[token], "duplicate token generated")
		seen[token] = true
	}

	assert.Equal(t, goroutines, len(seen))
}

func TestFixedGenerator_Sequential(t *testing.T) {
	gen := NewFixedGenerator("flow-1", "flow-2", "flow-3")

	assert.Equal(t, "flow-1", gen.Generate())
	assert.Equal(t, "flow-2", gen.Generate())
	assert.Equal(t, "flow-3", gen.Generate())
}

func TestFixedGenerator_PanicsWhenExhausted(t *testing.T) {
	gen := NewFixedGenerator("flow-1")

	// First call succeeds
	assert.Equal(t, "flow-1", gen.Generate())

	// Second call panics
	assert.Panics(t, func() {
		gen.Generate()
	}, "should panic when all tokens exhausted")
}

func TestFixedGenerator_EmptyTokens(t *testing.T) {
	gen := NewFixedGenerator()

	// Should panic immediately
	assert.Panics(t, func() {
		gen.Generate()
	}, "should panic when no tokens provided")
}

func TestFixedGenerator_SingleToken(t *testing.T) {
	gen := NewFixedGenerator("only-one")

	assert.Equal(t, "only-one", gen.Generate())

	// Second call panics
	assert.Panics(t, func() {
		gen.Generate()
	})
}

func TestEngine_NewFlow_WithFixedGenerator(t *testing.T) {
	s := setupTestStore(t)
	flowGen := NewFixedGenerator("test-flow-1", "test-flow-2")

	engine := New(s, nil, nil, flowGen)

	// First flow
	flow1 := engine.NewFlow()
	assert.Equal(t, "test-flow-1", flow1)

	// Second flow
	flow2 := engine.NewFlow()
	assert.Equal(t, "test-flow-2", flow2)
}

func TestEngine_NewFlow_WithUUIDv7(t *testing.T) {
	s := setupTestStore(t)
	flowGen := UUIDv7Generator{}

	engine := New(s, nil, nil, flowGen)

	flow := engine.NewFlow()

	// Verify it's a valid UUIDv7
	parsed, err := uuid.Parse(flow)
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), parsed.Version())
}
