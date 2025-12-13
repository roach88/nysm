# Story 1.1: Project Initialization & IR Type Definitions

Status: ready-for-dev

## Story

As a **developer building NYSM**,
I want **the project structure and core IR types defined**,
So that **I have a foundation to build all other components on**.

## Acceptance Criteria

1. **Project structure matches Architecture specification**
   - Go module initialized with `github.com/tyler/nysm`
   - Directory structure follows Architecture "Complete Project Directory Structure"
   - All internal packages created with stub files

2. **go.mod contains required dependencies**
   ```go
   module github.com/tyler/nysm
   go 1.25
   require (
       cuelang.org/go v0.15.1
       github.com/mattn/go-sqlite3 v1.14.32  // Requires CGO
       github.com/spf13/cobra v1.10.2
       github.com/google/uuid v1.6.0         // UUIDv7 flow tokens
       github.com/stretchr/testify v1.11.1
       github.com/google/go-cmp v0.7.0
       github.com/sebdah/goldie/v2 v2.8.0
   )
   ```

3. **Core IR types defined in `internal/ir/types.go`**
   - All types per Architecture "Core IR Types" section
   - `SecurityContext` on BOTH `Invocation` AND `Completion` (CP-6)
   - `SyncFiring` and `ProvenanceEdge` are store-layer types (not content-addressed)
   - **NO float64 types anywhere** - use int64 for numbers (CP-5)
   - `ActionSig` includes `Requires []string` for authz hooks

4. **IRValue sealed interface in `internal/ir/value.go`** (CP-5)
   - `IRString`, `IRInt`, `IRBool`, `IRArray`, `IRObject` only
   - NO `IRFloat` - floats forbidden in IR
   - `IRObject` iteration uses sorted keys for determinism

5. **JSON tags use snake_case per IR Field Names convention**
   - All struct fields have `json:"field_name"` tags
   - Verify via test that marshaling produces snake_case

6. **`internal/ir/` has NO dependencies on other internal packages**
   - IR is foundational; all packages import ir; ir imports nothing internal
   - Verify with `go list -f '{{.Imports}}' ./internal/ir`

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-1** | Binding-level idempotency: `UNIQUE(completion_id, sync_id, binding_hash)` |
| **CP-2** | Logical clocks (`seq`), NO wall-clock timestamps ever |
| **CP-3** | RFC 8785 UTF-16 code unit key ordering (not UTF-8 bytes) |
| **CP-5** | NO floats in IRValue - only string, int64, bool, array, object |
| **CP-6** | SecurityContext always present (non-pointer) on Invocation AND Completion |

## Tasks / Subtasks

- [ ] Task 1: Initialize Go module and project files (AC: #1, #2)
  - [ ] 1.1 Run `go mod init github.com/tyler/nysm`
  - [ ] 1.2 Add all required dependencies to go.mod
  - [ ] 1.3 Run `go mod tidy` to validate
  - [ ] 1.4 Create `.gitignore` (nysm, *.db, testdata/golden/*.golden)
  - [ ] 1.5 Create `README.md` stub linking to docs/
  - [ ] 1.6 Create `.golangci.yml` linter config

- [ ] Task 2: Create directory structure (AC: #1)
  - [ ] 2.1 Create core packages: `cmd/nysm/`, `internal/{ir,compiler,store,engine,queryir,querysql,harness,cli,testutil}/`
  - [ ] 2.2 Create test directories: `testdata/{scenarios,golden,fixtures/rfc8785}/`
  - [ ] 2.3 Create specs directory: `specs/`
  - [ ] 2.4 Add stub files to each internal package

- [ ] Task 3: Create IR types (AC: #3, #5, #6)
  - [ ] 3.1 Create `internal/ir/doc.go` - package documentation
  - [ ] 3.2 Create `internal/ir/types.go` - ConceptSpec, ActionSig, SyncRule, Invocation, Completion, SecurityContext
  - [ ] 3.3 Create `internal/ir/clause.go` - WhenClause, WhereClause, ThenClause
  - [ ] 3.4 Create `internal/ir/refs.go` - ActionRef, ConceptRef
  - [ ] 3.5 Create `internal/ir/store_types.go` - SyncFiring, ProvenanceEdge (store-layer, not IR)
  - [ ] 3.6 Create `internal/ir/version.go` - IRVersion, EngineVersion constants

- [ ] Task 4: Create IRValue type system (AC: #4)
  - [ ] 4.1 Create `internal/ir/value.go` - sealed IRValue interface
  - [ ] 4.2 Implement IRString, IRInt, IRBool, IRArray, IRObject
  - [ ] 4.3 Add deterministic iteration helper for IRObject

- [ ] Task 5: Verify and test (AC: #5, #6)
  - [ ] 5.1 Create `internal/ir/types_test.go` - instantiation + JSON marshaling tests
  - [ ] 5.2 Create `internal/ir/value_test.go` - IRValue type tests
  - [ ] 5.3 Verify `go build ./...` succeeds
  - [ ] 5.4 Verify `go vet ./...` passes
  - [ ] 5.5 Verify ir has no internal deps: `go list -f '{{.Imports}}' ./internal/ir`

## Dev Notes

### Critical Pattern Details

**CP-2: Logical Identity and Time**
```go
// CORRECT: Use logical clock (seq), NEVER wall-clock timestamps
type Invocation struct {
    Seq int64 `json:"seq"` // Monotonic per-engine counter
}

// WRONG - NEVER DO THIS:
type Invocation struct {
    CreatedAt time.Time `json:"created_at"` // ❌ Breaks deterministic replay
}
```

**CP-3: RFC 8785 UTF-16 Key Ordering**
```go
// CRITICAL: RFC 8785 uses UTF-16 code units, NOT UTF-8 bytes
// Go's sort.Strings uses UTF-8 which produces DIFFERENT order
func compareKeysRFC8785(a, b string) int {
    ar, br := []rune(a), []rune(b)
    for i := 0; i < len(ar) && i < len(br); i++ {
        if ar[i] != br[i] {
            return int(ar[i]) - int(br[i]) // UTF-16 comparison
        }
    }
    return len(ar) - len(br)
}

// WRONG: sort.Strings(keys) // ❌ UTF-8 order differs from UTF-16
```

**CP-5: Constrained Value Types (No Floats)**
```go
// internal/ir/value.go - Sealed interface pattern

type IRValue interface {
    irValue() // Sealed - only these types implement it
}

type IRString string
func (IRString) irValue() {}

type IRInt int64
func (IRInt) irValue() {}

type IRBool bool
func (IRBool) irValue() {}

type IRArray []IRValue
func (IRArray) irValue() {}

type IRObject map[string]IRValue
func (IRObject) irValue() {}

// NO IRFloat - floats are FORBIDDEN in IR (breaks determinism)

// Deterministic iteration (REQUIRED for all map operations)
func (obj IRObject) SortedKeys() []string {
    keys := make([]string, 0, len(obj))
    for k := range obj {
        keys = append(keys, k)
    }
    slices.SortFunc(keys, compareKeysRFC8785) // UTF-16 order
    return keys
}
```

**CP-6: Security Context on ALL Records**
```go
// SecurityContext MUST be on BOTH Invocation AND Completion
// ALWAYS non-pointer, NEVER nil

type Invocation struct {
    // ...
    SecurityContext SecurityContext `json:"security_context"` // ✓ Always present
}

type Completion struct {
    // ...
    SecurityContext SecurityContext `json:"security_context"` // ✓ Always present
}

// WRONG - pointer allows nil:
SecurityContext *SecurityContext `json:"security_context,omitempty"` // ❌ NEVER
```

**CP-1: Binding-Level Idempotency**
```go
// Binding hash enables per-binding idempotency
// MUST use canonical JSON with domain separation
// Returns error if bindings cannot be canonically marshaled.

func BindingHash(bindings IRObject) (string, error) {
    canonical, err := MarshalCanonical(bindings)
    if err != nil {
        return "", fmt.Errorf("BindingHash: failed to marshal: %w", err)
    }
    return hashWithDomain("nysm/binding/v1", canonical), nil
}

// MustBindingHash is like BindingHash but panics on error.
// Use only in tests or when inputs are known to be valid.
func MustBindingHash(bindings IRObject) string {
    hash, err := BindingHash(bindings)
    if err != nil {
        panic(err)
    }
    return hash
}

// Domain separation format: {domain}\x00{canonical_json_bytes}
func hashWithDomain(domain string, data []byte) string {
    h := sha256.New()
    h.Write([]byte(domain))
    h.Write([]byte{0x00}) // Null separator
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}

// Domain prefixes:
// "nysm/invocation/v1" - Invocation IDs
// "nysm/completion/v1" - Completion IDs
// "nysm/binding/v1"    - Binding hashes
```

### Type Definitions

**Core IR Types (internal/ir/types.go):**
```go
// Package ir provides canonical intermediate representation types for NYSM.
// This package contains type definitions only. All other internal packages
// import ir; ir imports nothing internal.

// ConceptSpec represents a compiled concept definition
type ConceptSpec struct {
    Name                  string                 `json:"name"`
    Purpose               string                 `json:"purpose"`
    StateSchema           []StateSchema          `json:"state_schema"`
    Actions               []ActionSig            `json:"actions"`
    OperationalPrinciples []OperationalPrinciple `json:"operational_principles"`
}

// ActionSig represents an action signature with typed inputs/outputs
type ActionSig struct {
    Name     string       `json:"name"`
    Args     []NamedArg   `json:"args"`
    Outputs  []OutputCase `json:"outputs"`
    Requires []string     `json:"requires,omitempty"` // Required permissions (authz)
}

// OutputCase represents a typed output variant (success or error)
type OutputCase struct {
    Case   string            `json:"case"`   // "Success", "InsufficientStock", etc.
    Fields map[string]string `json:"fields"` // field name → type name
}

// StateSchema represents a state table definition
type StateSchema struct {
    Name   string            `json:"name"`
    Fields map[string]string `json:"fields"` // field name → type name
}

// NamedArg represents a named argument with type
type NamedArg struct {
    Name string `json:"name"`
    Type string `json:"type"`
}

// OperationalPrinciple represents a testable behavioral contract
type OperationalPrinciple struct {
    Description string `json:"description"`
    Scenario    string `json:"scenario"` // Path to scenario file or inline
}

// SyncRule represents a compiled sync rule (when/where/then)
type SyncRule struct {
    ID    string      `json:"id"`
    Scope string      `json:"scope"` // "flow", "global", or "keyed(field)"
    When  WhenClause  `json:"when"`
    Where WhereClause `json:"where"`
    Then  ThenClause  `json:"then"`
}

// Invocation represents an action invocation record
type Invocation struct {
    ID              string          `json:"id"`               // Content-addressed hash
    FlowToken       string          `json:"flow_token"`
    ActionURI       ActionRef       `json:"action_uri"`       // Typed action reference
    Args            IRObject        `json:"args"`             // Constrained to IRValue types
    Seq             int64           `json:"seq"`              // Logical clock (CP-2)
    SecurityContext SecurityContext `json:"security_context"` // Always present (CP-6)
    SpecHash        string          `json:"spec_hash"`        // Hash of concept spec
    EngineVersion   string          `json:"engine_version"`   // Engine version
    IRVersion       string          `json:"ir_version"`       // IR schema version
}

// Completion represents an action completion record
type Completion struct {
    ID              string          `json:"id"`               // Content-addressed hash
    InvocationID    string          `json:"invocation_id"`
    OutputCase      string          `json:"output_case"`      // "Success", error variant
    Result          IRObject        `json:"result"`           // Constrained to IRValue types
    Seq             int64           `json:"seq"`              // Logical clock (CP-2)
    SecurityContext SecurityContext `json:"security_context"` // Always present (CP-6)
}

// SecurityContext contains security metadata for audit trails (CP-6)
// MUST be non-pointer and always present on Invocation and Completion
type SecurityContext struct {
    TenantID    string   `json:"tenant_id"`
    UserID      string   `json:"user_id"`
    Permissions []string `json:"permissions"`
}
```

**Clause Types (internal/ir/clause.go):**
```go
// WhenClause specifies what completion triggers the sync
type WhenClause struct {
    Action     ActionRef         `json:"action"`      // Typed action reference
    Event      string            `json:"event"`       // "completed" or "invoked"
    OutputCase *string           `json:"output_case"` // nil = match any
    Bindings   map[string]string `json:"bindings"`    // result field → bound var
}

// WhereClause specifies the query to produce bindings
type WhereClause struct {
    From     string            `json:"from"`     // Table/source name
    Filter   string            `json:"filter"`   // Filter expression
    Bindings map[string]string `json:"bindings"` // field → bound var
}

// ThenClause specifies the action to invoke
type ThenClause struct {
    Action ActionRef `json:"action"` // Typed action reference
    Args   IRObject  `json:"args"`   // Template with "bound.varName" refs
}
```

**Reference Types (internal/ir/refs.go):**
```go
// ActionRef is a typed reference to a concept action
// Format: "Concept.action" (will evolve to nysm://... URI in future)
type ActionRef string

// ConceptRef is a typed reference to a concept
type ConceptRef struct {
    Name    string `json:"name"`
    Version string `json:"version,omitempty"`
}
```

**Store-Layer Types (internal/ir/store_types.go):**
```go
// NOTE: These are store-internal types, not part of the canonical IR.
// They use auto-increment IDs for FK references (exception to CP-2).

// SyncFiring represents a sync rule firing record (store-layer)
type SyncFiring struct {
    ID           int64  `json:"id"`            // Auto-increment (store FK)
    CompletionID string `json:"completion_id"` // Content-addressed
    SyncID       string `json:"sync_id"`
    BindingHash  string `json:"binding_hash"`  // Hash of binding values (CP-1)
    Seq          int64  `json:"seq"`           // Logical clock
}

// ProvenanceEdge links a sync firing to its generated invocation (store-layer)
type ProvenanceEdge struct {
    ID           int64  `json:"id"`             // Auto-increment (store FK)
    SyncFiringID int64  `json:"sync_firing_id"`
    InvocationID string `json:"invocation_id"`  // Content-addressed
}
```

**Version Constants (internal/ir/version.go):**
```go
// Package version constants for IR schema and engine
const (
    IRVersion     = "1"     // IR schema version
    EngineVersion = "0.1.0" // NYSM engine version
)
```

### File List

Files to create (in dependency order):

1. `go.mod` / `go.sum` - Module definition
2. `.gitignore` - Git ignore patterns
3. `.golangci.yml` - Linter configuration
4. `README.md` - Project overview stub
5. `cmd/nysm/main.go` - CLI entry point stub
6. `internal/ir/doc.go` - Package documentation
7. `internal/ir/version.go` - Version constants
8. `internal/ir/value.go` - IRValue sealed interface
9. `internal/ir/refs.go` - ActionRef, ConceptRef
10. `internal/ir/types.go` - Core IR types
11. `internal/ir/clause.go` - Clause types
12. `internal/ir/store_types.go` - Store-layer types
13. `internal/ir/types_test.go` - Type tests
14. `internal/ir/value_test.go` - IRValue tests

### Testing Requirements

```go
// Example: Verify JSON snake_case naming
func TestJSONFieldNaming(t *testing.T) {
    inv := Invocation{FlowToken: "test", ActionURI: "Test.action"}
    data, _ := json.Marshal(inv)

    assert.Contains(t, string(data), `"flow_token"`)
    assert.Contains(t, string(data), `"action_uri"`)
    assert.NotContains(t, string(data), `"flowToken"`) // NOT camelCase
}

// Example: Verify empty structs don't panic
func TestEmptyStructMarshaling(t *testing.T) {
    empty := ConceptSpec{}
    _, err := json.Marshal(empty)
    require.NoError(t, err)
}

// Example: Table-driven tests
func TestSecurityContextAlwaysPresent(t *testing.T) {
    tests := []struct {
        name string
        inv  Invocation
    }{
        {"empty", Invocation{}},
        {"with_context", Invocation{SecurityContext: SecurityContext{UserID: "u1"}}},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            data, _ := json.Marshal(tt.inv)
            assert.Contains(t, string(data), `"security_context"`)
        })
    }
}
```

### Story Completion Checklist

- [ ] All expected files created
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./internal/ir/...` passes
- [ ] No import cycles
- [ ] IR package has no internal dependencies
- [ ] All JSON tags use snake_case
- [ ] SecurityContext on both Invocation AND Completion
- [ ] No float types anywhere in IR

### References

- [Source: docs/architecture.md#Technology Stack] - Go 1.25, CUE SDK v0.15.1
- [Source: docs/architecture.md#Core IR Types] - Type definitions
- [Source: docs/architecture.md#Critical Patterns CP-1 through CP-6] - All patterns
- [Source: docs/architecture.md#Complete Project Directory Structure] - Layout
- [Source: docs/prd.md#FR-1.1] - Concept specs requirements
- [Source: docs/epics.md#Story 1.1] - Story definition

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- Validation 1: Claude agent (14 critical, 22 enhancements, 11 optimizations)
- Validation 2: Codex agent via clink (+3 critical issues found)
- All 17 critical issues resolved
- All 24 enhancements applied
- All 11 optimizations applied

### Completion Notes

- Foundation story - all subsequent stories depend on IR types
- IRValue sealed interface enforces no-floats constraint (CP-5)
- SecurityContext on BOTH Invocation AND Completion (CP-6)
- SyncFiring/ProvenanceEdge are store-layer types with auto-increment IDs
- ActionRef typed reference replaces raw strings
- Args/Result use IRObject (constrained IRValue types)
