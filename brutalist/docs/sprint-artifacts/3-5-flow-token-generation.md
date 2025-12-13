# Story 3.5: Flow Token Generation

Status: ready-for-dev

## Story

As a **developer invoking actions**,
I want **flow tokens generated for new requests**,
So that **all related records are correlated**.

## Acceptance Criteria

1. **FlowTokenGenerator interface in `internal/engine/flow.go`**
   ```go
   // FlowTokenGenerator generates unique flow tokens for request correlation.
   // All invocations and completions within a flow carry the same token.
   type FlowTokenGenerator interface {
       Generate() string
   }
   ```
   - Single method interface for flow token generation
   - Returns 36-character hyphenated UUID string (e.g., "550e8400-e29b-41d4-a716-446655440000")

2. **UUIDv7Generator for production use**
   ```go
   // UUIDv7Generator generates time-sortable UUIDv7 flow tokens.
   // Time-sortability helps with debugging and trace visualization.
   type UUIDv7Generator struct{}

   // Generate creates a new UUIDv7 and returns it as a hyphenated string.
   // Uses github.com/google/uuid package.
   func (g UUIDv7Generator) Generate() string {
       return uuid.Must(uuid.NewV7()).String()
   }
   ```
   - Zero-config struct (no fields needed)
   - Uses `github.com/google/uuid` package
   - Returns 36-char hyphenated UUID string format
   - Time-sortable for debugging (UUIDv7 embeds timestamp)
   - Panics on generation failure (uuid.Must) since this should never happen

3. **FixedGenerator for deterministic tests**
   ```go
   // FixedGenerator returns predetermined flow tokens for testing.
   // Enables deterministic test execution and golden trace comparison.
   type FixedGenerator struct {
       tokens []string
       idx    int
   }

   // NewFixedGenerator creates a generator that returns tokens in order.
   func NewFixedGenerator(tokens ...string) *FixedGenerator {
       return &FixedGenerator{
           tokens: tokens,
           idx:    0,
       }
   }

   // Generate returns the next predetermined token.
   // Panics if all tokens exhausted.
   func (g *FixedGenerator) Generate() string {
       if g.idx >= len(g.tokens) {
           panic("FixedGenerator: all tokens exhausted")
       }
       token := g.tokens[g.idx]
       g.idx++
       return token
   }
   ```
   - Constructor takes variadic tokens for convenience
   - Sequential token generation (no randomness)
   - Panics if tokens exhausted (fail-fast for test misconfiguration)
   - Used in golden tests for reproducible trace files

4. **Engine.NewFlow() creates new flow**
   ```go
   // NewFlow generates a new flow token and returns it.
   // This is the entry point for external requests starting new flows.
   func (e *Engine) NewFlow() string {
       return e.flowGen.Generate()
   }
   ```
   - Public method on Engine struct
   - Delegates to injected FlowTokenGenerator
   - Returns flow token as string
   - No side effects (doesn't write to store yet)

5. **Engine struct stores FlowTokenGenerator**
   - Update Engine struct in `internal/engine/engine.go`:
   ```go
   type Engine struct {
       store    *store.Store
       compiler queryir.Compiler
       flowGen  FlowTokenGenerator // Now a real field, not placeholder
       clock    *Clock
       specs    []ir.ConceptSpec
       syncs    []ir.SyncRule
       queue    *eventQueue
   }
   ```
   - Remove TODO comment from Story 3.1
   - flowGen field is now fully implemented

6. **Engine.New() accepts FlowTokenGenerator**
   ```go
   func New(
       store *store.Store,
       specs []ir.ConceptSpec,
       syncs []ir.SyncRule,
       flowGen FlowTokenGenerator,
   ) *Engine {
       return &Engine{
           store:   store,
           clock:   NewClock(),
           specs:   specs,
           syncs:   syncs,
           queue:   newEventQueue(),
           flowGen: flowGen,
       }
   }
   ```
   - Add flowGen parameter to constructor
   - Production code uses UUIDv7Generator{}
   - Test code uses NewFixedGenerator(...)

7. **Tests verify uniqueness and determinism**
   - UUIDv7Generator produces unique tokens (test 1000 generations)
   - UUIDv7 tokens are valid UUIDs (can parse with uuid.Parse)
   - UUIDv7 tokens are 36 characters (hyphenated format)
   - FixedGenerator returns tokens in order
   - FixedGenerator panics when exhausted
   - Engine.NewFlow() delegates to generator correctly

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-3.1** | Generate unique flow tokens for request scoping |
| **Flow Token Format** | 36-char hyphenated UUIDv7 string |
| **Library** | github.com/google/uuid |
| **Testability** | Injectable generator with FixedGenerator for tests |

## Tasks / Subtasks

- [ ] Task 1: Create flow.go with interfaces and types (AC: #1, #2, #3)
  - [ ] 1.1 Create `internal/engine/flow.go`
  - [ ] 1.2 Define FlowTokenGenerator interface
  - [ ] 1.3 Implement UUIDv7Generator struct
  - [ ] 1.4 Implement UUIDv7Generator.Generate() using uuid.NewV7()
  - [ ] 1.5 Implement FixedGenerator struct with tokens slice and idx
  - [ ] 1.6 Implement NewFixedGenerator constructor
  - [ ] 1.7 Implement FixedGenerator.Generate() with exhaustion panic

- [ ] Task 2: Integrate with Engine struct (AC: #4, #5, #6)
  - [ ] 2.1 Update Engine struct to store FlowTokenGenerator (remove TODO)
  - [ ] 2.2 Update Engine.New() to accept flowGen parameter
  - [ ] 2.3 Implement Engine.NewFlow() method
  - [ ] 2.4 Update existing Engine tests to pass flowGen parameter

- [ ] Task 3: Write comprehensive tests (AC: #7)
  - [ ] 3.1 Create `internal/engine/flow_test.go`
  - [ ] 3.2 Test UUIDv7Generator generates valid UUIDs
  - [ ] 3.3 Test UUIDv7Generator produces unique tokens (1000 iterations)
  - [ ] 3.4 Test UUIDv7 tokens are 36 characters
  - [ ] 3.5 Test FixedGenerator returns tokens in order
  - [ ] 3.6 Test FixedGenerator panics when exhausted
  - [ ] 3.7 Test Engine.NewFlow() delegates to generator
  - [ ] 3.8 Test concurrent NewFlow() calls (thread safety)

- [ ] Task 4: Add package dependency (AC: #2)
  - [ ] 4.1 Add `github.com/google/uuid` to go.mod
  - [ ] 4.2 Run `go mod tidy`

## Dev Notes

### Flow Token Architecture

```
┌─────────────────────────────────────────────────┐
│ External Request (user action, webhook, etc.)  │
└──────────────────┬──────────────────────────────┘
                   │
                   ▼
        ┌──────────────────────┐
        │ engine.NewFlow()     │
        │  ↓                   │
        │ flowGen.Generate()   │
        │  ↓                   │
        │ "550e8400-e29b-..."  │
        └──────────┬───────────┘
                   │
     ┌─────────────┴─────────────┐
     ▼                           ▼
┌─────────────┐         ┌─────────────┐
│ Invocation  │         │ Completion  │
│ (flow: X)   │────────▶│ (flow: X)   │
└─────────────┘         └─────────────┘
     │                           │
     │    Sync Rule Matches      │
     │◀──────────────────────────┘
     ▼
┌─────────────┐
│ Invocation  │
│ (flow: X)   │  (Same flow token propagates)
└─────────────┘
```

**Key Points:**
1. **One flow token per request** - NewFlow() called once per external request
2. **Token propagates** - All invocations/completions inherit parent's flow token
3. **Scoped matching** - Sync rules only match records with same flow token (Story 3.7)
4. **Time-sortable** - UUIDv7 embeds timestamp for easier debugging
5. **Deterministic tests** - FixedGenerator enables golden trace comparison

### Why UUIDv7?

UUIDv7 was chosen over UUIDv4 for several reasons:

1. **Time-sortable** - Tokens sort chronologically, making logs easier to read
2. **Debugging** - Can estimate when a flow started from its token
3. **Standard** - RFC 4122 compliant, widely supported
4. **Unique** - 122 bits of entropy (timestamp + random)
5. **Testable** - Injectable interface allows deterministic tests

**Format:** `{timestamp-48bits}-{version-12bits}-{random-62bits}`

Example: `01890a5d-c000-7000-8000-00000000000a`
- First 48 bits: Unix timestamp (milliseconds)
- Next 4 bits: Version (7)
- Remaining: Random data

### Implementation Details

**UUIDv7Generator**

```go
// internal/engine/flow.go
package engine

import (
    "github.com/google/uuid"
)

// FlowTokenGenerator generates unique flow tokens for request correlation.
//
// Every external request gets a unique flow token via NewFlow().
// All invocations and completions within that flow carry the same token.
//
// Sync rules (by default) only match records with the same flow token,
// preventing accidental joins across concurrent requests.
type FlowTokenGenerator interface {
    // Generate creates and returns a new flow token.
    // Returns a 36-character hyphenated UUID string.
    Generate() string
}

// UUIDv7Generator generates time-sortable UUIDv7 flow tokens.
//
// UUIDv7 embeds a timestamp in the most significant bits, making tokens
// sortable by creation time. This is helpful for debugging and trace
// visualization.
//
// Uses github.com/google/uuid package for RFC 4122 compliant UUIDs.
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
type FixedGenerator struct {
    tokens []string
    idx    int
}

// NewFixedGenerator creates a generator that returns tokens in order.
//
// Example:
//   gen := NewFixedGenerator("flow-1", "flow-2", "flow-3")
//   gen.Generate() // "flow-1"
//   gen.Generate() // "flow-2"
//   gen.Generate() // "flow-3"
//   gen.Generate() // panic: all tokens exhausted
func NewFixedGenerator(tokens ...string) *FixedGenerator {
    return &FixedGenerator{
        tokens: tokens,
        idx:    0,
    }
}

// Generate returns the next predetermined token.
//
// Panics if all tokens have been consumed. This is a fail-fast approach
// to catch test misconfiguration (test tried to create more flows than expected).
func (g *FixedGenerator) Generate() string {
    if g.idx >= len(g.tokens) {
        panic("FixedGenerator: all tokens exhausted")
    }
    token := g.tokens[g.idx]
    g.idx++
    return token
}
```

**Engine Integration**

```go
// internal/engine/engine.go

// Update Engine struct (remove placeholder comment from Story 3.1)
type Engine struct {
    store    *store.Store
    compiler queryir.Compiler      // TODO: Story 4.1 - Query IR compiler
    flowGen  FlowTokenGenerator    // Flow token generation (Story 3.5)
    clock    *Clock
    specs    []ir.ConceptSpec
    syncs    []ir.SyncRule
    queue    *eventQueue
}

// Update constructor signature
func New(
    store *store.Store,
    specs []ir.ConceptSpec,
    syncs []ir.SyncRule,
    flowGen FlowTokenGenerator,
) *Engine {
    return &Engine{
        store:   store,
        clock:   NewClock(),
        specs:   specs,
        syncs:   syncs,
        queue:   newEventQueue(),
        flowGen: flowGen,
    }
}

// NewFlow generates a new flow token and returns it.
//
// This is the entry point for external requests starting new flows.
// Each external request (user action, webhook, scheduled job) should
// call NewFlow() once to get a correlation token.
//
// Usage:
//   flowToken := engine.NewFlow()
//   inv := ir.Invocation{
//       ID:        idgen.Generate(),
//       ActionURI: "Cart.addItem",
//       FlowToken: flowToken,
//       Input:     ir.IRObject{"item_id": ir.IRString("product-123")},
//       Seq:       engine.clock.Next(),
//   }
//   engine.Enqueue(Event{Type: EventTypeInvocation, Data: &inv})
func (e *Engine) NewFlow() string {
    return e.flowGen.Generate()
}
```

### Test Examples

**Test UUIDv7 Validity and Uniqueness**

```go
// internal/engine/flow_test.go
package engine

import (
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

func TestEngine_NewFlow(t *testing.T) {
    // Use FixedGenerator for deterministic test
    flowGen := NewFixedGenerator("test-flow-1", "test-flow-2")

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil, flowGen)

    // First flow
    flow1 := engine.NewFlow()
    assert.Equal(t, "test-flow-1", flow1)

    // Second flow
    flow2 := engine.NewFlow()
    assert.Equal(t, "test-flow-2", flow2)
}

func TestEngine_NewFlow_WithUUIDv7(t *testing.T) {
    // Test with real UUIDv7 generator
    flowGen := UUIDv7Generator{}

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil, flowGen)

    flow := engine.NewFlow()

    // Verify it's a valid UUIDv7
    parsed, err := uuid.Parse(flow)
    require.NoError(t, err)
    assert.Equal(t, uuid.Version(7), parsed.Version())
}

func TestEngine_NewFlow_Concurrent(t *testing.T) {
    // Test thread safety of NewFlow()
    flowGen := UUIDv7Generator{}

    dir := t.TempDir()
    st, err := store.Open(dir + "/test.db")
    require.NoError(t, err)
    defer st.Close()

    engine := New(st, nil, nil, flowGen)

    const goroutines = 100
    tokens := make(chan string, goroutines)

    // Spawn goroutines calling NewFlow()
    for i := 0; i < goroutines; i++ {
        go func() {
            tokens <- engine.NewFlow()
        }()
    }

    // Collect all tokens
    seen := make(map[string]bool)
    for i := 0; i < goroutines; i++ {
        token := <-tokens
        require.False(t, seen[token], "duplicate token generated")
        seen[token] = true
    }

    assert.Equal(t, goroutines, len(seen))
}
```

### Integration with Future Stories

**Story 3.6: Flow Token Propagation**

When creating invocations, the flow token is propagated:

```go
// Pseudo-code for Story 3.6
func (e *Engine) InvokeAction(ctx context.Context, parentFlowToken string, action ir.ActionRef, input ir.IRObject) error {
    inv := ir.Invocation{
        ID:        e.idGen.Generate(),
        ActionURI: action,
        FlowToken: parentFlowToken, // Inherit parent's flow token
        Input:     input,
        Seq:       e.clock.Next(),
    }
    e.Enqueue(Event{Type: EventTypeInvocation, Data: &inv})
    return nil
}
```

**Story 3.7: Flow-Scoped Sync Matching**

When matching sync rules, only records with same flow token are considered:

```go
// Pseudo-code for Story 3.7
func (e *Engine) executeWhereClause(
    ctx context.Context,
    sync ir.SyncRule,
    flowToken string,
    whenBindings ir.IRObject,
) ([]ir.IRObject, error) {
    query := e.compiler.Compile(sync.Where)

    if sync.Scope == "flow" { // Default scope
        // Only match records with same flow token
        query = query.WithFilter(queryir.Equals("flow_token", flowToken))
    }

    return query.Execute(ctx, e.store)
}
```

### Usage Examples

**Production Setup**

```go
// main.go or initialization code
func main() {
    // Open store
    st, err := store.Open("nysm.db")
    if err != nil {
        log.Fatalf("failed to open store: %v", err)
    }
    defer st.Close()

    // Parse specs and syncs
    specs, syncs := loadFromCUE("specs/")

    // Create engine with UUIDv7 generator
    engine := engine.New(
        st,
        specs,
        syncs,
        engine.UUIDv7Generator{}, // Production generator
    )

    // Start engine
    ctx := context.Background()
    go engine.Run(ctx)

    // Handle external request
    flowToken := engine.NewFlow() // Generate new flow
    // ... create invocation with flowToken
}
```

**Test Setup**

```go
// engine_test.go or integration tests
func TestCheckoutWorkflow(t *testing.T) {
    // Use predetermined tokens for golden comparison
    flowGen := engine.NewFixedGenerator(
        "flow-checkout-1",
        "flow-checkout-2",
    )

    engine := engine.New(store, specs, syncs, flowGen)

    // Test knows exact flow tokens
    flow := engine.NewFlow()
    assert.Equal(t, "flow-checkout-1", flow)

    // ... rest of test with deterministic flow tokens
}
```

### File List

Files to create:

1. `internal/engine/flow.go` - FlowTokenGenerator interface and implementations
2. `internal/engine/flow_test.go` - Comprehensive tests

Files to modify:

1. `internal/engine/engine.go` - Update Engine struct, New() signature, add NewFlow() method
2. `internal/engine/engine_test.go` - Update tests to pass flowGen parameter
3. `go.mod` - Add `github.com/google/uuid` dependency

Files to reference (must exist from previous stories):

1. `internal/engine/engine.go` - Engine struct (Story 3.1)
2. `internal/store/store.go` - Store type (Story 2.1)
3. `internal/ir/types.go` - IR types (Story 1.1)

### Dependencies

**Go Module:**

```bash
go get github.com/google/uuid@latest
go mod tidy
```

The `github.com/google/uuid` package is the standard library for UUID generation in Go.
It's widely used, well-maintained, and provides RFC 4122 compliant UUIDs including UUIDv7.

### Story Completion Checklist

- [ ] `internal/engine/flow.go` created
- [ ] FlowTokenGenerator interface defined
- [ ] UUIDv7Generator implemented
- [ ] FixedGenerator implemented
- [ ] NewFixedGenerator constructor created
- [ ] Engine struct updated with flowGen field (comment removed)
- [ ] Engine.New() accepts FlowTokenGenerator parameter
- [ ] Engine.NewFlow() method implemented
- [ ] Existing Engine tests updated to pass flowGen
- [ ] `internal/engine/flow_test.go` created
- [ ] Test: UUIDv7 produces valid UUIDs
- [ ] Test: UUIDv7 produces unique tokens (1000 iterations)
- [ ] Test: UUIDv7 tokens are 36 characters
- [ ] Test: FixedGenerator returns tokens in order
- [ ] Test: FixedGenerator panics when exhausted
- [ ] Test: Engine.NewFlow() delegates correctly
- [ ] Test: Concurrent NewFlow() calls work (thread safety)
- [ ] `github.com/google/uuid` added to go.mod
- [ ] All tests pass (`go test ./internal/engine/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/engine` passes

### References

- [Source: docs/architecture.md#Flow Token Generation] - UUIDv7 decision and injectable design
- [Source: docs/architecture.md#Technology Stack] - github.com/google/uuid package
- [Source: docs/prd.md#FR-3.1] - Generate unique flow tokens requirement
- [Source: docs/epics.md#Story 3.5] - Story definition and acceptance criteria
- [Source: Story 3.1] - Engine struct foundation
- [RFC 4122: UUIDv7 Specification] - UUIDv7 format and semantics

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation

### Completion Notes

- Flow tokens are the foundation of request correlation in NYSM
- UUIDv7 chosen for time-sortability (helps debugging and trace visualization)
- Injectable FlowTokenGenerator enables deterministic tests (FixedGenerator)
- Flow tokens are 36-character hyphenated UUID strings
- Engine.NewFlow() is the entry point for external requests
- Story 3.6 will handle flow token propagation through invocations/completions
- Story 3.7 will implement flow-scoped sync matching
- Uses github.com/google/uuid for RFC 4122 compliant UUIDs
- FixedGenerator panics when exhausted (fail-fast for test misconfiguration)
- UUIDv7Generator.Generate() panics on failure (should never happen)
- Thread-safe: UUIDv7Generator has no mutable state
- Production code uses UUIDv7Generator{}, tests use NewFixedGenerator(...)
