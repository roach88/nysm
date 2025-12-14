package engine

import (
	"sync"

	"github.com/google/uuid"
)

// UUIDv7Generator generates time-sortable UUIDv7 flow tokens.
//
// UUIDv7 embeds a timestamp in the most significant bits, making tokens
// sortable by creation time. This is helpful for debugging and trace
// visualization.
//
// Uses github.com/google/uuid package for RFC 4122 compliant UUIDs.
//
// Thread-safety: UUIDv7Generator is stateless and safe for concurrent use.
type UUIDv7Generator struct{}

// Generate creates a new UUIDv7 and returns it as a hyphenated string.
//
// Format: "550e8400-e29b-41d4-a716-446655440000" (36 characters)
//
// Panics if UUID generation fails (should never happen in practice).
func (g UUIDv7Generator) Generate() string {
	return uuid.Must(uuid.NewV7()).String()
}

// FixedGenerator returns predetermined flow tokens for testing.
//
// This enables deterministic test execution and golden trace comparison.
// Tests can provide a known sequence of tokens and verify exact trace output.
//
// Thread-safety: FixedGenerator is safe for concurrent use via internal mutex.
type FixedGenerator struct {
	mu     sync.Mutex
	tokens []string
	idx    int
}

// NewFixedGenerator creates a generator that returns tokens in order.
//
// Example:
//
//	gen := NewFixedGenerator("flow-1", "flow-2", "flow-3")
//	gen.Generate() // "flow-1"
//	gen.Generate() // "flow-2"
//	gen.Generate() // "flow-3"
//	gen.Generate() // panic: all tokens exhausted
func NewFixedGenerator(tokens ...string) *FixedGenerator {
	return &FixedGenerator{
		tokens: tokens,
		idx:    0,
	}
}

// Generate returns the next predetermined token.
// Thread-safe: uses mutex to protect index access.
//
// Panics if all tokens have been consumed. This is a fail-fast approach
// to catch test misconfiguration (test tried to create more flows than expected).
func (g *FixedGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.idx >= len(g.tokens) {
		panic("FixedGenerator: all tokens exhausted")
	}
	token := g.tokens[g.idx]
	g.idx++
	return token
}
