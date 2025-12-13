---
stepsCompleted: [0, 1, 2, 3]
inputDocuments:
  - docs/prd.md
  - docs/architecture.md
workflowType: 'epics-and-stories'
lastStep: 3
status: 'complete'
project_name: 'NYSM'
user_name: 'Tyler'
date: '2025-12-12'
---

# NYSM - Epic Breakdown

**Author:** Tyler
**Date:** 2025-12-12
**Project Level:** Framework/Compiler+Runtime
**Target Scale:** Single-tenant, Embeddable

---

## Overview

This document provides the complete epic and story breakdown for NYSM (Now You See Me), decomposing the requirements from the [PRD](./prd.md) into implementable stories with full technical context from the [Architecture](./architecture.md).

**Framework Type:** Compiler + Runtime Engine for deterministic event-sourced systems

---

## Context Validation

**Documents Loaded:**

| Document | Status | Description |
|----------|--------|-------------|
| **PRD.md** | ✅ Loaded | 6 Functional Requirements (FR-1 through FR-6), 4 NFR categories |
| **Architecture.md** | ✅ Loaded | 9 critical decisions, 6 implementation patterns (CP-1 through CP-6), 80+ file structure |
| **UX Design** | ⚪ N/A | CLI-only framework, no UI components |

---

## Functional Requirements Inventory

### FR-1: Concept Specification System

| ID | Requirement |
|----|-------------|
| FR-1.1 | Support concept specs in CUE format with purpose, state schema, action signatures, operational principles |
| FR-1.2 | Validate concept specs against canonical IR schema |
| FR-1.3 | Compile CUE specs to canonical JSON IR |
| FR-1.4 | Support typed action outputs with multiple cases (success, error variants) |

### FR-2: Synchronization System

| ID | Requirement |
|----|-------------|
| FR-2.1 | Support 3-clause sync rules: when → where → then |
| FR-2.2 | Compile when-clause to match on action completions |
| FR-2.3 | Compile where-clause to SQL queries over relational state |
| FR-2.4 | Execute then-clause to generate invocations from bindings |
| FR-2.5 | Maintain abstraction boundary for future SPARQL migration |

### FR-3: Flow Token System

| ID | Requirement |
|----|-------------|
| FR-3.1 | Generate unique flow tokens for request scoping |
| FR-3.2 | Propagate flow tokens through all invocations/completions |
| FR-3.3 | Enforce sync rules only match records with same flow token |

### FR-4: Provenance System

| ID | Requirement |
|----|-------------|
| FR-4.1 | Record provenance edges: (completion) -[sync-id]-> (invocation) |
| FR-4.2 | Support idempotency check via sync edges |
| FR-4.3 | Enable crash/restart replay with identical results |

### FR-5: Durable Engine

| ID | Requirement |
|----|-------------|
| FR-5.1 | SQLite-backed append-only log for invocations/completions |
| FR-5.2 | Store provenance edges with query support |
| FR-5.3 | Support crash recovery and replay |

### FR-6: Conformance Harness

| ID | Requirement |
|----|-------------|
| FR-6.1 | Load concept specs and sync rules for test execution |
| FR-6.2 | Run scenarios with assertions on action traces |
| FR-6.3 | Validate operational principles as executable tests |
| FR-6.4 | Generate golden trace snapshots |

**Total:** 18 functional requirements across 6 categories

---

## Epic Structure Plan

### Epic Overview

| Epic | Title | User Value | PRD Coverage |
|------|-------|------------|--------------|
| **1** | Foundation & IR Core | Developers can define concepts in CUE and get validated canonical IR | FR-1.1, FR-1.2, FR-1.3, FR-1.4 |
| **2** | Durable Event Store | Developers can persist and replay deterministic event logs | FR-5.1, FR-5.2, FR-5.3, FR-4.1, FR-4.3 |
| **3** | Sync Engine Core | Developers can define reactive sync rules that fire on completions | FR-2.1, FR-2.2, FR-3.1, FR-3.2, FR-3.3 |
| **4** | Query & Binding System | Developers can write where-clauses that query state and produce bindings | FR-2.3, FR-2.4, FR-2.5 |
| **5** | Idempotency & Cycle Safety | Developers get guaranteed idempotent firing and cycle detection | FR-4.2, CRITICAL-1, CRITICAL-3 |
| **6** | Conformance Harness | Developers can validate operational principles as executable tests | FR-6.1, FR-6.2, FR-6.3, FR-6.4 |
| **7** | CLI & Demo Application | Developers can compile, run, test, and trace NYSM applications | CLI commands, Demo specs |

### Epic Dependencies

```
Epic 1 (Foundation) ─────────────────────────────────────────┐
       │                                                      │
       ▼                                                      │
Epic 2 (Store) ──────────────────────────────┐               │
       │                                      │               │
       ▼                                      │               │
Epic 3 (Sync Engine) ◄────────────────────────┤               │
       │                                      │               │
       ▼                                      ▼               │
Epic 4 (Query/Binding) ──────────► Epic 5 (Idempotency) ◄────┘
       │                                      │
       └──────────────────┬───────────────────┘
                          ▼
                   Epic 6 (Harness)
                          │
                          ▼
                   Epic 7 (CLI/Demo)
```

### Technical Context per Epic

**Epic 1 - Foundation & IR Core:**
- Uses CUE SDK v0.15.1 for parsing
- Implements RFC 8785 canonical JSON (UTF-16 key ordering)
- SHA-256 content-addressed identity with domain separation
- Constrained IRValue types (no floats) per CP-5

**Epic 2 - Durable Event Store:**
- SQLite v1.14.32 with WAL mode
- Logical clocks (seq INTEGER, not timestamps) per CP-2
- Schema with invocations, completions, sync_firings, provenance_edges tables
- Parameterized queries only per HIGH-3

**Epic 3 - Sync Engine Core:**
- Single-writer event loop for determinism
- When-clause matching with output case support per MEDIUM-2
- UUIDv7 flow tokens (injectable) per Architecture
- Flow scoping modes (flow/global/keyed) per HIGH-1

**Epic 4 - Query & Binding System:**
- QueryIR abstraction layer per HIGH-2
- SQL backend with deterministic ORDER BY per CP-4
- Portable fragment validation (no NULLs, no outer joins)

**Epic 5 - Idempotency & Cycle Safety:**
- Binding-level idempotency per CP-1: UNIQUE(completion_id, sync_id, binding_hash)
- Cycle detection per flow per CRITICAL-3
- Max-steps quota enforcement

**Epic 6 - Conformance Harness:**
- Scenario loading from YAML
- Trace assertions
- Golden snapshot comparison with goldie
- Deterministic clock/flow injection for tests

**Epic 7 - CLI & Demo:**
- Cobra CLI framework
- Commands: compile, validate, run, replay, test, trace
- Demo specs: Cart, Inventory, Web concepts + cart-inventory sync

---

## FR Coverage Map

| FR | Epic | Stories |
|----|------|---------|
| FR-1.1 | Epic 1 | 1.1, 1.2, 1.3 |
| FR-1.2 | Epic 1 | 1.4, 1.5 |
| FR-1.3 | Epic 1 | 1.6, 1.7 |
| FR-1.4 | Epic 1 | 1.3 |
| FR-2.1 | Epic 3 | 3.1, 3.2 |
| FR-2.2 | Epic 3 | 3.3, 3.4 |
| FR-2.3 | Epic 4 | 4.1, 4.2, 4.3 |
| FR-2.4 | Epic 4 | 4.4, 4.5 |
| FR-2.5 | Epic 4 | 4.1 |
| FR-3.1 | Epic 3 | 3.5 |
| FR-3.2 | Epic 3 | 3.6 |
| FR-3.3 | Epic 3 | 3.7 |
| FR-4.1 | Epic 2 | 2.5, 2.6 |
| FR-4.2 | Epic 5 | 5.1, 5.2 |
| FR-4.3 | Epic 2 | 2.7 |
| FR-5.1 | Epic 2 | 2.1, 2.2, 2.3 |
| FR-5.2 | Epic 2 | 2.5, 2.6 |
| FR-5.3 | Epic 2 | 2.7 |
| FR-6.1 | Epic 6 | 6.1, 6.2 |
| FR-6.2 | Epic 6 | 6.3, 6.4 |
| FR-6.3 | Epic 6 | 6.5 |
| FR-6.4 | Epic 6 | 6.6 |

---

## Epic 1: Foundation & IR Core

**Goal:** Developers can define concepts in CUE and get validated canonical IR output.

**User Value:** After this epic, developers can write concept specs in CUE format and compile them to the canonical JSON IR that the rest of NYSM uses. This is the foundation that all other epics build upon.

**PRD Coverage:** FR-1.1, FR-1.2, FR-1.3, FR-1.4

**Architecture References:**
- Technology Stack (Go 1.25, CUE SDK v0.15.1)
- Core Architectural Decisions (RFC 8785, SHA-256, IRValue types)
- Critical Patterns CP-2, CP-3, CP-5

---

### Story 1.1: Project Initialization & IR Type Definitions

As a **developer building NYSM**,
I want **the project structure and core IR types defined**,
So that **I have a foundation to build all other components on**.

**Acceptance Criteria:**

**Given** a new Go project
**When** I initialize the NYSM module
**Then** the project structure matches Architecture section "Complete Project Directory Structure"

**And** `go.mod` contains:
```go
module github.com/tyler/nysm
go 1.25
require (
    cuelang.org/go v0.15.1
    github.com/mattn/go-sqlite3 v1.14.32
    github.com/spf13/cobra v1.10.2
    github.com/google/uuid v1.6.0
    github.com/stretchr/testify v1.11.1
    github.com/google/go-cmp v0.7.0
    github.com/sebdah/goldie/v2 v2.8.0
)
```

**And** `internal/ir/types.go` defines:
- `ConceptSpec` struct with purpose, state schema, actions, operational principles
- `ActionSig` struct with name, args (NamedArg[]), outputs (OutputCase[])
- `OutputCase` struct with case name and fields
- `StateSchema` struct with name and fields
- `NamedArg` struct with name and type
- `SyncRule` struct with id, when, where, then clauses
- `Invocation` struct with ID, flow_token, action_uri, args, seq, security_context
- `Completion` struct with ID, invocation_id, output_case, result, seq
- `SyncFiring` struct with completion_id, sync_id, binding_hash, seq
- `ProvenanceEdge` struct with sync_firing_id, invocation_id
- `SecurityContext` struct with tenant_id, user_id, permissions (per CP-6)

**And** all JSON tags use snake_case (per Architecture naming conventions)

**Technical Notes:**
- `internal/ir/` has NO dependencies on other internal packages (Architecture rule)
- All types use `json:"field_name"` tags matching IR Field Names convention
- SecurityContext is always present (non-pointer) per CP-6

**Prerequisites:** None (first story)

---

### Story 1.2: Constrained IRValue Type System

As a **developer building NYSM**,
I want **a constrained value type system that forbids floats**,
So that **all IR values are deterministically serializable**.

**Acceptance Criteria:**

**Given** the IR package
**When** I define values for action args and results
**Then** I can only use these types (per CP-5):

```go
// internal/ir/value.go
type IRValue interface {
    irValue() // Sealed interface marker
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

// NO IRFloat - floats are forbidden in IR
```

**And** attempting to use `float64` anywhere in IR types causes a compile error
**And** `IRObject` iteration uses `slices.Sorted(maps.Keys())` for determinism

**Technical Notes:**
- This implements CP-5 from Architecture
- No `map[string]any` with unconstrained types allowed in public API
- All IRObject serialization must sort keys

**Prerequisites:** Story 1.1

---

### Story 1.3: Typed Action Outputs with Error Variants

As a **developer defining concepts**,
I want **actions to have typed output cases including success and error variants**,
So that **sync rules can match on specific outcomes**.

**Acceptance Criteria:**

**Given** an ActionSig definition
**When** I define outputs
**Then** I can specify multiple output cases:

```go
type ActionSig struct {
    Name    string       `json:"name"`
    Args    []NamedArg   `json:"args"`
    Outputs []OutputCase `json:"outputs"`
}

type OutputCase struct {
    Case   string              `json:"case"`   // "Success", "InsufficientStock", etc.
    Fields map[string]string   `json:"fields"` // field name -> type string ("string", "int", "bool", "array", "object")
}
```

**And** at least one output case is required
**And** the "Success" case is conventionally first but not enforced
**And** error variants can have their own typed fields (e.g., `InsufficientStock` has `available`, `requested`)

**Technical Notes:**
- Implements FR-1.4 (typed action outputs with multiple cases)
- Supports MEDIUM-2 (error matching in when-clause)
- OutputCase.Fields is a TYPE SCHEMA (field name → type string), not runtime values
- Runtime Completion.Result uses IRObject (map[string]IRValue) for actual values
- Type strings: "string", "int", "bool", "array", "object" - NO "float"

**Prerequisites:** Story 1.2

---

### Story 1.4: RFC 8785 Canonical JSON Marshaling

As a **developer building NYSM**,
I want **deterministic JSON serialization following RFC 8785**,
So that **identical data always produces identical bytes for hashing**.

**Acceptance Criteria:**

**Given** any IR type or IRValue
**When** I call `ir.MarshalCanonical(v)`
**Then** the output follows RFC 8785:

1. **Object keys sorted by UTF-16 code units** (not UTF-8 bytes) per CP-3:
```go
import "unicode/utf16"

// CRITICAL: Must use unicode/utf16.Encode for correct surrogate pair handling.
// Rune-by-rune comparison is WRONG for supplementary characters (U+10000+).
func compareKeysRFC8785(a, b string) int {
    a16 := utf16.Encode([]rune(a))
    b16 := utf16.Encode([]rune(b))

    minLen := len(a16)
    if len(b16) < minLen {
        minLen = len(b16)
    }

    for i := 0; i < minLen; i++ {
        if a16[i] != b16[i] {
            if a16[i] < b16[i] {
                return -1
            }
            return 1
        }
    }

    if len(a16) < len(b16) {
        return -1
    }
    if len(a16) > len(b16) {
        return 1
    }
    return 0
}
```

2. **No floats** - only int64, string, bool, array, object
3. **No HTML escaping** - `<>&` not escaped
4. **Compact output** - no whitespace
5. **Unicode NFC normalization** at ingestion boundaries

**And** fuzz tests verify: `MarshalCanonical(Unmarshal(MarshalCanonical(x))) == MarshalCanonical(x)`

**And** cross-language fixtures in `testdata/fixtures/rfc8785/` validate correctness

**Technical Notes:**
- This is the most critical correctness requirement
- Implements Architecture "Canonical JSON (RFC 8785)" decision
- UTF-16 ordering differs from Go's default - must use custom comparator
- Test against fixtures from other languages (Python, JS) to ensure interop

**Prerequisites:** Story 1.2

---

### Story 1.5: Content-Addressed Identity with Domain Separation

As a **developer building NYSM**,
I want **content-addressed IDs using SHA-256 with domain separation**,
So that **records have stable identity for replay and provenance**.

**Acceptance Criteria:**

**Given** an Invocation, Completion, or binding values
**When** I compute its ID
**Then** I use the format from Architecture:

```go
// internal/ir/hash.go

// InvocationID computes content-addressed ID
func InvocationID(flowToken, actionURI string, args IRObject, seq int64) string {
    canonical, _ := MarshalCanonical(map[string]any{
        "flow_token": flowToken,
        "action_uri": actionURI,
        "args":       args,
        "seq":        seq,
    })
    return hashWithDomain("nysm/invocation/v1", canonical)
}

// CompletionID computes content-addressed ID
func CompletionID(invocationID, outputCase string, result IRObject, seq int64) string {
    canonical, _ := MarshalCanonical(map[string]any{
        "invocation_id": invocationID,
        "output_case":   outputCase,
        "result":        result,
        "seq":           seq,
    })
    return hashWithDomain("nysm/completion/v1", canonical)
}

// BindingHash computes hash for idempotency
func BindingHash(bindings IRObject) string {
    canonical, _ := MarshalCanonical(bindings)
    return hashWithDomain("nysm/binding/v1", canonical)
}

func hashWithDomain(domain string, data []byte) string {
    h := sha256.New()
    h.Write([]byte(domain))
    h.Write([]byte{0x00}) // Null separator
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
```

**And** domain separation prevents cross-type collisions
**And** version prefix (`v1`) allows future algorithm migration

**Technical Notes:**
- Implements CP-2 (Logical Identity)
- SHA-256 is "overkill" but eliminates collision as correctness concern
- Domain separation format: `{domain}\x00{canonical_json_bytes}`

**Prerequisites:** Story 1.4

---

### Story 1.6: CUE Concept Spec Parser

As a **developer defining concepts**,
I want **to write concept specs in CUE and have them parsed**,
So that **I get the benefits of CUE's type system and validation**.

**Acceptance Criteria:**

**Given** a CUE concept spec file like:
```cue
concept Cart {
    purpose: "Manages shopping cart state for a user session"

    state CartItem {
        item_id: string
        quantity: int
    }

    action addItem {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { item_id: string, new_quantity: int }
        }, {
            case: "InvalidQuantity"
            fields: { message: string }
        }]
    }

    operational_principle: """
        Adding an item increases quantity or creates new entry
        """
}
```

**When** I call `compiler.CompileConcept(cueValue)`
**Then** I get an `ir.ConceptSpec` with all fields populated
**And** CUE validation errors are surfaced with line numbers
**And** missing required fields produce clear error messages

**Technical Notes:**
- Uses CUE SDK's Go API directly (not CLI subprocess)
- Implements FR-1.1 (CUE format with purpose, state, actions, operational principles)
- Parser in `internal/compiler/concept.go`

**Prerequisites:** Story 1.3

---

### Story 1.7: CUE Sync Rule Parser

As a **developer defining synchronizations**,
I want **to write sync rules in CUE with when/where/then clauses**,
So that **I can define reactive coordination between concepts**.

**Acceptance Criteria:**

**Given** a CUE sync spec file like:
```cue
sync "cart-inventory" {
    scope: "flow"  // or "global" or keyed("user_id")

    when: Cart.checkout.completed {
        case: "Success"
        bind: { cart_id: result.cart_id }
    }

    where: {
        from: CartItem
        filter: cart_id == bound.cart_id
        bind: { item_id: item_id, quantity: quantity }
    }

    then: Inventory.reserve {
        args: { item_id: bound.item_id, quantity: bound.quantity }
    }
}
```

**When** I call `compiler.CompileSync(cueValue)`
**Then** I get an `ir.SyncRule` with:
- `ID` (the sync name)
- `Scope` (flow/global/keyed)
- `When` clause with action ref, event type, output case, bindings
- `Where` clause with source, filter, bindings
- `Then` clause with action ref and arg template

**And** scoping modes are validated (only flow/global/keyed allowed)
**And** when-clause supports output case matching per MEDIUM-2

**Technical Notes:**
- Implements FR-2.1 (3-clause sync rules)
- Implements HIGH-1 (flow token scoping modes)
- Parser in `internal/compiler/sync.go`

**Prerequisites:** Story 1.6

---

### Story 1.8: IR Schema Validation

As a **developer compiling specs**,
I want **comprehensive validation of compiled IR against schema rules**,
So that **invalid specs are caught at compile time, not runtime**.

**Acceptance Criteria:**

**Given** compiled IR (ConceptSpec or SyncRule)
**When** I call `compiler.Validate(ir)`
**Then** validation checks:

1. **ConceptSpec validation:**
   - `purpose` is non-empty
   - At least one `action` defined
   - Each action has at least one output case
   - State field types are valid IRValue types
   - No duplicate action or state names

2. **SyncRule validation:**
   - `when` references a valid action
   - `scope` is one of: "flow", "global", or keyed("field")
   - `where` clause is syntactically valid
   - `then` references a valid action
   - Bound variables in `then` are defined in `when` or `where`

**And** validation errors are collected (not fail-fast) with clear messages:
```go
type ValidationError struct {
    Field   string
    Message string
    Line    int  // From CUE source if available
}
```

**And** error codes follow Architecture pattern: E100-E199 for validation errors

**Technical Notes:**
- Implements FR-1.2 (validate against canonical IR schema)
- Validator in `internal/compiler/validate.go`
- Collect all errors, don't stop at first

**Prerequisites:** Story 1.7

---

## Epic 2: Durable Event Store

**Goal:** Developers can persist and replay deterministic event logs.

**User Value:** After this epic, NYSM has a SQLite-backed append-only log that survives crashes and replays identically. This is the durability foundation that makes NYSM reliable.

**PRD Coverage:** FR-5.1, FR-5.2, FR-5.3, FR-4.1, FR-4.3

**Architecture References:**
- SQLite Configuration (WAL mode, pragmas)
- Schema design with logical clocks
- Critical Patterns CP-1, CP-2, CP-4

---

### Story 2.1: SQLite Store Initialization & Migrations

As a **developer running NYSM**,
I want **a SQLite store that initializes with the correct schema**,
So that **the event log is ready for use**.

**Acceptance Criteria:**

**Given** a database path
**When** I call `store.Open(path)`
**Then** the database is created with WAL mode and correct pragmas:

```sql
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA busy_timeout = 5000;
PRAGMA foreign_keys = ON;
```

**And** the schema is embedded via `go:embed`:
```go
//go:embed schema.sql
var schemaSQL string
```

**And** migrations are applied idempotently (safe to run multiple times)

**Technical Notes:**
- Implements FR-5.1 (SQLite-backed append-only log)
- Store in `internal/store/store.go`
- Schema in `internal/store/schema.sql`
- Uses `github.com/mattn/go-sqlite3` (requires CGO)

**Prerequisites:** Epic 1 complete

---

### Story 2.2: Event Log Schema with Logical Clocks

As a **developer building the store**,
I want **a schema using logical clocks instead of timestamps**,
So that **replay produces identical results regardless of wall time**.

**Acceptance Criteria:**

**Given** the store schema
**When** I define tables
**Then** they use `seq INTEGER` for ordering (per CP-2):

```sql
CREATE TABLE invocations (
    id TEXT PRIMARY KEY,           -- Content-addressed hash
    flow_token TEXT NOT NULL,
    action_uri TEXT NOT NULL,
    args TEXT NOT NULL,            -- Canonical JSON
    seq INTEGER NOT NULL,          -- Logical clock, NOT timestamp
    security_context TEXT NOT NULL, -- JSON (always present per CP-6)
    spec_hash TEXT NOT NULL,       -- Hash of concept spec at invoke time
    engine_version TEXT NOT NULL
);
CREATE INDEX idx_invocations_flow_token ON invocations(flow_token);
CREATE INDEX idx_invocations_seq ON invocations(seq);

CREATE TABLE completions (
    id TEXT PRIMARY KEY,           -- Content-addressed hash
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    output_case TEXT NOT NULL,
    result TEXT NOT NULL,          -- Canonical JSON
    seq INTEGER NOT NULL
);
CREATE INDEX idx_completions_flow_token_seq ON completions(
    (SELECT flow_token FROM invocations WHERE id = invocation_id), seq
);
CREATE INDEX idx_completions_seq ON completions(seq);
```

**And** NO `CURRENT_TIMESTAMP`, `datetime('now')`, or similar
**And** `seq` is monotonically increasing per engine instance

**Technical Notes:**
- Implements CP-2 (Logical Identity and Time)
- `id` is content-addressed via `ir.InvocationID()` / `ir.CompletionID()`
- `seq` managed by engine, not auto-increment

**Prerequisites:** Story 2.1

---

### Story 2.3: Write Invocations and Completions

As a **developer using the store**,
I want **to write invocations and completions to the event log**,
So that **all actions are durably recorded**.

**Acceptance Criteria:**

**Given** an `ir.Invocation` or `ir.Completion`
**When** I call `store.WriteInvocation(ctx, inv)` or `store.WriteCompletion(ctx, comp)`
**Then** the record is inserted with:
- Content-addressed `id` computed via `ir.InvocationID()` / `ir.CompletionID()`
- `args` / `result` serialized via `ir.MarshalCanonical()`
- `seq` from the logical clock

**And** duplicate IDs are handled gracefully (idempotent insert)
**And** all queries use parameterized SQL (never string interpolation) per HIGH-3

```go
func (s *Store) WriteInvocation(ctx context.Context, inv ir.Invocation) error {
    _, err := s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO invocations
        (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?)
    `, inv.ID, inv.FlowToken, inv.ActionURI,
       mustMarshal(inv.Args), inv.Seq,
       mustMarshal(inv.SecurityContext),
       inv.SpecHash, inv.EngineVersion)
    return err
}
```

**Technical Notes:**
- Implements FR-5.1 (append-only log for invocations/completions)
- Write functions in `internal/store/write.go`
- `INSERT OR IGNORE` makes writes idempotent

**Prerequisites:** Story 2.2

---

### Story 2.4: Read Flow and Query Support

As a **developer querying the store**,
I want **to read all events for a flow token**,
So that **I can reconstruct flow state and debug issues**.

**Acceptance Criteria:**

**Given** a flow token
**When** I call `store.ReadFlow(ctx, flowToken)`
**Then** I get all invocations and completions for that flow
**And** results are ordered by `seq ASC` with deterministic tiebreaker per CP-4:

```sql
SELECT id, flow_token, action_uri, args, seq, security_context
FROM invocations
WHERE flow_token = ?
ORDER BY seq ASC, id ASC COLLATE BINARY
```

**And** completions are joined to their invocations
**And** query functions return strongly-typed Go structs (not `map[string]any`)

**Technical Notes:**
- All queries MUST have `ORDER BY` with tiebreaker per CP-4
- Uses `COLLATE BINARY` for deterministic text ordering
- Read functions in `internal/store/read.go`

**Prerequisites:** Story 2.3

---

### Story 2.5: Sync Firings Table with Binding Hash

As a **developer building idempotency**,
I want **a sync_firings table that tracks each binding separately**,
So that **multi-binding syncs work correctly**.

**Acceptance Criteria:**

**Given** the need to track sync firings
**When** I define the schema
**Then** I use binding-level granularity per CP-1:

```sql
CREATE TABLE sync_firings (
    id INTEGER PRIMARY KEY,
    completion_id TEXT NOT NULL REFERENCES completions(id),
    sync_id TEXT NOT NULL,
    binding_hash TEXT NOT NULL,  -- Hash of binding values via ir.BindingHash()
    seq INTEGER NOT NULL,
    UNIQUE(completion_id, sync_id, binding_hash)  -- Idempotency per binding
);
CREATE INDEX idx_sync_firings_completion ON sync_firings(completion_id);
```

**And** `UNIQUE(completion_id, sync_id, binding_hash)` enforces idempotency
**And** `binding_hash` is computed via `ir.BindingHash(bindings)`
**And** attempting to insert duplicate (completion, sync, binding) fails gracefully

**Technical Notes:**
- Implements CP-1 (Binding-Level Idempotency)
- This is CRITICAL - without binding_hash, multi-binding syncs break
- Never use `UNIQUE(completion_id, sync_id)` alone

**Prerequisites:** Story 2.4

---

### Story 2.6: Provenance Edges Table

As a **developer tracing causality**,
I want **a provenance_edges table linking firings to invocations**,
So that **I can answer "why did this happen?"**.

**Acceptance Criteria:**

**Given** a sync firing
**When** it produces an invocation
**Then** a provenance edge is recorded:

```sql
CREATE TABLE provenance_edges (
    id INTEGER PRIMARY KEY,
    sync_firing_id INTEGER NOT NULL REFERENCES sync_firings(id),
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    UNIQUE(sync_firing_id)  -- Each firing produces exactly one invocation
);
CREATE INDEX idx_provenance_invocation ON provenance_edges(invocation_id);
```

**And** I can query backward: "what caused this invocation?"
```go
func (s *Store) ReadProvenance(ctx context.Context, invocationID string) ([]ir.ProvenanceEdge, error)
```

**And** I can query forward: "what did this completion trigger?"
```go
func (s *Store) ReadTriggered(ctx context.Context, completionID string) ([]ir.Invocation, error)
```

**Technical Notes:**
- Implements FR-4.1 (record provenance edges)
- Implements FR-5.2 (store provenance with query support)
- `UNIQUE(sync_firing_id)` ensures 1:1 firing→invocation

**Prerequisites:** Story 2.5

---

### Story 2.7: Crash Recovery and Replay

As a **developer running NYSM**,
I want **the engine to recover from crashes and replay identically**,
So that **durability guarantees are upheld**.

**Acceptance Criteria:**

**Given** a database with partial state (crash mid-flow)
**When** the engine restarts
**Then** it can:
1. **Detect incomplete flows** - flows with invocations but no terminal completion
2. **Resume from last checkpoint** - reprocess pending completions
3. **Produce identical results** - replay generates same invocations/firings

**And** replay is deterministic because:
- IDs are content-addressed (same inputs → same ID)
- `seq` is restored from database, not regenerated
- Binding hashes ensure idempotent re-firing

**And** a `store.ReplayFlow(ctx, flowToken)` function exists for explicit replay

**Technical Notes:**
- Implements FR-4.3 and FR-5.3 (crash recovery with identical replay)
- Relies on all earlier determinism work (CP-2, CP-4, content-addressed IDs)
- Recovery logic in `internal/store/store.go`

**Prerequisites:** Story 2.6

---

## Epic 3: Sync Engine Core

**Goal:** Developers can define reactive sync rules that fire on completions.

**User Value:** After this epic, the heart of NYSM works - when actions complete, sync rules automatically fire and generate follow-on invocations. This is what makes NYSM reactive.

**PRD Coverage:** FR-2.1, FR-2.2, FR-3.1, FR-3.2, FR-3.3

**Architecture References:**
- Concurrency Model (single-writer event loop)
- Flow Token Generation
- When-clause matching with output cases
- Critical Pattern CP-6

---

### Story 3.1: Single-Writer Event Loop

As a **developer building the engine**,
I want **a single-writer event loop for deterministic scheduling**,
So that **sync evaluation order is predictable and reproducible**.

**Acceptance Criteria:**

**Given** the engine design
**When** I implement the main loop
**Then** it follows the Architecture concurrency model:

```go
type Engine struct {
    store    Store
    compiler queryir.Compiler
    flowGen  FlowTokenGenerator
    clock    Clock
    specs    []ir.ConceptSpec
    syncs    []ir.SyncRule
    queue    *eventQueue  // FIFO
}

func (e *Engine) Run(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            event := e.queue.Dequeue()
            if event == nil {
                // Wait for new events
                continue
            }
            if err := e.processEvent(ctx, event); err != nil {
                return err
            }
        }
    }
}
```

**And** only ONE goroutine writes to SQLite
**And** events are processed in FIFO order
**And** no parallelism in the core evaluation loop

**Technical Notes:**
- Implements Architecture "Concurrency Model" decision
- Determinism requires single-threaded evaluation
- External action execution may be parallel, but completions enqueue back to single writer

**Prerequisites:** Epic 2 complete

---

### Story 3.2: Sync Rule Registration and Declaration Order

As a **developer using the engine**,
I want **sync rules evaluated in declaration order**,
So that **rule priority is explicit and predictable**.

**Acceptance Criteria:**

**Given** compiled sync rules
**When** I register them with the engine
**Then** they are stored in declaration order

**And** when a completion arrives, matching syncs are evaluated in that order:
```go
func (e *Engine) evaluateSyncs(ctx context.Context, comp ir.Completion) error {
    for _, sync := range e.syncs {  // Declaration order
        if e.matches(sync.When, comp) {
            // Process this sync
        }
    }
    return nil
}
```

**And** the order is preserved across restarts (determinism)

**Technical Notes:**
- Implements CRITICAL-3 (deterministic scheduling policy)
- Declaration order = order in compiled sync rule slice
- Sync rules registered via `engine.RegisterSyncs(syncs []ir.SyncRule)`

**Prerequisites:** Story 3.1

---

### Story 3.3: When-Clause Matching

As a **developer defining sync rules**,
I want **when-clauses to match completions by action and event type**,
So that **syncs fire at the right times**.

**Acceptance Criteria:**

**Given** a when-clause:
```go
type WhenClause struct {
    Action     ActionRef `json:"action"`      // e.g., "Cart.checkout"
    Event      string    `json:"event"`       // "completed" or "invoked"
    OutputCase *string   `json:"output_case"` // nil = match any, "Success" = match specific
    Bindings   map[string]string `json:"bindings"` // result field → bound variable
}
```

**When** a completion arrives
**Then** matching checks:
1. Action URI matches `when.Action`
2. Event type is "completed" (for completions)
3. Output case matches if specified (nil = any)

**And** if matched, bindings are extracted:
```go
func (e *Engine) extractBindings(when WhenClause, comp ir.Completion) ir.IRObject {
    bindings := make(ir.IRObject)
    for boundVar, resultField := range when.Bindings {
        bindings[boundVar] = comp.Result[resultField]
    }
    return bindings
}
```

**Technical Notes:**
- Implements FR-2.2 (compile when-clause to match completions)
- Implements MEDIUM-2 (error matching in when-clause)
- Matcher in `internal/engine/matcher.go`

**Prerequisites:** Story 3.2

---

### Story 3.4: Output Case Matching for Errors

As a **developer handling errors**,
I want **when-clauses to match specific error variants**,
So that **I can build error-handling sync rules**.

**Acceptance Criteria:**

**Given** a sync rule that matches an error:
```cue
sync "handle-insufficient-stock" {
    when: Inventory.reserve.completed {
        case: "InsufficientStock"
        bind: { item: result.item, available: result.available }
    }
    where: ...
    then: Cart.checkout.fail(reason: "insufficient_stock")
}
```

**When** an `Inventory.reserve` completion arrives with case "InsufficientStock"
**Then** the sync matches and extracts the error fields

**And** a sync with `case: "Success"` does NOT match error completions
**And** a sync with no case specified matches ALL outcomes

**Technical Notes:**
- Implements MEDIUM-2 (error matching in when-clause)
- This is how "no transactions" works - errors are explicit and handled via syncs
- Matcher must check `comp.OutputCase` against `when.OutputCase`

**Prerequisites:** Story 3.3

---

### Story 3.5: Flow Token Generation

As a **developer invoking actions**,
I want **flow tokens generated for new requests**,
So that **all related records are correlated**.

**Acceptance Criteria:**

**Given** a new external request
**When** I call `engine.NewFlow()`
**Then** a UUIDv7 flow token is generated:

```go
type FlowTokenGenerator interface {
    Generate() string
}

type UUIDv7Generator struct{}
func (UUIDv7Generator) Generate() string {
    return uuid.Must(uuid.NewV7()).String()
}
```

**And** the flow token is stored as 36-char string (hyphenated UUID)
**And** for tests, a `FixedGenerator` can return predetermined tokens:
```go
type FixedGenerator struct {
    tokens []string
    idx    int
}
func (g *FixedGenerator) Generate() string {
    token := g.tokens[g.idx]
    g.idx++
    return token
}
```

**Technical Notes:**
- Implements FR-3.1 (generate unique flow tokens)
- UUIDv7 is time-sortable which helps debugging
- Injectable generator enables deterministic tests

**Prerequisites:** Story 3.1

---

### Story 3.6: Flow Token Propagation

As a **developer building the engine**,
I want **flow tokens propagated through all invocations and completions**,
So that **sync rules can scope by flow**.

**Acceptance Criteria:**

**Given** an initial invocation with flow token F
**When** that action completes and triggers sync rules
**Then** all generated invocations also have flow token F

```go
func (e *Engine) generateInvocation(flowToken string, then ThenClause, bindings ir.IRObject) ir.Invocation {
    return ir.Invocation{
        ID:        ir.InvocationID(flowToken, then.Action, args, seq),
        FlowToken: flowToken,  // Propagated from triggering completion
        ActionURI: then.Action,
        Args:      e.resolveArgs(then.Args, bindings),
        Seq:       e.clock.Next(),
        SecurityContext: e.currentSecurityContext(),
        // ...
    }
}
```

**And** the flow token chain is unbroken from root to leaf
**And** provenance edges maintain the flow relationship

**Technical Notes:**
- Implements FR-3.2 (propagate flow tokens through all records)
- Flow token is inherited, never generated mid-flow

**Prerequisites:** Story 3.5

---

### Story 3.7: Flow-Scoped Sync Matching

As a **developer defining sync rules**,
I want **syncs to only match records with the same flow token by default**,
So that **concurrent requests don't accidentally join**.

**Acceptance Criteria:**

**Given** a sync rule with `scope: "flow"` (the default)
**When** matching completions for the where-clause
**Then** only completions with the SAME flow token are considered

```go
func (e *Engine) executeWhereClause(
    ctx context.Context,
    sync ir.SyncRule,
    flowToken string,
    whenBindings ir.IRObject,
) ([]ir.IRObject, error) {
    query := e.compiler.Compile(sync.Where)

    if sync.Scope == "flow" {
        // Add flow token filter
        query = query.WithFilter(queryir.Equals("flow_token", flowToken))
    }
    // ... execute query
}
```

**And** scoping modes work as specified in HIGH-1:
- `"flow"` - only same flow_token (default)
- `"global"` - all records regardless of flow
- `keyed("field")` - records sharing same value for field

**Technical Notes:**
- Implements FR-3.3 (enforce sync rules only match same flow token)
- Implements HIGH-1 (flow token scoping modes)
- Default is "flow" - explicit opt-in required for cross-flow matching

**Prerequisites:** Story 3.6

---

## Epic 4: Query & Binding System

**Goal:** Developers can write where-clauses that query state and produce bindings.

**User Value:** After this epic, sync rules can query the current state (projections) and extract multiple bindings, enabling complex reactive patterns like "for each item in cart, reserve inventory."

**PRD Coverage:** FR-2.3, FR-2.4, FR-2.5

**Architecture References:**
- QueryIR Abstraction (HIGH-2)
- SQL Backend with deterministic ordering (CP-4)
- State model (event-sourced projections)

---

### Story 4.1: QueryIR Type System

As a **developer building the query layer**,
I want **an abstract QueryIR between DSL and SQL**,
So that **I can migrate to SPARQL later without rewriting syncs**.

**Acceptance Criteria:**

**Given** the need for query abstraction per HIGH-2
**When** I define QueryIR
**Then** it supports:

```go
// internal/queryir/types.go

type Query interface {
    queryNode()
}

type Select struct {
    From      string            // Table/source name
    Filter    Predicate         // WHERE conditions
    Bindings  map[string]string // field → bound variable
}
func (Select) queryNode() {}

type Join struct {
    Left  Query
    Right Query
    On    Predicate
}
func (Join) queryNode() {}

type Predicate interface {
    predicateNode()
}

type Equals struct {
    Field string
    Value ir.IRValue
}
func (Equals) predicateNode() {}

type And struct {
    Predicates []Predicate
}
func (And) predicateNode() {}

type BoundEquals struct {
    Field     string
    BoundVar  string  // References a variable from when-clause bindings
}
func (BoundEquals) predicateNode() {}
```

**And** the portable fragment excludes:
- NULLs (use explicit Option types)
- Outer joins (inner only)
- Aggregations (not in portable fragment)

**Technical Notes:**
- Implements HIGH-2 (Query IR abstraction boundary)
- Implements FR-2.5 (maintain abstraction for SPARQL migration)
- QueryIR is the contract; backends implement it

**Prerequisites:** Epic 3 complete

---

### Story 4.2: QueryIR Validation

As a **developer using QueryIR**,
I want **queries validated against portable fragment rules**,
So that **I know if my query will work with future SPARQL backend**.

**Acceptance Criteria:**

**Given** a QueryIR query
**When** I call `queryir.Validate(query)`
**Then** it checks:

1. **No NULLs** - all fields have values
2. **No outer joins** - only inner joins
3. **Set semantics** - no duplicate handling required
4. **Explicit bindings** - no `SELECT *`

**And** validation returns:
```go
type ValidationResult struct {
    IsPortable bool
    Warnings   []string  // Non-portable features used
}
```

**And** non-portable queries are allowed but logged with warnings

**Technical Notes:**
- Portable fragment enables SPARQL migration
- SQL backend can handle non-portable queries, but they won't migrate
- Validator in `internal/queryir/validate.go`

**Prerequisites:** Story 4.1

---

### Story 4.3: SQL Backend Compiler

As a **developer executing queries**,
I want **QueryIR compiled to parameterized SQL**,
So that **where-clauses execute against SQLite**.

**Acceptance Criteria:**

**Given** a QueryIR query
**When** I call `querysql.Compile(query)`
**Then** I get parameterized SQL:

```go
func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error) {
    switch query := q.(type) {
    case queryir.Select:
        sql := fmt.Sprintf("SELECT %s FROM %s",
            c.compileBindings(query.Bindings),
            query.From)
        params := []any{}
        if query.Filter != nil {
            filterSQL, filterParams := c.compileFilter(query.Filter)
            sql += " WHERE " + filterSQL
            params = append(params, filterParams...)
        }
        // MANDATORY: Always add ORDER BY per CP-4
        sql += " ORDER BY " + c.stableOrderKey(query)
        return sql, params, nil
    // ...
    }
}
```

**And** ALL queries have `ORDER BY` with deterministic tiebreaker per CP-4
**And** string values are NEVER interpolated - always use `?` parameters
**And** `COLLATE BINARY` is used for text ordering

**Technical Notes:**
- Implements FR-2.3 (compile where-clause to SQL)
- Implements CP-4 (deterministic query ordering)
- Compiler in `internal/querysql/compile.go`

**Prerequisites:** Story 4.2

---

### Story 4.4: Binding Set Execution

As a **developer running sync rules**,
I want **where-clauses to return a SET of bindings**,
So that **then-clauses can fire multiple invocations**.

**Acceptance Criteria:**

**Given** a where-clause query
**When** executed against state tables
**Then** it returns zero or more binding sets:

```go
func (e *Engine) executeWhere(
    ctx context.Context,
    where ir.WhereClause,
    whenBindings ir.IRObject,
    flowToken string,
) ([]ir.IRObject, error) {
    query := e.buildQuery(where, whenBindings)
    sql, params, _ := e.compiler.Compile(query)

    rows, err := e.store.Query(ctx, sql, params...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var bindings []ir.IRObject
    for rows.Next() {
        binding := e.scanBinding(rows, where.Bindings)
        bindings = append(bindings, binding)
    }
    return bindings, nil
}
```

**And** bindings are ordered (not a set) per query ORDER BY
**And** empty result = sync doesn't fire (valid, not an error)
**And** multiple bindings = multiple invocations generated

**Technical Notes:**
- Implements FR-2.4 (execute then-clause to generate invocations from bindings)
- A single completion can trigger 0, 1, or N invocations depending on bindings
- Order matters for determinism

**Prerequisites:** Story 4.3

---

### Story 4.5: Then-Clause Invocation Generation

As a **developer building sync execution**,
I want **then-clauses to generate invocations from bindings**,
So that **follow-on actions are triggered**.

**Acceptance Criteria:**

**Given** binding sets from where-clause
**When** processing then-clause
**Then** for EACH binding, generate an invocation:

```go
func (e *Engine) executeThen(
    ctx context.Context,
    then ir.ThenClause,
    bindings []ir.IRObject,
    flowToken string,
    completion ir.Completion,
    sync ir.SyncRule,
) error {
    for _, binding := range bindings {
        // Compute binding hash for idempotency
        bindingHash := ir.BindingHash(binding)

        // Check if already fired (idempotency)
        if e.store.HasFiring(ctx, completion.ID, sync.ID, bindingHash) {
            continue  // Skip - already processed
        }

        // Generate invocation
        args := e.resolveArgs(then.Args, binding)
        inv := ir.Invocation{
            ID:        ir.InvocationID(flowToken, then.Action, args, e.clock.Next()),
            FlowToken: flowToken,
            ActionURI: then.Action,
            Args:      args,
            // ...
        }

        // Record firing and invocation
        firing := ir.SyncFiring{
            CompletionID: completion.ID,
            SyncID:       sync.ID,
            BindingHash:  bindingHash,
            Seq:          e.clock.Next(),
        }

        if err := e.store.WriteSyncFiring(ctx, firing); err != nil {
            return err
        }
        if err := e.store.WriteInvocation(ctx, inv); err != nil {
            return err
        }
        if err := e.store.WriteProvenanceEdge(ctx, firing.ID, inv.ID); err != nil {
            return err
        }

        // Enqueue for execution
        e.queue.Enqueue(inv)
    }
    return nil
}
```

**And** arg templates support `bound.varName` syntax:
```go
func (e *Engine) resolveArgs(template map[string]any, binding ir.IRObject) ir.IRObject {
    args := make(ir.IRObject)
    for key, val := range template {
        if ref, ok := val.(string); ok && strings.HasPrefix(ref, "bound.") {
            varName := strings.TrimPrefix(ref, "bound.")
            args[key] = binding[varName]
        } else {
            args[key] = val.(ir.IRValue)
        }
    }
    return args
}
```

**Technical Notes:**
- Implements FR-2.4 (generate invocations from bindings)
- Binding hash enables CP-1 (binding-level idempotency)
- Provenance edge links firing→invocation

**Prerequisites:** Story 4.4

---

## Epic 5: Idempotency & Cycle Safety

**Goal:** Developers get guaranteed idempotent firing and cycle detection.

**User Value:** After this epic, NYSM is safe from duplicate firings and infinite loops. Developers can confidently write sync rules knowing the engine handles edge cases correctly.

**PRD Coverage:** FR-4.2, CRITICAL-1, CRITICAL-3

**Architecture References:**
- CP-1 (Binding-Level Idempotency)
- CRITICAL-3 (Sync Engine Termination)

---

### Story 5.1: Binding-Level Idempotency Enforcement

As a **developer relying on idempotency**,
I want **sync firings to be idempotent at the binding level**,
So that **replay and recovery don't create duplicate invocations**.

**Acceptance Criteria:**

**Given** a completion that matches a sync rule
**When** the sync is evaluated (or re-evaluated after crash)
**Then** each (completion_id, sync_id, binding_hash) triple fires AT MOST once

```go
func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) bool {
    var count int
    s.db.QueryRowContext(ctx, `
        SELECT COUNT(*) FROM sync_firings
        WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
    `, completionID, syncID, bindingHash).Scan(&count)
    return count > 0
}
```

**And** the database enforces this via `UNIQUE(completion_id, sync_id, binding_hash)`
**And** `INSERT` on duplicate is handled gracefully (not an error, just skipped)

**Technical Notes:**
- Implements FR-4.2 (idempotency via sync edges)
- Implements CP-1 (binding-level idempotency)
- This is CRITICAL for correctness - without it, replays create duplicates

**Prerequisites:** Epic 4 complete

---

### Story 5.2: Idempotency in Replay Scenarios

As a **developer recovering from crashes**,
I want **replay to skip already-fired sync bindings**,
So that **recovery produces identical results**.

**Acceptance Criteria:**

**Given** a partially-processed flow (crash mid-execution)
**When** the engine replays the flow
**Then**:
1. Completions are re-processed
2. Sync matching is re-evaluated
3. Binding hashes are re-computed (deterministically)
4. Already-fired bindings are SKIPPED (via `HasFiring` check)
5. Only NEW bindings produce invocations

**And** the final state is identical to a complete run without crash

**Technical Notes:**
- This validates the idempotency model works end-to-end
- Relies on deterministic binding hash computation
- Test with chaos testing (kill at random points, replay, verify identical)

**Prerequisites:** Story 5.1

---

### Story 5.3: Cycle Detection per Flow

As a **developer writing sync rules**,
I want **cycles detected and prevented**,
So that **self-triggering rules don't loop forever**.

**Acceptance Criteria:**

**Given** sync rules that could cycle (A triggers B triggers A)
**When** evaluation would create a cycle within a flow
**Then** the cycle is detected BEFORE firing:

```go
type CycleDetector struct {
    history map[string]bool  // key: sync_id + binding_hash
}

func (c *CycleDetector) WouldCycle(syncID, bindingHash string) bool {
    key := syncID + ":" + bindingHash
    return c.history[key]
}

func (c *CycleDetector) Record(syncID, bindingHash string) {
    key := syncID + ":" + bindingHash
    c.history[key] = true
}
```

**And** cycles are logged with full trace:
```
WARN: Cycle detected in flow {flow_token}
  sync_id: cart-inventory
  binding_hash: abc123...
  trace: checkout → reserve → (would trigger checkout again)
```

**And** the cycle is broken (firing skipped), not an error

**Technical Notes:**
- Implements CRITICAL-3 (cycle detection per flow)
- Cycle detection is per-flow, not global
- Same (sync_id, binding_hash) in same flow = cycle

**Prerequisites:** Story 5.1

---

### Story 5.4: Max-Steps Quota Enforcement

As a **developer preventing runaway flows**,
I want **a maximum steps quota per flow**,
So that **non-terminating patterns are caught**.

**Acceptance Criteria:**

**Given** a flow with many sync firings
**When** the firing count exceeds the quota (default: 1000)
**Then** the flow is terminated with error:

```go
type QuotaEnforcer struct {
    maxSteps int
    current  int
}

func (q *QuotaEnforcer) Check() error {
    q.current++
    if q.current > q.maxSteps {
        return ErrQuotaExceeded{
            FlowToken: flowToken,
            Steps:     q.current,
            Limit:     q.maxSteps,
        }
    }
    return nil
}
```

**And** `ErrQuotaExceeded` is a typed error that can be handled
**And** the quota is configurable via engine options:
```go
engine := engine.New(
    engine.WithMaxSteps(5000),
    // ...
)
```

**Technical Notes:**
- Implements CRITICAL-3 (max-steps quota)
- Quota is per-flow, not global
- Default 1000 is reasonable; adjust based on use case

**Prerequisites:** Story 5.3

---

### Story 5.5: Compile-Time Cycle Analysis

As a **developer defining sync rules**,
I want **potential cycles detected at compile time**,
So that **I catch problems before runtime**.

**Acceptance Criteria:**

**Given** a set of sync rules
**When** compiling/loading them
**Then** static analysis detects potential cycles:

```go
func AnalyzeCycles(syncs []ir.SyncRule) []CycleWarning {
    // Build action → sync graph
    // Detect cycles in graph
    // Return warnings (not errors - runtime may not trigger cycle)
}

type CycleWarning struct {
    Path    []string  // ["sync-a", "sync-b", "sync-a"]
    Message string
}
```

**And** warnings are displayed but don't block compilation
**And** `@allow_recursion` annotation suppresses warning for intentional recursion:
```cue
sync "intentional-recursion" {
    @allow_recursion
    // ...
}
```

**Technical Notes:**
- Implements CRITICAL-3 (stratification analysis)
- Static analysis is conservative - may warn on non-cyclic patterns
- Annotation allows intentional recursion (rare but valid)

**Prerequisites:** Story 5.3

---

## Epic 6: Conformance Harness

**Goal:** Developers can validate operational principles as executable tests.

**User Value:** After this epic, the operational principles in concept specs are not just documentation - they're tests that verify the system behaves correctly. This is NYSM's answer to "documentation rot."

**PRD Coverage:** FR-6.1, FR-6.2, FR-6.3, FR-6.4

**Architecture References:**
- Harness package structure
- Golden snapshot testing
- Deterministic test helpers

---

### Story 6.1: Scenario Definition Format

As a **developer writing conformance tests**,
I want **a YAML format for defining test scenarios**,
So that **I can specify inputs, expected outputs, and assertions**.

**Acceptance Criteria:**

**Given** a scenario file like:
```yaml
# testdata/scenarios/cart_checkout_success.yaml
name: cart_checkout_success
description: "Successful checkout triggers inventory reservation"

specs:
  - specs/cart.concept.cue
  - specs/inventory.concept.cue
  - specs/cart-inventory.sync.cue

setup:
  - action: Inventory.setStock
    args: { item_id: "widget", quantity: 10 }

flow:
  - invoke: Cart.addItem
    args: { item_id: "widget", quantity: 3 }
    expect:
      case: Success
      result: { item_id: "widget", new_quantity: 3 }

  - invoke: Cart.checkout
    args: {}
    expect:
      case: Success

assertions:
  - type: trace_contains
    action: Inventory.reserve
    args: { item_id: "widget", quantity: 3 }

  - type: final_state
    table: inventory
    where: { item_id: "widget" }
    expect: { quantity: 7 }  # 10 - 3
```

**When** I load the scenario
**Then** I get a structured `Scenario` object with all test steps

**Technical Notes:**
- Implements FR-6.1 (load concept specs and sync rules)
- Scenario loader in `internal/harness/scenario.go`
- YAML chosen for human readability

**Prerequisites:** Epic 5 complete

---

### Story 6.2: Test Execution Engine

As a **developer running conformance tests**,
I want **scenarios executed with deterministic clock and flow tokens**,
So that **tests are reproducible**.

**Acceptance Criteria:**

**Given** a loaded scenario
**When** I call `harness.Run(scenario)`
**Then**:
1. A fresh in-memory database is created
2. Specs are compiled and loaded
3. Setup steps are executed
4. Flow steps are executed with fixed flow token
5. Assertions are evaluated
6. Results are reported

```go
func Run(scenario *Scenario) (*Result, error) {
    // Use deterministic test helpers
    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator(scenario.FlowToken)

    engine := engine.New(
        engine.WithClock(clock),
        engine.WithFlowGenerator(flowGen),
        // ...
    )

    // Execute scenario steps
    for _, step := range scenario.Flow {
        result, err := engine.Invoke(ctx, step.Action, step.Args)
        // Validate against step.Expect
    }

    // Evaluate assertions
    for _, assertion := range scenario.Assertions {
        // Check assertion
    }
}
```

**And** the same scenario run twice produces identical results

**Technical Notes:**
- Implements FR-6.2 (run scenarios with assertions)
- Deterministic helpers from `internal/testutil/`
- In-memory SQLite for isolation

**Prerequisites:** Story 6.1

---

### Story 6.3: Trace Assertions

As a **developer verifying behavior**,
I want **to assert on the action trace**,
So that **I can verify sync rules fired correctly**.

**Acceptance Criteria:**

**Given** assertion types:
```go
type TraceContains struct {
    Action string
    Args   ir.IRObject
}

type TraceOrder struct {
    Actions []string  // Must appear in this order
}

type TraceCount struct {
    Action string
    Count  int
}
```

**When** evaluating assertions
**Then** `trace_contains` checks the action appears in the trace
**And** `trace_order` checks actions appear in specified order
**And** `trace_count` checks exact number of occurrences

```go
func (h *Harness) assertTraceContains(trace []ir.Event, assertion TraceContains) error {
    for _, event := range trace {
        if inv, ok := event.(ir.Invocation); ok {
            if inv.ActionURI == assertion.Action && h.matchArgs(inv.Args, assertion.Args) {
                return nil
            }
        }
    }
    return fmt.Errorf("trace does not contain %s with args %v", assertion.Action, assertion.Args)
}
```

**Technical Notes:**
- Implements FR-6.2 (assertions on action traces)
- Assertions in `internal/harness/assertions.go`
- Support flexible matching (subset of args)

**Prerequisites:** Story 6.2

---

### Story 6.4: Final State Assertions

As a **developer verifying outcomes**,
I want **to assert on final state table contents**,
So that **I can verify projections are correct**.

**Acceptance Criteria:**

**Given** assertion type:
```go
type FinalState struct {
    Table  string
    Where  map[string]ir.IRValue
    Expect map[string]ir.IRValue
}
```

**When** evaluating the assertion
**Then** the specified row is queried
**And** the expected values are compared:

```go
func (h *Harness) assertFinalState(assertion FinalState) error {
    query := fmt.Sprintf("SELECT * FROM %s WHERE %s",
        assertion.Table,
        h.buildWhere(assertion.Where))

    row := h.store.QueryRow(query, h.whereArgs(assertion.Where)...)
    actual := h.scanRow(row)

    for key, expected := range assertion.Expect {
        if !reflect.DeepEqual(actual[key], expected) {
            return fmt.Errorf("expected %s=%v, got %v", key, expected, actual[key])
        }
    }
    return nil
}
```

**And** missing rows are reported clearly
**And** extra columns in actual result are ignored (subset check)

**Technical Notes:**
- State assertions check projections are correctly updated
- Query uses parameterized SQL per HIGH-3

**Prerequisites:** Story 6.3

---

### Story 6.5: Operational Principle Validation

As a **developer defining concepts**,
I want **operational principles validated as executable tests**,
So that **documentation can't rot**.

**Acceptance Criteria:**

**Given** a concept spec with operational principle:
```cue
concept Cart {
    // ...
    operational_principle: """
        When a user adds an item that already exists in the cart,
        the quantity is increased rather than creating a duplicate entry.
        """
}
```

**When** I run the conformance harness
**Then** the operational principle is parsed for test scenarios
**And** scenarios are extracted and executed
**And** failures indicate the principle is not upheld

```go
func ExtractScenarios(principle string) []Scenario {
    // Parse natural language into test steps
    // This can be AI-assisted or use structured format
}
```

**And** alternatively, operational principles can reference explicit scenario files:
```cue
operational_principle: {
    description: "Adding existing item increases quantity"
    scenario: "testdata/scenarios/cart_add_existing.yaml"
}
```

**Technical Notes:**
- Implements FR-6.3 (validate operational principles as tests)
- Start with explicit scenario references; add NL parsing later
- This is what makes NYSM's documentation "legible" - it's tested

**Prerequisites:** Story 6.4

---

### Story 6.6: Golden Trace Snapshots

As a **developer maintaining tests**,
I want **golden file comparison for trace snapshots**,
So that **I can easily detect unexpected changes**.

**Acceptance Criteria:**

**Given** a scenario execution
**When** I request golden comparison
**Then** the full trace is compared against `testdata/golden/{scenario}.golden`:

```go
func (h *Harness) RunWithGolden(scenario *Scenario) error {
    result, _ := h.Run(scenario)

    // Format trace for comparison
    traceJSON, _ := ir.MarshalCanonical(result.Trace)

    // Compare with golden file using goldie
    g := goldie.New(h.t)
    g.Assert(h.t, scenario.Name, traceJSON)

    return nil
}
```

**And** running with `-update` flag regenerates golden files:
```bash
go test ./internal/harness -update
```

**And** golden files use canonical JSON for deterministic comparison

**Technical Notes:**
- Implements FR-6.4 (generate golden trace snapshots)
- Uses `github.com/sebdah/goldie/v2`
- Golden files committed to repo in `testdata/golden/`

**Prerequisites:** Story 6.5

---

## Epic 7: CLI & Demo Application

**Goal:** Developers can compile, run, test, and trace NYSM applications.

**User Value:** After this epic, NYSM is a usable CLI tool. Developers can build the canonical cart/inventory demo and have a complete reference implementation of the WYSIWYG pattern.

**PRD Coverage:** CLI commands, Demo specs from PRD Appendix A

---

### Story 7.1: CLI Framework Setup

As a **developer using NYSM**,
I want **a well-structured CLI with standard commands**,
So that **I can interact with NYSM from the terminal**.

**Acceptance Criteria:**

**Given** the CLI entry point
**When** I run `nysm --help`
**Then** I see:
```
NYSM - Now You See Me

A framework for building legible software with the WYSIWYG pattern.

Usage:
  nysm [command]

Available Commands:
  compile     Compile CUE specs to canonical IR
  validate    Validate specs without full compilation
  run         Start engine with compiled specs
  replay      Replay event log from scratch
  test        Run conformance harness
  trace       Query provenance for a flow

Flags:
  -h, --help      help for nysm
  -v, --verbose   verbose output
  --format        output format (json|text)

Use "nysm [command] --help" for more information about a command.
```

**And** the CLI uses Cobra v1.10.2
**And** all commands follow kebab-case flag convention

**Technical Notes:**
- CLI entry in `cmd/nysm/main.go`
- Commands in `internal/cli/`
- Uses Architecture CLI flags convention

**Prerequisites:** Epic 6 complete

---

### Story 7.2: Compile Command

As a **developer building NYSM apps**,
I want **a compile command that produces canonical IR**,
So that **I can see what my specs compile to**.

**Acceptance Criteria:**

**Given** CUE spec files
**When** I run `nysm compile ./specs`
**Then** I get compiled IR output:

```bash
$ nysm compile ./specs
✓ Compiled 3 concepts, 2 syncs

Concepts:
  Cart: 3 actions, 1 operational principle
  Inventory: 2 actions, 1 operational principle
  Web: 2 actions

Syncs:
  cart-inventory: Cart.checkout → Inventory.reserve
  inventory-response: Inventory.reserve → Web.respond

$ nysm compile ./specs --format json
{
  "concepts": [...],
  "syncs": [...]
}
```

**And** validation errors are reported with file:line references
**And** `--output` flag writes IR to file

**Technical Notes:**
- Implements compilation workflow for end users
- Command in `internal/cli/compile.go`
- Human output by default, JSON with `--format json`

**Prerequisites:** Story 7.1

---

### Story 7.3: Validate Command

As a **developer checking specs**,
I want **a validate command that checks specs without full compilation**,
So that **I can get fast feedback during development**.

**Acceptance Criteria:**

**Given** CUE spec files
**When** I run `nysm validate ./specs`
**Then** I get validation results:

```bash
$ nysm validate ./specs
✓ All specs valid

$ nysm validate ./specs-with-errors
✗ Validation failed

specs/cart.concept.cue:15:3
  E101: Missing required field: purpose

specs/cart-inventory.sync.cue:8:5
  E102: Unknown action reference: Cart.invalid
```

**And** exit code 0 for success, 1 for errors
**And** all errors collected and reported (not fail-fast)

**Technical Notes:**
- Faster than compile (skips IR generation)
- Good for editor integration / CI
- Command in `internal/cli/validate.go`

**Prerequisites:** Story 7.2

---

### Story 7.4: Run Command

As a **developer running NYSM apps**,
I want **a run command that starts the engine**,
So that **I can execute flows against my specs**.

**Acceptance Criteria:**

**Given** compiled specs and a database
**When** I run `nysm run --db ./nysm.db ./specs`
**Then** the engine starts and accepts invocations:

```bash
$ nysm run --db ./nysm.db ./specs
Engine started. Listening for invocations...

# In another terminal or via HTTP adapter:
$ nysm invoke Cart.addItem --args '{"item_id":"widget","quantity":3}'
{
  "flow_token": "019376f8-...",
  "result": {"case": "Success", ...}
}
```

**And** the database is created if it doesn't exist
**And** ctrl-C gracefully shuts down

**Technical Notes:**
- For MVP, invocations via CLI invoke subcommand
- HTTP adapter is Phase 6 (out of MVP scope)
- Command in `internal/cli/run.go`

**Prerequisites:** Story 7.3

---

### Story 7.5: Replay Command

As a **developer debugging issues**,
I want **a replay command that re-executes the event log**,
So that **I can verify determinism and debug problems**.

**Acceptance Criteria:**

**Given** a database with events
**When** I run `nysm replay --db ./nysm.db`
**Then** the event log is replayed from scratch:

```bash
$ nysm replay --db ./nysm.db
Replaying event log...
  Invocations: 15
  Completions: 15
  Sync firings: 8

✓ Replay complete
  Result: IDENTICAL (determinism verified)
```

**And** `--flow` flag replays specific flow only
**And** differences from original are reported (should be none if deterministic)

**Technical Notes:**
- Implements FR-5.3 validation via user command
- Command in `internal/cli/replay.go`
- This is how users verify determinism

**Prerequisites:** Story 7.4

---

### Story 7.6: Test Command

As a **developer running tests**,
I want **a test command that runs the conformance harness**,
So that **I can validate my specs against scenarios**.

**Acceptance Criteria:**

**Given** scenario files
**When** I run `nysm test ./specs ./testdata/scenarios`
**Then** scenarios are executed:

```bash
$ nysm test ./specs ./testdata/scenarios
Running 5 scenarios...

✓ cart_checkout_success (0.12s)
✓ cart_checkout_insufficient_stock (0.08s)
✓ cart_add_existing_item (0.05s)
✗ inventory_negative_quantity (0.03s)
    assertion failed: expected quantity >= 0, got -3
✓ cycle_detection (0.15s)

4/5 passed, 1 failed
```

**And** `--update` regenerates golden files
**And** `--filter` runs subset of scenarios
**And** exit code reflects pass/fail

**Technical Notes:**
- Wraps harness package for CLI access
- Command in `internal/cli/test.go`
- Integrates with CI (exit codes)

**Prerequisites:** Story 7.5

---

### Story 7.7: Trace Command

As a **developer debugging flows**,
I want **a trace command that shows provenance**,
So that **I can understand "why did this happen?"**.

**Acceptance Criteria:**

**Given** a flow token
**When** I run `nysm trace --db ./nysm.db --flow 019376f8-...`
**Then** I see the provenance chain:

```bash
$ nysm trace --db ./nysm.db --flow 019376f8-...

Flow: 019376f8-...

Timeline:
  [1] Cart.addItem invoked
      args: {item_id: "widget", quantity: 3}
  [2] Cart.addItem completed (Success)
      result: {item_id: "widget", new_quantity: 3}
  [3] Cart.checkout invoked
  [4] Cart.checkout completed (Success)
  [5] Inventory.reserve invoked
      triggered by: sync "cart-inventory" from [4]
      args: {item_id: "widget", quantity: 3}
  [6] Inventory.reserve completed (Success)

Provenance:
  [5] ← [4] via cart-inventory (binding: {item_id: "widget"})
```

**And** `--format json` outputs structured provenance
**And** `--action` filters to specific action

**Technical Notes:**
- This is the "why did this happen?" query from the paper
- Command in `internal/cli/trace.go`
- Uses provenance edges for backward tracing

**Prerequisites:** Story 7.6

---

### Story 7.8: Demo Concept Specs

As a **developer learning NYSM**,
I want **canonical demo specs for Cart, Inventory, and Web**,
So that **I have a reference implementation to learn from**.

**Acceptance Criteria:**

**Given** the specs directory
**When** I look at the demo specs
**Then** I find:

**specs/cart.concept.cue:**
```cue
concept Cart {
    purpose: "Manages shopping cart state for a user session"

    state CartItem {
        item_id: string
        quantity: int
    }

    action addItem {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { item_id: string, new_quantity: int }
        }]
    }

    action removeItem {
        args: { item_id: string }
        outputs: [{
            case: "Success"
            fields: {}
        }, {
            case: "ItemNotFound"
            fields: { item_id: string }
        }]
    }

    action checkout {
        args: {}
        outputs: [{
            case: "Success"
            fields: { cart_id: string }
        }, {
            case: "EmptyCart"
            fields: {}
        }]
    }

    operational_principle: """
        Adding an item that exists increases quantity.
        Checkout with empty cart returns EmptyCart.
        """
}
```

**And** `specs/inventory.concept.cue` with reserve/release actions
**And** `specs/web.concept.cue` with request/respond actions

**Technical Notes:**
- Implements PRD Phase 1 (canonical demo specs)
- Specs serve as documentation and test fixtures
- Located in `specs/` directory

**Prerequisites:** Story 7.7

---

### Story 7.9: Demo Sync Rules

As a **developer learning NYSM**,
I want **canonical sync rules demonstrating the 3-clause pattern**,
So that **I understand how concepts coordinate**.

**Acceptance Criteria:**

**Given** the demo specs
**When** I look at sync rules
**Then** I find:

**specs/cart-inventory.sync.cue:**
```cue
sync "cart-inventory-reserve" {
    scope: "flow"

    when: Cart.checkout.completed {
        case: "Success"
        bind: { cart_id: result.cart_id }
    }

    where: {
        from: CartItem
        filter: flow_token == bound.flow_token
        bind: { item_id: item_id, quantity: quantity }
    }

    then: Inventory.reserve {
        args: {
            item_id: bound.item_id,
            quantity: bound.quantity
        }
    }
}

sync "handle-insufficient-stock" {
    scope: "flow"

    when: Inventory.reserve.completed {
        case: "InsufficientStock"
        bind: { item: result.item, available: result.available }
    }

    where: {}  // No additional query needed

    then: Cart.checkout.fail {
        args: {
            reason: "insufficient_stock",
            details: { item: bound.item, available: bound.available }
        }
    }
}
```

**And** the syncs demonstrate:
- Normal flow (checkout → reserve)
- Error handling (InsufficientStock → fail)
- Multi-binding (one reserve per cart item)

**Technical Notes:**
- Implements PRD Phase 1 (cart-inventory sync)
- Demonstrates MEDIUM-2 (error matching)
- Shows multi-binding pattern from CRITICAL-1

**Prerequisites:** Story 7.8

---

### Story 7.10: Demo Scenarios and Golden Traces

As a **developer validating the demo**,
I want **test scenarios with golden traces**,
So that **the demo is verified and serves as test fixtures**.

**Acceptance Criteria:**

**Given** the demo specs and syncs
**When** I run `nysm test ./specs ./testdata/scenarios`
**Then** all demo scenarios pass:

**testdata/scenarios/cart_checkout_success.yaml** (from PRD Appendix A)
**testdata/scenarios/cart_checkout_insufficient_stock.yaml** (from PRD Appendix A)

**And** golden traces in `testdata/golden/`:
- `cart_checkout_success.golden`
- `cart_checkout_insufficient_stock.golden`

**And** the golden traces show:
- Complete invocation/completion chain
- Provenance edges
- Final state

**Technical Notes:**
- Validates full MVP end-to-end
- Golden files committed to repo
- Serves as regression test for NYSM development

**Prerequisites:** Story 7.9

---

## FR Coverage Matrix

| Functional Requirement | Epic | Stories | Status |
|----------------------|------|---------|--------|
| **FR-1.1** | Epic 1 | 1.1, 1.6 | Covered |
| **FR-1.2** | Epic 1 | 1.8 | Covered |
| **FR-1.3** | Epic 1 | 1.4, 1.6, 1.7 | Covered |
| **FR-1.4** | Epic 1 | 1.3 | Covered |
| **FR-2.1** | Epic 1, 3 | 1.7, 3.2 | Covered |
| **FR-2.2** | Epic 3 | 3.3, 3.4 | Covered |
| **FR-2.3** | Epic 4 | 4.1, 4.2, 4.3 | Covered |
| **FR-2.4** | Epic 4 | 4.4, 4.5 | Covered |
| **FR-2.5** | Epic 4 | 4.1, 4.2 | Covered |
| **FR-3.1** | Epic 3 | 3.5 | Covered |
| **FR-3.2** | Epic 3 | 3.6 | Covered |
| **FR-3.3** | Epic 3 | 3.7 | Covered |
| **FR-4.1** | Epic 2 | 2.5, 2.6 | Covered |
| **FR-4.2** | Epic 5 | 5.1, 5.2 | Covered |
| **FR-4.3** | Epic 2 | 2.7 | Covered |
| **FR-5.1** | Epic 2 | 2.1, 2.2, 2.3 | Covered |
| **FR-5.2** | Epic 2 | 2.5, 2.6 | Covered |
| **FR-5.3** | Epic 2, 7 | 2.7, 7.5 | Covered |
| **FR-6.1** | Epic 6 | 6.1, 6.2 | Covered |
| **FR-6.2** | Epic 6 | 6.3, 6.4 | Covered |
| **FR-6.3** | Epic 6 | 6.5 | Covered |
| **FR-6.4** | Epic 6 | 6.6 | Covered |

**Critical Architecture Decisions Coverage:**

| Decision | Epic | Stories |
|----------|------|---------|
| CRITICAL-1 (Binding-Level Idempotency) | Epic 2, 5 | 2.5, 5.1, 5.2 |
| CRITICAL-2 (Logical Identity) | Epic 1, 2 | 1.5, 2.2 |
| CRITICAL-3 (Termination) | Epic 5 | 5.3, 5.4, 5.5 |
| HIGH-1 (Scoping Modes) | Epic 3 | 3.7 |
| HIGH-2 (Query Abstraction) | Epic 4 | 4.1, 4.2 |
| HIGH-3 (Security) | Epic 1, 2 | 1.1, 2.3 |
| CP-1 through CP-6 | All | Throughout |

---

## Summary

**Total Epics:** 7
**Total Stories:** 45

| Epic | Title | Stories |
|------|-------|---------|
| 1 | Foundation & IR Core | 8 |
| 2 | Durable Event Store | 7 |
| 3 | Sync Engine Core | 7 |
| 4 | Query & Binding System | 5 |
| 5 | Idempotency & Cycle Safety | 5 |
| 6 | Conformance Harness | 6 |
| 7 | CLI & Demo Application | 10 |

**FR Coverage:** 18/18 functional requirements (100%)
**Critical Decision Coverage:** All 9 critical decisions addressed
**Critical Pattern Coverage:** All 6 critical patterns (CP-1 through CP-6) implemented

---

_For implementation: Use the `create-story` workflow to generate individual story implementation plans from this epic breakdown._

_Each story is sized for completion by a single dev agent in one focused session._

