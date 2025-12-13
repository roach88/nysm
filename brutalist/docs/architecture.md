---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
inputDocuments:
  - docs/prd.md
  - docs/analysis/product-brief-nysm-2025-12-12.md
  - initial-report.md
workflowType: 'architecture'
lastStep: 8
status: 'complete'
completedAt: '2025-12-12'
project_name: 'NYSM'
user_name: 'Tyler'
date: '2025-12-12'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
NYSM requires 6 interconnected subsystems forming a compiler + runtime pipeline:

1. **Concept Specification System (FR-1)** - Parses CUE surface format, validates against schema, compiles to canonical JSON IR. This is the "front-end" of the framework.

2. **Synchronization System (FR-2)** - The reactive core. Matches action completions against `when` clauses, executes `where` queries to produce bindings, generates `then` invocations. Must maintain abstraction boundary for future SPARQL migration.

3. **Flow Token System (FR-3)** - Request correlation mechanism. Every invocation/completion carries a flow token; sync rules only fire for same-flow records.

4. **Provenance System (FR-4)** - Structural audit trail. Records (completion, sync-id) → invocation edges enabling idempotency checks and deterministic replay.

5. **Durable Engine (FR-5)** - SQLite-backed append-only log for invocations, completions, and provenance edges. Must support crash recovery with identical replay results.

6. **Conformance Harness (FR-6)** - Test framework that validates operational principles as executable contract tests. Supports golden trace snapshots.

**Non-Functional Requirements:**
- **Legibility** (NFR-1): Artifacts must be 100% self-documenting with no implicit behavior
- **Correctness** (NFR-2): Deterministic replay, idempotent sync firing, isolated flow scoping
- **Extensibility** (NFR-3): Versioned IR schemas, abstracted query substrate, abstracted surface format
- **Developer Experience** (NFR-4): Actionable errors, traceable evaluation, diff-based test output

**Scale & Complexity:**
- Primary domain: Language tooling / Runtime engine
- Complexity level: Medium
- Estimated architectural components: 6 major subsystems + shared IR layer

### Technical Constraints & Dependencies

**From PRD Technical Decisions:**

| Constraint | Choice | Implication |
|------------|--------|-------------|
| Surface format | CUE | Leverage CUE tooling for parsing/validation |
| Canonical IR | JSON Schema | Version-controlled, language-agnostic contract |
| Query substrate | SQL (v0) | Compile where-clause to SQL; abstract for SPARQL later |
| Storage | SQLite | Embeddable, append-only log pattern |

**From Paper Constraints:**
- No transactions - integrity via rule scheme + flow scoping + error-matching
- No getters - reads via query only, writes via actions only
- Named args + typed outputs - enables deterministic replay

### Cross-Cutting Concerns Identified

1. **Canonical IR as System Contract**
   - All subsystems read/write IR types
   - IR versioning affects every component
   - Breaking IR changes require coordinated updates

2. **Flow Token Propagation**
   - Must flow through compiler, engine, sync, and storage layers
   - All queries must be flow-scoped

3. **Error Type Propagation**
   - Action signatures define typed error cases
   - Sync engine must match and propagate errors correctly
   - Conformance harness must validate error paths

4. **Abstraction Boundaries**
   - Query interface must hide SQL vs SPARQL
   - Surface format compiler must target stable IR
   - These boundaries are architectural firewalls

---

## Critical Design Issues (from Codex Review)

External review identified critical gaps in the PRD that must be addressed architecturally.

### CRITICAL-1: Idempotency Schema Blocks Multi-Invocation

**Problem:** The PRD's schema uses `UNIQUE(completion_id, sync_id)` for idempotency, but a single sync rule can emit *multiple* invocations when the `where` clause returns multiple bindings. The current schema would either fail inserts, silently drop actions, or force single-binding semantics.

**Impact:** Breaks the fundamental `when→where→then` semantics where `where` produces a *set* of bindings.

**Architectural Decision:**
Model idempotency at **binding granularity**, not sync granularity:

```sql
-- Sync firings track each binding that triggered
CREATE TABLE sync_firings (
  id INTEGER PRIMARY KEY,
  completion_id INTEGER REFERENCES completions(id),
  sync_id TEXT NOT NULL,
  binding_hash TEXT NOT NULL,  -- Hash of the binding values
  created_at INTEGER NOT NULL, -- Logical clock, not wall time
  UNIQUE(completion_id, sync_id, binding_hash)  -- Idempotency per binding
);

-- Provenance edges link firings to invocations (1:1)
CREATE TABLE provenance_edges (
  id INTEGER PRIMARY KEY,
  sync_firing_id INTEGER REFERENCES sync_firings(id),
  invocation_id INTEGER REFERENCES invocations(id),
  UNIQUE(sync_firing_id)  -- Each firing produces exactly one invocation
);
```

**Rationale:** A completion + sync + binding triple uniquely identifies a "should we fire?" decision. Multiple bindings from the same where-clause each get their own firing record.

---

### CRITICAL-2: Deterministic Replay Requires Logical Identity

**Problem:** The PRD uses `CURRENT_TIMESTAMP`, auto-increment IDs, and doesn't specify query ordering. This breaks the "crash/restart replay produces identical results" guarantee and makes golden trace snapshots drift.

**Impact:** NFR-2.1 (deterministic replay) and FR-6.4 (golden snapshots) are unachievable.

**Architectural Decisions:**

1. **Logical Identity** - Records identified by content-addressed hash or explicit UUID stored in-log, not auto-increment:
   ```
   invocation_id = hash(flow_token, action_uri, canonical_json(args), sequence_number)
   ```

2. **Logical Clock** - Monotonic sequence per flow (or per engine), not wall-clock:
   ```sql
   created_at INTEGER NOT NULL  -- Sequence number, not timestamp
   ```

3. **Canonical JSON Encoding** - All args/results serialized with deterministic key ordering (sorted keys, no whitespace variance).

4. **Deterministic Query Ordering** - All `where` clause queries must include `ORDER BY` over a stable key. Binding sets are ordered, not just sets.

**Rationale:** "Identical results" means byte-identical logs and behaviorally equivalent traces. This requires removing all sources of non-determinism.

---

### CRITICAL-3: Sync Engine Termination Semantics

**Problem:** Self-triggering or mutually-recursive sync rules can loop infinitely or cause binding explosions. The PRD has no formal evaluation semantics.

**Impact:** The sync engine (the "heart" of NYSM) has undefined behavior for non-trivial rule sets.

**Architectural Decisions:**

1. **Cycle Detection Per Flow** - Track sync rule firing history within a flow; detect when same (sync_id, binding_hash) would fire twice.

2. **Stratification Analysis** - At compile time, analyze sync rules for potential cycles. Warn on cycles, require explicit `@allow_recursion` annotation if intentional.

3. **Max-Steps Quota** - Runtime limit on sync firings per flow (configurable, default 1000). Exceeding quota is a fatal error for that flow.

4. **Deterministic Scheduling Policy** - Define explicit evaluation order:
   - FIFO queue of pending completions
   - For each completion, evaluate matching syncs in declaration order
   - For each sync, process bindings in query result order
   - Generated invocations enqueue their completions

**Rationale:** Without formal termination semantics, the sync engine is a footgun. These constraints make behavior predictable and debuggable.

---

### HIGH-1: Flow Token Scoping Modes

**Problem:** "Syncs only match same-flow records" is too restrictive. Real systems need cross-flow coordination (inventory reservations, rate limits, deduplication across users).

**Impact:** Developers will work around the restriction with ugly hacks that reintroduce accidental joins.

**Architectural Decision:**
Introduce explicit **scoping modes** for sync rules:

```cue
sync "cart-inventory" {
  scope: "flow"  // Default: only same-flow records
  when: Cart.checkout.completed
  where: ...
  then: ...
}

sync "inventory-global-check" {
  scope: "global"  // Matches across all flows
  when: Inventory.reserve.invoked
  where: ...
  then: ...
}

sync "rate-limit-per-user" {
  scope: keyed("user_id")  // Matches within same user_id
  when: API.request.completed
  where: ...
  then: ...
}
```

**Scoping Modes:**
- `flow` (default) - Only records with same flow_token
- `global` - All records regardless of flow
- `keyed(field)` - Records sharing same value for specified field

**Rationale:** Make the join semantics explicit in the sync rule declaration rather than implicit in the engine.

---

### HIGH-2: Query IR Abstraction Boundary

**Problem:** Mapping the `where` DSL "too directly" to SQL bakes in closed-world assumptions, NULL behavior, and non-graph joins that won't round-trip to SPARQL.

**Impact:** SPARQL migration becomes a rewrite rather than a backend swap.

**Architectural Decision:**
Insert a **Query IR (relational algebra)** layer between DSL and backends:

```
[where DSL] → [Query IR] → [SQL Backend]
                        → [SPARQL Backend] (future)
```

**Query IR Constraints:**
- No NULLs in the portable fragment (use explicit Option types)
- No outer joins (inner joins only in portable fragment)
- Set semantics (not bag/multiset)
- Explicit variable binding (no implicit column selection)

**Portable Fragment:**
```
QueryIR =
  | Select(bindings, source, predicate)
  | Join(left, right, on)
  | Union(queries)
  | Project(query, fields)
  | Filter(query, predicate)

Predicate =
  | Equals(field, value)
  | And(predicates)
  | Or(predicates)
  | Exists(subquery)
```

**Rationale:** The Query IR is the contract. SQL and SPARQL backends implement this contract. DSL features outside the portable fragment are SQL-only and documented as such.

---

### HIGH-3: Security Model Foundation

**Problem:** Provenance + query substrate creates an easy data exfiltration path. SQL compilation without parameterization risks injection. No tenant boundaries defined.

**Impact:** NYSM applications would be insecure by default.

**Architectural Decision:**
Treat security as first-class in the IR and engine:

1. **Parameterized Queries Only** - Query IR compiles to parameterized SQL. No string interpolation ever.

2. **Authz in IR** - Actions can declare required permissions:
   ```cue
   action checkout {
     requires: ["cart:write", "inventory:read"]
     args: { ... }
     outputs: { ... }
   }
   ```

3. **Provenance Redaction** - Field-level policies for what appears in provenance:
   ```cue
   state CartItem {
     item_id: string
     quantity: int
     price: money @redact_from_provenance  // PII/sensitive
   }
   ```

4. **Flow Token ≠ Security Boundary** - Explicit tenant/user context separate from flow correlation:
   ```
   Invocation {
     flow_token: "...",      // Correlation
     security_context: {     // Authz
       tenant_id: "...",
       user_id: "...",
       permissions: [...]
     }
   }
   ```

**Rationale:** Security can't be bolted on later. Even for MVP, the architecture must have hooks for authz, even if enforcement is minimal.

---

### MEDIUM-1: State Model Clarification

**Problem:** Unclear relationship between append-only event log and "relational state tables."

**Architectural Decision:**
**Event-sourced projections** - State tables are materialized views derived from the event log:

1. **Event Log is Authoritative** - Invocations, completions, and provenance edges are the source of truth.

2. **State Tables are Projections** - Built by replaying completions through projection functions:
   ```
   projection CartItems {
     on Cart.addItem.completed(item, qty) {
       UPSERT cart_items SET quantity = quantity + qty WHERE item_id = item
     }
     on Cart.removeItem.completed(item) {
       DELETE FROM cart_items WHERE item_id = item
     }
   }
   ```

3. **Rebuildable** - State tables can be rebuilt from event log at any time (for recovery, migration, or debugging).

4. **Action Implementations Query Projections** - Actions read from state tables (projections), write by emitting completions.

**Rationale:** This is the only model consistent with "deterministic replay" and "no transactions."

---

### MEDIUM-2: Error Matching in When-Clause

**Problem:** The `when` clause IR doesn't explicitly support matching on output cases (success vs typed error variants).

**Architectural Decision:**
Extend `WhenClause` to match output cases:

```cue
sync "handle-insufficient-stock" {
  when: Inventory.reserve.completed {
    case: "InsufficientStock"  // Match specific error variant
    bind: { item: result.item, requested: result.requested, available: result.available }
  }
  where: ...
  then: Cart.checkout.fail(reason: "insufficient_stock", details: bound.item)
}

sync "on-successful-reserve" {
  when: Inventory.reserve.completed {
    case: "Success"  // Match success case
    bind: { reservation_id: result.reservation_id }
  }
  where: ...
  then: ...
}
```

**IR Extension:**
```
WhenClause {
  action: ActionRef
  event: "invoked" | "completed"
  output_case: string | null  // null = match any case
  bindings: Record<string, ResultPath>
}
```

**Rationale:** Error matching is central to the paper's "no transactions" integrity model. Without it, error propagation is impossible.

---

### MEDIUM-3: Naming and Versioning Scheme

**Problem:** No URI scheme or version markers defined for long-lived replay and "why did this happen?" queries.

**Architectural Decision:**
Define explicit naming and versioning:

1. **URI Scheme:**
   ```
   nysm://{namespace}/{concept}/{action}
   nysm://{namespace}/{concept}/{action}@{version}

   Examples:
   nysm://myapp/Cart/addItem
   nysm://myapp/Cart/addItem@1.2.0
   nysm://myapp/sync/cart-inventory@1.0.0
   ```

2. **Version Markers on Records:**
   ```
   Invocation {
     action_uri: "nysm://myapp/Cart/addItem@1.2.0"
     spec_hash: "sha256:abc123..."  // Hash of concept spec at invoke time
     engine_version: "0.1.0"
   }
   ```

3. **Spec Hash Persistence** - Every invocation/completion records the hash of the concept spec that defined the action. Enables "which version of the rules produced this?"

**Rationale:** Without versioning, you can't answer "why did this happen?" for old events after specs change.

---

## Architecture Principles (Derived)

Based on the requirements and critical issues, these principles will guide all architectural decisions:

1. **Determinism is Non-Negotiable** - Every source of non-determinism (timestamps, IDs, ordering) must be eliminated or controlled.

2. **Explicit Over Implicit** - Scoping modes, error matching, and coordination semantics must be declared, not inferred.

3. **Event Log is Truth** - State is derived, never authoritative. Replay must be possible.

4. **Abstraction Boundaries are Firewalls** - Query IR, Surface Format IR, and Storage interfaces are contracts that hide implementation.

5. **Security Hooks from Day One** - Even minimal MVP must have authz and redaction extension points.

6. **Termination is Guaranteed** - Sync engine must provably terminate for all inputs.

---

## Technology Stack

### Language Selection: Go

**Decision:** Go is the primary implementation language for NYSM.

**Rationale:**
1. **Native CUE Integration** - CUE is written in Go; `cuelang.org/go` provides full API access without subprocess overhead or WASM limitations
2. **Determinism-Friendly** - Go's stronger type system and constraints reduce accidental non-determinism compared to dynamic languages
3. **Excellent CLI Tooling** - Cobra, Bubble Tea, and the Go ecosystem excel at developer-facing CLI tools
4. **Performance** - Suitable for processing large event logs and running conformance tests
5. **Single Binary** - Easy distribution without runtime dependencies

**Trade-off Accepted:** Slower iteration than TypeScript, but correctness constraints outweigh velocity for this project.

### Core Dependencies

| Component | Package | Version | Purpose |
|-----------|---------|---------|---------|
| **CUE SDK** | `cuelang.org/go` | v0.15.1 | Parse/validate CUE specs, compile to IR |
| **SQLite** | `github.com/mattn/go-sqlite3` | v1.14.32 | Durable storage (requires CGO) |
| **SQLite (alt)** | `modernc.org/sqlite` | v1.40.1 | Pure Go alternative (no CGO) |
| **CLI Framework** | `github.com/spf13/cobra` | v1.10.2 | Command-line interface |
| **CLI TUI (future)** | `github.com/charmbracelet/bubbletea` | v1.3.10 | Interactive trace explorer |

### Testing Dependencies

| Component | Package | Version | Purpose |
|-----------|---------|---------|---------|
| **Assertions** | `github.com/stretchr/testify` | v1.11.1 | Test assertions and mocks |
| **Struct Diffs** | `github.com/google/go-cmp` | v0.7.0 | Readable struct comparisons |
| **Golden Files** | `github.com/sebdah/goldie/v2` | v2.8.0 | Snapshot testing for traces |
| **CLI Testing** | `github.com/rogpeppe/go-internal/testscript` | v1.14.1 | Integration test fixtures |

### CUE Integration Strategy

**Decision:** Use CUE's Go API directly (not CLI subprocess, not WASM).

**Rationale:**
- Full control over compilation and error reporting
- Rich error messages for developer experience (NFR-4)
- No subprocess overhead for each compile
- Access to CUE's validation and constraint system

**Implementation:**
```go
import (
    "cuelang.org/go/cue"
    "cuelang.org/go/cue/cuecontext"
    "cuelang.org/go/cue/load"
)

// Load and compile concept specs
ctx := cuecontext.New()
instances := load.Instances([]string{"./specs"}, nil)
value := ctx.BuildInstance(instances[0])
```

### SQLite Configuration

**Required Settings:**
```go
db.Exec(`
    PRAGMA journal_mode = WAL;           -- Concurrent reads during writes
    PRAGMA synchronous = NORMAL;         -- Balance durability/performance
    PRAGMA busy_timeout = 5000;          -- Wait for locks
    PRAGMA foreign_keys = ON;            -- Enforce referential integrity
`)
```

**Key Requirements:**
- WAL mode for concurrent read access during sync engine execution
- Prepared statements only (parameterized queries per HIGH-3)
- JSON functions for querying args/results columns
- No ORM - direct `database/sql` for control

### Project Structure

```
nysm/
├── cmd/
│   └── nysm/
│       └── main.go              # CLI entry point
├── internal/
│   ├── compiler/                # CUE → Canonical IR
│   │   ├── compiler.go          # Main compilation logic
│   │   ├── concept.go           # ConceptSpec parsing
│   │   ├── sync.go              # SyncRule parsing
│   │   └── validate.go          # Schema validation
│   │
│   ├── ir/                      # Canonical IR Types
│   │   ├── types.go             # ConceptSpec, ActionSig, SyncRule, etc.
│   │   ├── json.go              # Canonical JSON encoding (sorted keys)
│   │   ├── hash.go              # Content-addressed identity
│   │   └── version.go           # IR schema versioning
│   │
│   ├── engine/                  # Sync Engine + Scheduler
│   │   ├── engine.go            # Main engine loop
│   │   ├── scheduler.go         # FIFO queue, deterministic ordering
│   │   ├── matcher.go           # When-clause matching
│   │   ├── cycle.go             # Cycle detection per flow
│   │   └── quota.go             # Max-steps enforcement
│   │
│   ├── store/                   # SQLite Storage Layer
│   │   ├── store.go             # Database connection, migrations
│   │   ├── schema.sql           # Table definitions
│   │   ├── invocations.go       # Invocation CRUD
│   │   ├── completions.go       # Completion CRUD
│   │   ├── provenance.go        # Provenance edge queries
│   │   └── projections.go       # State table management
│   │
│   ├── queryir/                 # Query IR Types
│   │   ├── types.go             # Select, Join, Filter, etc.
│   │   └── validate.go          # Portable fragment validation
│   │
│   ├── querysql/                # Query IR → SQL Backend
│   │   ├── compile.go           # IR to parameterized SQL
│   │   └── ordering.go          # Deterministic ORDER BY
│   │
│   └── harness/                 # Conformance Test Runner
│       ├── harness.go           # Test execution
│       ├── scenario.go          # Scenario loading
│       ├── assertions.go        # Trace assertions
│       └── golden.go            # Snapshot comparison
│
├── specs/                       # Demo Concept Specs
│   ├── cart.concept.cue
│   ├── inventory.concept.cue
│   ├── web.concept.cue
│   └── cart-inventory.sync.cue
│
├── testdata/                    # Test Fixtures
│   ├── golden/                  # Golden trace snapshots
│   └── scenarios/               # Test scenario definitions
│
├── go.mod
├── go.sum
└── README.md
```

### CLI Commands

```
nysm compile <specs-dir>     # Compile CUE specs to canonical IR
nysm validate <specs-dir>    # Validate specs without full compile
nysm run <db> <specs-dir>    # Start engine with specs
nysm replay <db>             # Replay event log from scratch
nysm test <specs-dir>        # Run conformance harness
nysm trace <db> <flow-id>    # Query provenance for a flow
```

### Testing Strategy

| Layer | Approach | Tools |
|-------|----------|-------|
| **Unit Tests** | Table-driven tests for each package | `testing`, `testify` |
| **IR Canonicalization** | Fuzz testing for JSON stability | Go fuzzing |
| **Query Compilation** | Property tests for parameterization | Go fuzzing |
| **Engine Determinism** | Replay invariant tests | Custom assertions |
| **Golden Traces** | Snapshot tests for demo scenarios | `goldie` |
| **CLI Integration** | Script-based end-to-end tests | `testscript` |

### Build Configuration

**go.mod:**
```go
module github.com/tyler/nysm

go 1.25

require (
    cuelang.org/go v0.15.1
    github.com/mattn/go-sqlite3 v1.14.32
    github.com/spf13/cobra v1.10.2
    github.com/stretchr/testify v1.11.1
    github.com/google/go-cmp v0.7.0
    github.com/sebdah/goldie/v2 v2.8.0
)
```

**Build Commands:**
```bash
# Development build
go build -o nysm ./cmd/nysm

# Production build (with CGO for SQLite)
CGO_ENABLED=1 go build -ldflags="-s -w" -o nysm ./cmd/nysm

# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Fuzz testing
go test -fuzz=FuzzCanonicalJSON ./internal/ir
```

---

## Core Architectural Decisions

### Decision Summary

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Canonical JSON | RFC 8785 semantics | Cross-language stability, deterministic hashing |
| Number Policy | No floats in IR/log | Eliminates formatting drift |
| Hashing | SHA-256 + domain separation | Collision = correctness bug in append-only systems |
| Flow Tokens | UUIDv7 (injectable) | Time-sortable, standard, testable |
| Logging | log/slog | Stdlib, sufficient for CLI |
| Security MVP | Basic + redaction | Log is immutable, redact at write time |
| Concurrency | Single-writer event loop | Deterministic scheduling |

### Data Serialization

#### Canonical JSON (RFC 8785)

**Decision:** Implement RFC 8785 JSON Canonicalization Scheme semantics.

**Rules:**
1. **Sorted keys** - Object keys sorted lexicographically by UTF-16 code units
2. **No floats** - Use `int64`, strings, or fixed-point (money in minor units)
3. **No HTML escaping** - Disable `<>&` escaping for cross-language stability
4. **Unicode NFC normalization** - Normalize at ingestion boundaries
5. **No whitespace** - Compact output only

**Implementation:**
```go
// internal/ir/json.go
func MarshalCanonical(v any) ([]byte, error) {
    // 1. Marshal to map[string]any
    // 2. Recursively sort keys
    // 3. Marshal with no HTML escaping, no indent
    // 4. Return deterministic bytes
}
```

**Validation:** Fuzz tests verify that `MarshalCanonical(Unmarshal(MarshalCanonical(x))) == MarshalCanonical(x)`

#### Content-Addressed Identity

**Decision:** SHA-256 with explicit domain separation.

**Format:**
```
invocation_id = sha256("nysm/invocation/v1\x00" + canonical_json({
    flow_token: "...",
    action_uri: "...",
    args: {...},
    seq: N
}))

completion_id = sha256("nysm/completion/v1\x00" + canonical_json({
    invocation_id: "...",
    output_case: "...",
    result: {...},
    seq: N
}))

binding_hash = sha256("nysm/binding/v1\x00" + canonical_json(binding_values))
```

**Rationale:**
- Domain separation prevents cross-type collisions
- Version prefix allows future hash algorithm migration
- SHA-256 is "overkill" but eliminates collision as a correctness concern

### Flow Token Generation

**Decision:** UUIDv7 with injectable generation for deterministic tests.

**Library:** `github.com/google/uuid`

**Implementation:**
```go
// internal/engine/flow.go
type FlowTokenGenerator interface {
    Generate() string
}

type UUIDv7Generator struct{}
func (g UUIDv7Generator) Generate() string {
    return uuid.Must(uuid.NewV7()).String()
}

type FixedGenerator struct { tokens []string; idx int }
func (g *FixedGenerator) Generate() string {
    // Return predetermined tokens for golden tests
}
```

**Storage:** Store as 16-byte BLOB in SQLite for index efficiency; render as text at API boundaries.

### Structured Logging

**Decision:** Use `log/slog` with consistent correlation keys.

**Standard Keys:**
```go
slog.String("flow_token", flowToken),
slog.String("invocation_id", invocationID),
slog.String("completion_id", completionID),
slog.String("sync_id", syncID),
slog.String("binding_hash", bindingHash),
slog.Int64("seq", sequenceNumber),
slog.String("event", eventType),  // "invoke", "complete", "fire", "skip"
```

**Test Configuration:**
```go
// Omit timestamps for deterministic log snapshots
handler := slog.NewTextHandler(w, &slog.HandlerOptions{
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        if a.Key == slog.TimeKey {
            return slog.Attr{}  // Remove timestamp
        }
        return a
    },
})
```

**Principle:** Logs are debugging output, not source of truth. The append-only event log is truth.

### Security Model (MVP Scope)

**Decision:** Basic security context + provenance redaction. Authz enforcement deferred.

#### What's Implemented in MVP

| Component | MVP Scope |
|-----------|-----------|
| **Parameterized Queries** | Required - Query IR always emits `(sql, []args)` |
| **Security Context** | Recorded on all invocations/completions |
| **Provenance Redaction** | Applied at write time (log is immutable) |
| **Authz Enforcement** | Deferred - context recorded but not checked |

#### Security Context Schema

```go
type SecurityContext struct {
    TenantID    string   `json:"tenant_id,omitempty"`
    UserID      string   `json:"user_id,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
}

type Invocation struct {
    // ... other fields
    SecurityContext *SecurityContext `json:"security_context,omitempty"`
}
```

#### Provenance Redaction

**Decision:** Apply redaction at write time, not read time.

```cue
state CartItem {
    item_id: string
    quantity: int
    credit_card: string @redact  // Never written to provenance
}
```

**Implementation:**
- Compiler extracts `@redact` annotations from state schema
- Engine filters redacted fields before writing to completions table
- Original values never enter the append-only log

**Rationale:** The log is immutable. If sensitive data enters, it cannot be removed. Redact at ingestion.

#### Security Assumptions (MVP)

- Single-tenant deployment
- Trusted concept specs only (no untrusted user-provided specs)
- Authz enforcement added when multi-tenant support required

### Concurrency Model

**Decision:** Single-writer event loop with deterministic scheduling.

**Architecture:**
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

**Rules:**
1. **Single writer** - One goroutine writes to SQLite
2. **FIFO queue** - Invocations/completions processed in arrival order
3. **Sync evaluation order** - Rules evaluated in declaration order
4. **Binding order** - Query results processed in ORDER BY order
5. **No parallelism in core loop** - Determinism over throughput

**Action Execution:**
- Actions may execute in parallel (I/O bound)
- Completions enqueue back to the single-writer loop
- Engine sees completions in deterministic order

### Version Pinning

**Decision:** Record version information on all event log entries.

**Fields on Every Record:**
```go
type RecordMetadata struct {
    EngineVersion string `json:"engine_version"`  // "0.1.0"
    IRVersion     string `json:"ir_version"`      // "1"
    SpecHash      string `json:"spec_hash"`       // SHA-256 of compiled specs
}
```

**Rationale:**
- Enables "which version of the rules produced this?"
- Supports replay across engine upgrades
- Enables migration tooling to detect version boundaries

### Additional Dependencies

Based on decisions above, additional dependencies:

```go
require (
    github.com/google/uuid v1.6.0  // UUIDv7 flow tokens
)
```

No additional dependencies needed - SHA-256, slog, and JSON are all stdlib.

### Decision Impact Analysis

**Implementation Sequence:**
1. `internal/ir/json.go` - Canonical JSON (RFC 8785)
2. `internal/ir/hash.go` - Content-addressed identity
3. `internal/store/schema.sql` - Logical clocks, security context, version metadata
4. `internal/engine/flow.go` - Injectable flow token generation
5. `internal/engine/engine.go` - Single-writer event loop
6. `internal/compiler/redact.go` - Provenance redaction extraction

**Cross-Component Dependencies:**
- Canonical JSON affects: IR, store, harness (golden snapshots)
- Content-addressed IDs affect: store, engine, provenance queries
- Security context affects: store schema, IR types, engine
- Single-writer loop affects: engine, store (single connection)

---

## Implementation Patterns & Consistency Rules

These patterns ensure multiple AI agents write compatible, consistent code that works together seamlessly. All patterns address potential conflict points where agents could make different choices.

### Critical Pattern Requirements (from Codex Review)

These patterns are **mandatory** for correctness, not stylistic preferences:

#### CP-1: Binding-Level Idempotency

**Requirement:** All sync firing operations MUST use binding-granular uniqueness.

```sql
-- CORRECT: Idempotency per binding
CREATE TABLE sync_firings (
  id INTEGER PRIMARY KEY,
  completion_id INTEGER REFERENCES completions(id),
  sync_id TEXT NOT NULL,
  binding_hash TEXT NOT NULL,  -- REQUIRED: Hash of binding values
  seq INTEGER NOT NULL,        -- Logical clock, NOT wall time
  UNIQUE(completion_id, sync_id, binding_hash)
);

-- WRONG: Blocks multi-binding syncs
CREATE TABLE sync_firings (
  ...
  UNIQUE(completion_id, sync_id)  -- ❌ NEVER do this
);
```

**Enforcement:** Store layer rejects any sync firing without `binding_hash`.

#### CP-2: Logical Identity and Time

**Requirement:** No wall-clock timestamps in persisted records. All identity is content-addressed.

```go
// CORRECT: Logical clock
type Invocation struct {
    ID        string `json:"id"`         // Content-addressed hash
    Seq       int64  `json:"seq"`        // Monotonic per-flow counter
    FlowToken string `json:"flow_token"`
    // ...
}

// WRONG: Wall-clock time
type Invocation struct {
    CreatedAt time.Time `json:"created_at"` // ❌ NEVER persist wall time
    ID        int64     `json:"id"`          // ❌ NEVER use auto-increment as identity
}
```

**Enforcement:** IR types have no `time.Time` fields. Store schema uses `INTEGER` for `seq`, not `TIMESTAMP`.

#### CP-3: RFC 8785 UTF-16 Key Ordering

**Requirement:** Canonical JSON key sorting MUST use UTF-16 code unit ordering, not Go's default UTF-8 byte ordering.

```go
import "unicode/utf16"

// CORRECT: UTF-16 code unit comparison using unicode/utf16.Encode
// CRITICAL: Must convert entire strings to UTF-16, not compare rune-by-rune.
// Supplementary characters (U+10000+) become surrogate pairs in UTF-16.
func compareKeysRFC8785(a, b string) int {
    // Convert to UTF-16 code units - handles surrogate pairs correctly
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

// WRONG: Default Go string comparison
sort.Strings(keys) // ❌ UTF-8 byte order differs from UTF-16

// Example where UTF-8 and UTF-16 differ:
// U+E000 vs U+10000:
//   UTF-8:  0xEE < 0xF0, so U+E000 < U+10000
//   UTF-16: 0xE000 > 0xD800, so U+E000 > U+10000 ← DIFFERENT!
```

**Enforcement:** Cross-language fixture tests in `testdata/fixtures/rfc8785/` verify canonicalization.

#### CP-4: Deterministic Query Ordering

**Requirement:** ALL where-clause queries MUST include `ORDER BY` over a stable key.

```go
// CORRECT: Explicit ordering
func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error) {
    sql := c.compileSelect(q)
    // MANDATORY: Every query gets ORDER BY
    sql += " ORDER BY " + c.stableOrderKey(q)
    return sql, params, nil
}

// WRONG: Relying on implicit ordering
func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error) {
    return c.compileSelect(q), params, nil // ❌ Missing ORDER BY
}
```

**Enforcement:** QueryIR→SQL compiler always appends `ORDER BY`. Tests verify ordering consistency.

#### CP-5: Constrained Value Types

**Requirement:** No `map[string]any` with unconstrained types. Use explicit IR value model.

```go
// CORRECT: Constrained IR value type
type IRValue interface {
    irValue() // Sealed interface
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

// No IRFloat - floats are forbidden in IR

// WRONG: Unconstrained any
type OutputCase struct {
    Fields map[string]any `json:"fields"` // ❌ Admits floats, inconsistent types
}
```

**Enforcement:** IR package exports only constrained types. No `any` in public signatures.

#### CP-6: Security Context on All Records

**Requirement:** Every invocation and completion MUST carry security context, even if empty in MVP.

```go
// CORRECT: Security context always present
type Invocation struct {
    // ... other fields
    SecurityContext SecurityContext `json:"security_context"` // Always present
}

type SecurityContext struct {
    TenantID    string   `json:"tenant_id,omitempty"`
    UserID      string   `json:"user_id,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
}

// WRONG: Optional security context
type Invocation struct {
    SecurityContext *SecurityContext `json:"security_context,omitempty"` // ❌ Nullable
}
```

### Naming Patterns

#### SQLite Tables and Columns

| Element | Convention | Example |
|---------|------------|---------|
| Tables | `snake_case`, plural | `invocations`, `completions`, `sync_firings` |
| Columns | `snake_case` | `flow_token`, `action_uri`, `binding_hash` |
| Foreign keys | `{table_singular}_id` | `invocation_id`, `completion_id` |
| Indexes | `idx_{table}_{columns}` | `idx_completions_flow_token` |
| Constraints | `{table}_{type}_{columns}` | `sync_firings_unique_binding` |
| Logical clock | `seq` | Never `created_at`, `timestamp`, or similar |

#### Go Code Naming

| Element | Convention | Example |
|---------|------------|---------|
| Packages | `lowercase`, single word | `engine`, `store`, `compiler`, `ir` |
| Exported types | `PascalCase` | `ConceptSpec`, `SyncRule`, `FlowToken` |
| Exported functions | Package-qualified verbs | `compiler.Compile`, `engine.Run`, `store.Write` |
| Internal functions | `camelCase` | `parseActionSig`, `bindWhereClause` |
| Error variables | `Err` prefix | `ErrInvalidSpec`, `ErrCyclicSync` |
| Error types | `Error` suffix | `ValidationError`, `CompilationError` |
| Interfaces | `-er` suffix or descriptive | `FlowTokenGenerator`, `QueryCompiler` |

#### IR Field Names (JSON)

| Context | Convention | Example |
|---------|------------|---------|
| JSON fields | `snake_case` | `flow_token`, `action_uri`, `output_case` |
| Go struct tags | Match JSON | `json:"flow_token"` |
| Never | `camelCase` in JSON | ❌ `flowToken` |

#### URI Scheme (Unified)

**Decision:** Use `nysm://` scheme with `@semver` versioning (not `nysm:` with `/v{N}`).

```
nysm://{namespace}/{type}/{name}@{semver}

Examples:
nysm://myapp/concept/Cart@1.0.0
nysm://myapp/action/Cart/addItem@1.0.0
nysm://myapp/sync/cart-inventory@1.0.0
```

**Rules:**
- Namespace: lowercase, alphanumeric + hyphens
- Type: `concept`, `action`, `sync`, `state`
- Name: PascalCase for concepts, camelCase for actions
- Version: Semver (major.minor.patch)
- Case-sensitive: Yes

**Rationale:** Single canonical form prevents parsing ambiguity and ensures stable hashing inputs.

#### CLI Flags

| Type | Convention | Example |
|------|------------|---------|
| Long flags | `kebab-case` | `--flow-token`, `--spec-dir`, `--output-format` |
| Short flags | Single letter | `-v` (verbose), `-o` (output), `-q` (quiet) |
| Boolean flags | Positive form | `--verbose` not `--no-quiet` |

#### Test Files

| Type | Location | Naming |
|------|----------|--------|
| Unit tests | Co-located | `{file}_test.go` |
| Golden files | `testdata/{pkg}/` | `{scenario}.golden` |
| Fixtures | `testdata/fixtures/` | `{name}.cue`, `{name}.json` |
| RFC 8785 fixtures | `testdata/fixtures/rfc8785/` | `{lang}_{scenario}.json` |

### Structure Patterns

#### Package Organization

```
internal/
├── ir/          # Canonical IR types ONLY - no logic except (de)serialization
│   ├── types.go       # ConceptSpec, ActionSig, SyncRule, etc.
│   ├── json.go        # RFC 8785 canonical JSON
│   ├── hash.go        # Content-addressed identity with domain separation
│   ├── value.go       # Constrained IRValue types (no floats)
│   └── version.go     # IR schema versioning
│
├── compiler/    # CUE → IR compilation (one direction only)
│   ├── compiler.go    # Main entry point: compiler.Compile()
│   ├── concept.go     # ConceptSpec parsing
│   ├── sync.go        # SyncRule parsing
│   ├── validate.go    # Schema validation
│   └── redact.go      # @redact annotation extraction
│
├── engine/      # Sync execution (stateful, single-writer)
│   ├── engine.go      # Main loop: engine.Run()
│   ├── scheduler.go   # FIFO queue, deterministic ordering
│   ├── matcher.go     # When-clause + output case matching
│   ├── cycle.go       # Cycle detection per flow
│   ├── quota.go       # Max-steps enforcement
│   └── flow.go        # FlowTokenGenerator interface
│
├── store/       # SQLite persistence (all DB access here)
│   ├── store.go       # Connection, migrations: store.Open()
│   ├── schema.sql     # Embedded SQL schema
│   ├── write.go       # Insert invocations, completions, firings
│   ├── read.go        # Query by flow, provenance
│   └── projections.go # State table management
│
├── queryir/     # Query IR types (abstraction layer)
│   ├── types.go       # Select, Join, Filter, etc.
│   ├── validate.go    # Portable fragment validation
│   └── ordering.go    # Stable ORDER BY requirements
│
├── querysql/    # Query IR → SQL (one backend)
│   └── compile.go     # Always parameterized, always ordered
│
├── harness/     # Conformance testing
│   ├── harness.go     # Test runner: harness.Run()
│   ├── scenario.go    # Scenario loading
│   ├── assertions.go  # Trace assertions
│   └── golden.go      # Snapshot comparison
│
└── testutil/    # Shared test helpers
    ├── clock.go       # Deterministic clock
    ├── flow.go        # Fixed flow token generator
    └── db.go          # Test database setup
```

**Package Rules:**
1. `ir/` has NO dependencies on other internal packages
2. `store/` is the ONLY package that imports `database/sql`
3. `engine/` depends on `ir/`, `store/`, `queryir/`, `querysql/`
4. `compiler/` depends only on `ir/` and CUE SDK
5. No circular dependencies - use interfaces at boundaries

#### Test Organization

| Test Type | Location | Command |
|-----------|----------|---------|
| Unit tests | `{pkg}/*_test.go` | `go test ./internal/{pkg}` |
| Golden tests | `testdata/{pkg}/*.golden` | Auto-discovered |
| Integration | `internal/{pkg}/integration_test.go` | `go test -tags=integration` |
| Fuzz tests | `{pkg}/fuzz_test.go` | `go test -fuzz=FuzzX` |
| E2E | `testdata/e2e/*.txtar` | `testscript` |

### Format Patterns

#### CLI Output Structure

```go
// JSON output (--format=json)
type CLIResponse struct {
    Status  string `json:"status"`   // "ok" or "error"
    Data    any    `json:"data,omitempty"`
    Error   *CLIError `json:"error,omitempty"`
    TraceID string `json:"trace_id,omitempty"`
}

type CLIError struct {
    Code    string `json:"code"`    // "E001", "E002", etc.
    Message string `json:"message"`
    Details any    `json:"details,omitempty"`
}

// Human output (default)
// ✓ Compiled 3 concepts, 2 syncs
//   Cart: 3 actions, 1 operational principle
```

#### Error Codes

| Code | Category | Example |
|------|----------|---------|
| E001-E099 | Compilation errors | E001: Invalid CUE syntax |
| E100-E199 | Validation errors | E101: Missing required field |
| E200-E299 | Engine errors | E201: Cycle detected |
| E300-E399 | Store errors | E301: Database locked |

#### Log Message Structure

```go
// Standard correlation keys (ALWAYS include when available)
slog.Info("sync fired",
    "flow_token", flowToken,      // REQUIRED when in flow context
    "invocation_id", invocationID,
    "completion_id", completionID,
    "sync_id", syncID,
    "binding_hash", bindingHash,
    "seq", seq,                   // Logical sequence number
    "event", "fire",              // Event type
)

// Event types: "invoke", "complete", "fire", "skip", "cycle", "quota"
```

### Communication Patterns

#### Error Wrapping

```go
// CORRECT: Include context, use %w
if err != nil {
    return fmt.Errorf("compile concept %q: %w", name, err)
}

// CORRECT: Check with errors.Is/As
if errors.Is(err, ErrCyclicSync) {
    // Handle cycle
}

// WRONG: Lose error chain
if err != nil {
    return errors.New("compilation failed") // ❌ Lost original error
}
```

#### Interface Boundaries

```go
// CORRECT: Small interfaces at package boundaries
type QueryCompiler interface {
    Compile(queryir.Query) (sql string, args []any, err error)
}

type Store interface {
    WriteInvocation(ctx context.Context, inv ir.Invocation) error
    WriteCompletion(ctx context.Context, comp ir.Completion) error
    WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) error
}

// Engine accepts interfaces, not concrete types
func NewEngine(store Store, compiler QueryCompiler) *Engine
```

### Process Patterns

#### Determinism Checklist

Every code path in the sync engine MUST satisfy:

- [ ] No `time.Now()` - use injected clock
- [ ] No `rand.*` - use injected random source
- [ ] No `map` iteration without `slices.Sorted(maps.Keys())`
- [ ] No goroutines in evaluation path
- [ ] All queries have `ORDER BY`
- [ ] All IDs are content-addressed

#### Map Iteration

```go
// CORRECT: Sorted iteration
keys := slices.Sorted(maps.Keys(bindings))
for _, k := range keys {
    v := bindings[k]
    // Process in deterministic order
}

// WRONG: Non-deterministic order
for k, v := range bindings { // ❌ Order varies between runs
    // ...
}
```

#### Table-Driven Tests

```go
// REQUIRED for all public functions
func TestCompile(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    *ir.ConceptSpec
        wantErr string
    }{
        {
            name:  "valid concept",
            input: `concept Cart { ... }`,
            want:  &ir.ConceptSpec{...},
        },
        {
            name:    "missing purpose",
            input:   `concept Cart {}`,
            wantErr: "missing required field: purpose",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := compiler.Compile(tt.input)
            if tt.wantErr != "" {
                require.ErrorContains(t, err, tt.wantErr)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Enforcement Guidelines

#### CI Checks (All Required)

```yaml
# .github/workflows/ci.yml
- run: go vet ./...
- run: golangci-lint run
- run: go test ./...
- run: go test -race ./...
- run: go test -fuzz=FuzzCanonicalJSON -fuzztime=30s ./internal/ir
```

#### golangci-lint Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck      # Check error returns
    - govet         # Vet examines Go source code
    - staticcheck   # Static analysis
    - unused        # Check for unused code
    - gosimple      # Simplify code
    - ineffassign   # Detect ineffectual assignments
    - misspell      # Check spelling

linters-settings:
  govet:
    check-shadowing: true
```

#### Pre-Commit Hooks

```bash
#!/bin/sh
# .git/hooks/pre-commit
go vet ./...
golangci-lint run --fast
go test -short ./...
```

### Anti-Patterns Reference

| Category | ❌ Wrong | ✅ Correct |
|----------|----------|-----------|
| Time | `time.Now()` | `clock.Now()` (injected) |
| Random | `rand.Int()` | `rng.Int()` (injected) |
| Maps | `for k := range m` | `for _, k := range slices.Sorted(maps.Keys(m))` |
| Identity | Auto-increment ID | Content-addressed hash |
| Timestamps | `CURRENT_TIMESTAMP` | `seq INTEGER` (logical clock) |
| JSON keys | Default Go ordering | RFC 8785 UTF-16 ordering |
| Queries | Implicit ordering | Explicit `ORDER BY` |
| Errors | `errors.New("failed")` | `fmt.Errorf("context: %w", err)` |
| Values | `map[string]any` | Constrained `IRValue` types |
| Floats | `float64` in IR | `int64` or fixed-point |

---

## Project Structure & Boundaries

### Complete Project Directory Structure

```
nysm/
├── README.md                           # Project overview, quick start
├── LICENSE                             # MIT or similar
├── ARCHITECTURE.md                     # Link to docs/architecture.md
├── go.mod                              # Go module definition
├── go.sum                              # Dependency checksums
├── .golangci.yml                       # Linter configuration
├── .gitignore                          # Git ignore patterns
│
├── .github/
│   └── workflows/
│       ├── ci.yml                      # Build, test, lint, fuzz
│       └── release.yml                 # Tagged release builds
│
├── cmd/
│   └── nysm/
│       └── main.go                     # CLI entry point (cobra root)
│
├── internal/
│   ├── ir/                             # Canonical IR types (NO external deps)
│   │   ├── types.go                    # ConceptSpec, ActionSig, SyncRule, etc.
│   │   ├── json.go                     # RFC 8785 canonical JSON marshaling
│   │   ├── json_test.go                # Canonicalization fuzz tests
│   │   ├── hash.go                     # Content-addressed identity (SHA-256)
│   │   ├── hash_test.go                # Domain separation tests
│   │   ├── value.go                    # IRValue sealed interface (no floats)
│   │   ├── value_test.go               # Type constraint tests
│   │   └── version.go                  # IR schema version constants
│   │
│   ├── compiler/                       # CUE → IR compilation
│   │   ├── compiler.go                 # compiler.Compile() entry point
│   │   ├── compiler_test.go            # Table-driven compilation tests
│   │   ├── concept.go                  # ConceptSpec parsing from CUE
│   │   ├── concept_test.go
│   │   ├── sync.go                     # SyncRule parsing from CUE
│   │   ├── sync_test.go
│   │   ├── validate.go                 # Schema validation against CUE constraints
│   │   ├── validate_test.go
│   │   ├── redact.go                   # @redact annotation extraction
│   │   └── redact_test.go
│   │
│   ├── engine/                         # Sync execution engine
│   │   ├── engine.go                   # engine.New(), engine.Run()
│   │   ├── engine_test.go              # Determinism invariant tests
│   │   ├── scheduler.go                # FIFO queue, declaration-order eval
│   │   ├── scheduler_test.go
│   │   ├── matcher.go                  # When-clause + output case matching
│   │   ├── matcher_test.go
│   │   ├── binder.go                   # Where-clause binding execution
│   │   ├── binder_test.go
│   │   ├── invoker.go                  # Then-clause invocation generation
│   │   ├── invoker_test.go
│   │   ├── cycle.go                    # Cycle detection per flow
│   │   ├── cycle_test.go
│   │   ├── quota.go                    # Max-steps enforcement
│   │   ├── quota_test.go
│   │   ├── flow.go                     # FlowTokenGenerator interface
│   │   └── flow_test.go
│   │
│   ├── store/                          # SQLite persistence layer
│   │   ├── store.go                    # store.Open(), migrations
│   │   ├── store_test.go
│   │   ├── schema.sql                  # Embedded DDL (go:embed)
│   │   ├── write.go                    # Insert invocations, completions, firings
│   │   ├── write_test.go
│   │   ├── read.go                     # Query by flow, provenance traversal
│   │   ├── read_test.go
│   │   ├── projections.go              # State table generation + updates
│   │   └── projections_test.go
│   │
│   ├── queryir/                        # Query IR abstraction layer
│   │   ├── types.go                    # Select, Join, Filter, Predicate
│   │   ├── types_test.go
│   │   ├── validate.go                 # Portable fragment validation
│   │   ├── validate_test.go
│   │   ├── ordering.go                 # Stable ORDER BY requirements
│   │   └── ordering_test.go
│   │
│   ├── querysql/                       # Query IR → SQL backend
│   │   ├── compile.go                  # IR to parameterized SQL + args
│   │   └── compile_test.go             # Golden SQL output tests
│   │
│   ├── harness/                        # Conformance test runner
│   │   ├── harness.go                  # harness.Run() entry point
│   │   ├── harness_test.go
│   │   ├── scenario.go                 # Scenario loading from fixtures
│   │   ├── scenario_test.go
│   │   ├── assertions.go               # Trace assertions
│   │   ├── assertions_test.go
│   │   ├── golden.go                   # Snapshot comparison
│   │   └── golden_test.go
│   │
│   ├── cli/                            # CLI command implementations
│   │   ├── compile.go                  # nysm compile
│   │   ├── validate.go                 # nysm validate
│   │   ├── run.go                      # nysm run
│   │   ├── replay.go                   # nysm replay
│   │   ├── test.go                     # nysm test
│   │   ├── trace.go                    # nysm trace
│   │   └── output.go                   # JSON/human output formatting
│   │
│   └── testutil/                       # Shared test helpers
│       ├── clock.go                    # Deterministic clock implementation
│       ├── flow.go                     # Fixed flow token generator
│       ├── db.go                       # Test database setup/teardown
│       └── assert.go                   # Custom test assertions
│
├── specs/                              # Demo concept specs (CUE)
│   ├── cart.concept.cue                # Cart concept: addItem, removeItem, checkout
│   ├── inventory.concept.cue           # Inventory concept: reserve, release
│   ├── web.concept.cue                 # Web concept: request, respond
│   └── cart-inventory.sync.cue         # Sync rules: checkout → reserve
│
├── testdata/
│   ├── fixtures/
│   │   ├── concepts/                   # Valid/invalid concept CUE files
│   │   │   ├── valid_cart.cue
│   │   │   ├── invalid_missing_purpose.cue
│   │   │   └── ...
│   │   ├── syncs/                      # Valid/invalid sync CUE files
│   │   │   ├── valid_cart_inventory.cue
│   │   │   └── ...
│   │   ├── rfc8785/                    # Cross-language JSON canonicalization
│   │   │   ├── go_basic.json
│   │   │   ├── python_basic.json
│   │   │   ├── utf16_ordering.json
│   │   │   └── ...
│   │   └── ir/                         # IR validation fixtures
│   │       ├── valid_concept_spec.json
│   │       └── ...
│   │
│   ├── golden/
│   │   ├── compiler/                   # Expected compilation outputs
│   │   │   ├── cart_concept.golden
│   │   │   └── ...
│   │   ├── engine/                     # Expected engine traces
│   │   │   ├── checkout_flow.golden
│   │   │   ├── insufficient_stock.golden
│   │   │   └── ...
│   │   └── querysql/                   # Expected SQL generation
│   │       ├── simple_select.golden
│   │       └── ...
│   │
│   ├── scenarios/                      # Conformance test scenarios
│   │   ├── cart_checkout_success.yaml
│   │   ├── cart_checkout_insufficient_stock.yaml
│   │   ├── cycle_detection.yaml
│   │   └── quota_exceeded.yaml
│   │
│   └── e2e/                            # End-to-end test scripts
│       ├── compile_and_run.txtar
│       ├── replay_determinism.txtar
│       └── golden_trace.txtar
│
└── docs/
    ├── architecture.md                 # This document
    ├── prd.md                          # Product requirements
    └── analysis/
        ├── product-brief-nysm-2025-12-12.md
        └── brainstorming-session-2025-12-03.md
```

### Architectural Boundaries

#### Package Dependencies (Directed Acyclic Graph)

```
                    ┌─────────────┐
                    │   cmd/nysm  │
                    └──────┬──────┘
                           │
                    ┌──────▼──────┐
                    │  internal/  │
                    │    cli/     │
                    └──────┬──────┘
                           │
           ┌───────────────┼───────────────┐
           │               │               │
    ┌──────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │  compiler/  │ │   engine/   │ │  harness/   │
    └──────┬──────┘ └──────┬──────┘ └──────┬──────┘
           │               │               │
           │        ┌──────┴──────┐        │
           │        │             │        │
           │    ┌───▼───┐   ┌─────▼─────┐  │
           │    │ store/│   │ queryir/  │  │
           │    └───────┘   │ querysql/ │  │
           │                └───────────┘  │
           │                               │
           └───────────────┬───────────────┘
                           │
                    ┌──────▼──────┐
                    │     ir/     │  ← NO INTERNAL DEPS
                    └─────────────┘
```

**Dependency Rules:**
1. `ir/` imports only stdlib - it's the leaf dependency
2. `store/` is the only package importing `database/sql`
3. `compiler/` imports `ir/` and `cuelang.org/go/*` only
4. `engine/` imports `ir/`, `store/`, `queryir/`, `querysql/`
5. `cli/` imports all internal packages
6. No import cycles allowed

#### API Boundaries

| Boundary | Interface | Location |
|----------|-----------|----------|
| Storage | `store.Store` | `internal/store/store.go` |
| Query Compilation | `queryir.Compiler` | `internal/queryir/types.go` |
| Flow Generation | `engine.FlowTokenGenerator` | `internal/engine/flow.go` |
| Clock | `engine.Clock` | `internal/engine/engine.go` |

```go
// internal/store/store.go
type Store interface {
    WriteInvocation(ctx context.Context, inv ir.Invocation) error
    WriteCompletion(ctx context.Context, comp ir.Completion) error
    WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) error
    ReadFlow(ctx context.Context, flowToken string) ([]ir.Event, error)
    ReadProvenance(ctx context.Context, completionID string) ([]ir.ProvenanceEdge, error)
}

// internal/queryir/types.go
type Compiler interface {
    Compile(q Query) (sql string, args []any, err error)
}
```

#### Data Boundaries

| Layer | Responsibility | Storage |
|-------|----------------|---------|
| Event Log | Invocations, completions, sync firings | `invocations`, `completions`, `sync_firings` tables |
| Provenance | Audit trail edges | `provenance_edges` table |
| State Projections | Materialized views | Per-concept state tables |

**Data Flow:**
```
[CUE Specs] → compiler.Compile() → [ir.ConceptSpec, ir.SyncRule]
                                            │
                                            ▼
[External Request] → engine.Invoke() → [ir.Invocation]
                                            │
                                            ▼
                                    store.WriteInvocation()
                                            │
                                            ▼
                              [Action Execution] → [ir.Completion]
                                            │
                                            ▼
                                    store.WriteCompletion()
                                            │
                                            ▼
                              engine.EvaluateSyncs() → [ir.SyncFiring, ir.Invocation...]
                                            │
                                            ▼
                              store.WriteSyncFiring() + store.WriteInvocation()
```

### Requirements to Structure Mapping

#### FR Category Mapping

| FR Category | Primary Package | Key Files |
|-------------|-----------------|-----------|
| **FR-1: Concept Specification** | `internal/compiler/` | `concept.go`, `validate.go` |
| **FR-2: Synchronization System** | `internal/engine/` | `matcher.go`, `binder.go`, `invoker.go` |
| **FR-3: Flow Token System** | `internal/engine/` | `flow.go`, `engine.go` |
| **FR-4: Provenance System** | `internal/store/` | `write.go`, `read.go` |
| **FR-5: Durable Engine** | `internal/store/` | `store.go`, `schema.sql` |
| **FR-6: Conformance Harness** | `internal/harness/` | `harness.go`, `assertions.go`, `golden.go` |

#### Cross-Cutting Concerns Mapping

| Concern | Files |
|---------|-------|
| **Canonical JSON (RFC 8785)** | `internal/ir/json.go` |
| **Content-Addressed Identity** | `internal/ir/hash.go` |
| **Deterministic Ordering** | `internal/queryir/ordering.go`, `internal/querysql/compile.go` |
| **Flow Token Propagation** | `internal/engine/flow.go`, all store write methods |
| **Security Context** | `internal/ir/types.go` (SecurityContext), all store write methods |
| **Error Type Propagation** | `internal/ir/types.go` (OutputCase), `internal/engine/matcher.go` |

### Integration Points

#### Internal Communication

```go
// Engine uses interfaces for testability
type Engine struct {
    store       Store              // SQLite storage
    compiler    queryir.Compiler   // Query IR → SQL
    flowGen     FlowTokenGenerator // UUIDv7 or fixed for tests
    clock       Clock              // wall clock or deterministic
    specs       []ir.ConceptSpec
    syncs       []ir.SyncRule
}

// All internal communication via method calls, no events/channels
func (e *Engine) processCompletion(ctx context.Context, comp ir.Completion) error {
    // 1. Write completion to store
    // 2. Match against sync rules
    // 3. For each match, execute where-clause
    // 4. For each binding, generate invocation
    // 5. Write firings and invocations
}
```

#### External Integrations

| Integration | Location | Notes |
|-------------|----------|-------|
| CUE SDK | `internal/compiler/` | `cuelang.org/go/cue/*` |
| SQLite | `internal/store/` | `github.com/mattn/go-sqlite3` |
| CLI | `internal/cli/` + `cmd/nysm/` | `github.com/spf13/cobra` |

### File Organization Patterns

#### Configuration Files

| File | Purpose |
|------|---------|
| `go.mod` | Module definition, Go version, dependencies |
| `go.sum` | Dependency checksums (committed) |
| `.golangci.yml` | Linter configuration |
| `.gitignore` | Git ignore patterns |
| `.github/workflows/*.yml` | CI/CD pipelines |

#### Embedded Files

```go
// internal/store/store.go
import _ "embed"

//go:embed schema.sql
var schemaSQL string

func (s *Store) migrate() error {
    _, err := s.db.Exec(schemaSQL)
    return err
}
```

#### Test Data Organization

| Directory | Content |
|-----------|---------|
| `testdata/fixtures/` | Input files for parsing/validation tests |
| `testdata/golden/` | Expected output snapshots (updated with `-update` flag) |
| `testdata/scenarios/` | Conformance test scenario definitions |
| `testdata/e2e/` | End-to-end test scripts (txtar format) |

### Development Workflow Integration

#### Build Commands

```bash
# Development
go build -o nysm ./cmd/nysm

# Production (with CGO for SQLite)
CGO_ENABLED=1 go build -ldflags="-s -w" -o nysm ./cmd/nysm

# Cross-compile (pure Go SQLite)
CGO_ENABLED=0 go build -tags sqlite_omit_load_extension -o nysm ./cmd/nysm
```

#### Test Commands

```bash
go test ./...                                    # All unit tests
go test -race ./...                              # Race detection
go test -fuzz=FuzzCanonicalJSON ./internal/ir   # Fuzz testing
go test -tags=integration ./...                  # Integration tests
go test -run=TestE2E ./internal/harness         # E2E via harness
```

#### CI Pipeline Stages

```yaml
# .github/workflows/ci.yml
jobs:
  test:
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      - run: go vet ./...
      - run: golangci-lint run
      - run: go test ./...
      - run: go test -race ./...
      - run: go test -fuzz=FuzzCanonicalJSON -fuzztime=30s ./internal/ir
```

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**

| Decision Area | Status | Notes |
|---------------|--------|-------|
| Go 1.25 + CUE SDK v0.15.1 | ✅ Compatible | CUE SDK is native Go |
| SQLite + mattn/go-sqlite3 | ✅ Compatible | Standard Go SQL interface |
| RFC 8785 + SHA-256 | ✅ Compatible | Both require deterministic JSON |
| UUIDv7 + google/uuid | ✅ Compatible | Stdlib-compatible library |
| Cobra CLI + slog | ✅ Compatible | Both stdlib/standard patterns |
| Single-writer + SQLite WAL | ✅ Compatible | WAL enables concurrent reads |

**Pattern Consistency:**
- Naming conventions (snake_case SQL, PascalCase Go, kebab-case CLI) are consistent across all components
- URI scheme unified to `nysm://{namespace}/{type}/{name}@{semver}` - no conflicting formats
- Error handling pattern (fmt.Errorf + %w) consistent throughout all packages
- Determinism patterns (sorted maps, injectable clock/random) consistently enforced in Critical Patterns

**Structure Alignment:**
- Package DAG prevents cycles: `ir` → `compiler`/`store`/`queryir` → `engine` → `cli`
- Interface boundaries (Store, QueryCompiler, FlowTokenGenerator, Clock) enable testing and substitution
- Test organization (co-located `*_test.go` + `testdata/`) supports golden testing workflow

### Requirements Coverage Validation ✅

**Functional Requirements Coverage:**

| FR | Architectural Support | Key Files |
|----|----------------------|-----------|
| FR-1: Concept Specification | CUE→IR compiler with validation | `internal/compiler/concept.go`, `validate.go` |
| FR-2: Synchronization System | Engine with when/where/then clauses | `internal/engine/matcher.go`, `binder.go`, `invoker.go` |
| FR-3: Flow Token System | UUIDv7 generator + propagation on all records | `internal/engine/flow.go`, all store writes |
| FR-4: Provenance System | Sync firings + provenance edges tables | `internal/store/write.go`, `read.go` |
| FR-5: Durable Engine | SQLite + WAL + logical clocks + crash recovery | `internal/store/store.go`, `schema.sql` |
| FR-6: Conformance Harness | Test runner + assertions + golden comparison | `internal/harness/*.go` |

**Non-Functional Requirements Coverage:**

| NFR | How Addressed |
|-----|---------------|
| NFR-1.1: Self-documenting specs | CUE surface format with purpose, state, actions, operational principles |
| NFR-1.2: No implicit behavior | Sync rules explicit in DSL; scoping modes declared |
| NFR-1.3: Queryable provenance | Provenance edges table with completion→invocation links |
| NFR-2.1: Deterministic replay | Logical clocks (seq), content-addressed IDs, sorted iteration, RFC 8785 |
| NFR-2.2: Idempotent sync firing | Binding-level uniqueness: UNIQUE(completion_id, sync_id, binding_hash) |
| NFR-2.3: Flow isolation | Flow token on all records; queries scoped by flow_token |
| NFR-3.1: Versioned IR | IR version constants + spec_hash + engine_version on records |
| NFR-3.2: Query abstraction | QueryIR layer compiles to SQL; clean interface for future SPARQL |
| NFR-3.3: Surface format abstraction | CUE compiles to canonical IR; same IR target for future custom DSL |
| NFR-4.1: Actionable errors | Error codes (E001-E399) with categories; ValidationErrors collects all |
| NFR-4.2: Traceable evaluation | slog with correlation keys (flow_token, sync_id, etc.) |
| NFR-4.3: Trace diff output | Golden file comparison with goldie; `-update` flag for regeneration |

### Implementation Readiness Validation ✅

**Decision Completeness:**
- All technology choices have pinned versions (CUE v0.15.1, SQLite v1.14.32, Cobra v1.10.2, etc.)
- RFC 8785 implementation details specified including UTF-16 code unit ordering
- Content-addressed ID format with domain separation fully documented
- Security context schema defined with redaction policy (enforcement deferred to post-MVP)

**Structure Completeness:**
- Complete file tree with 80+ files defined across 10 packages
- Package dependency DAG explicitly documented with rules
- Interface definitions at all major boundaries
- Test organization (unit, golden, fuzz, integration, e2e) fully specified

**Pattern Completeness:**
- 6 Critical Patterns (CP-1 through CP-6) with code examples and enforcement rules
- Anti-patterns reference table with ❌/✅ comparisons
- Determinism checklist for code review
- CI configuration and golangci-lint setup provided

### Gap Analysis Results

**Critical Gaps:** None identified

**Important Gaps:** None identified

**Nice-to-Have Gaps (Deferred per PRD):**

| Gap | Priority | Rationale |
|-----|----------|-----------|
| CUE schema reference docs | Low | Demo specs in `specs/` serve as examples |
| Custom DSL syntax | Deferred | CUE sufficient for MVP; add when semantics stabilize |
| SPARQL backend | Deferred | SQL sufficient for MVP; QueryIR abstraction ready |
| Migration tooling | Deferred | Out of MVP scope |
| Observability UI | Deferred | Phase 7 per PRD |

### Validation Issues Addressed

Three rounds of Codex review identified and resolved:

1. **CRITICAL-1**: Binding-level idempotency → Added `binding_hash` to sync_firings
2. **CRITICAL-2**: Deterministic replay → Logical clocks, content-addressed IDs, no wall time
3. **CRITICAL-3**: Termination semantics → Cycle detection, stratification, max-steps quota
4. **HIGH-1**: Flow token scoping → Added scoping modes (flow/global/keyed)
5. **HIGH-2**: Query abstraction → QueryIR layer between DSL and SQL
6. **HIGH-3**: Security model → Security context on all records, parameterized queries only
7. **MEDIUM-1/2/3**: State model (event-sourced projections), error matching in when-clause, unified URI scheme

All issues incorporated into architecture document.

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] Project context thoroughly analyzed (compiler + runtime engine, not web app)
- [x] Scale and complexity assessed (6 major subsystems, medium complexity)
- [x] Technical constraints identified (no transactions, no getters, named args, typed outputs)
- [x] Cross-cutting concerns mapped (canonical JSON, flow tokens, provenance, security context)

**✅ Critical Design Issues (from Codex Reviews)**
- [x] CRITICAL-1: Binding-level idempotency
- [x] CRITICAL-2: Logical identity + time
- [x] CRITICAL-3: Sync engine termination semantics
- [x] HIGH-1: Flow token scoping modes
- [x] HIGH-2: Query IR abstraction boundary
- [x] HIGH-3: Security model foundation
- [x] MEDIUM-1: State model clarification (event-sourced projections)
- [x] MEDIUM-2: Error matching in when-clause
- [x] MEDIUM-3: Naming and versioning scheme

**✅ Architectural Decisions**
- [x] Technology stack fully specified with pinned versions
- [x] Go + CUE + SQLite + Cobra stack documented with rationale
- [x] RFC 8785 canonical JSON with UTF-16 key ordering
- [x] SHA-256 content-addressed IDs with domain separation
- [x] UUIDv7 flow tokens with injectable generation
- [x] Single-writer event loop for deterministic scheduling
- [x] Security context + provenance redaction

**✅ Implementation Patterns**
- [x] 6 Critical Patterns (CP-1 through CP-6) mandatory for correctness
- [x] Naming conventions (SQL, Go, IR, URI, CLI, tests)
- [x] Structure patterns (package organization, dependency rules)
- [x] Communication patterns (error wrapping, interface boundaries)
- [x] Process patterns (determinism checklist, map iteration, table-driven tests)
- [x] Enforcement guidelines (CI, golangci-lint, pre-commit hooks)
- [x] Anti-patterns reference table

**✅ Project Structure**
- [x] Complete 80+ file directory tree
- [x] Package dependency DAG with 6 rules
- [x] Interface boundaries at Store/QueryCompiler/FlowTokenGenerator/Clock
- [x] FR category to package mapping
- [x] Cross-cutting concerns to file mapping
- [x] Build commands (dev, production, cross-compile)
- [x] Test commands (unit, race, fuzz, integration, e2e)
- [x] CI pipeline configuration

### Architecture Readiness Assessment

**Overall Status:** ✅ READY FOR IMPLEMENTATION

**Confidence Level:** HIGH

**Key Strengths:**
1. Three rounds of external Codex review incorporated critical correctness fixes
2. Determinism-first design eliminates replay failures and enables golden testing
3. Clear package boundaries with interface contracts enable parallel development
4. Comprehensive Critical Patterns prevent AI agent implementation conflicts
5. Full requirements traceability from FRs/NFRs to specific code locations
6. Unified naming and URI schemes prevent cross-component inconsistencies

**Areas for Future Enhancement:**
- Custom DSL syntax (CUE sufficient for MVP per PRD decision)
- SPARQL backend (SQL sufficient for MVP; QueryIR abstraction ready)
- Web/HTTP as ordinary concept (Phase 6 per PRD)
- Observability tooling and "why" queries (Phase 7 per PRD)
- SDK/codegen ergonomics (post-MVP per PRD)

### Implementation Handoff

**AI Agent Guidelines:**

1. Follow all architectural decisions exactly as documented in this file
2. Use implementation patterns consistently across all components
3. Respect package dependency DAG - no cycles allowed
4. Adhere to all 6 Critical Patterns (CP-1 through CP-6) - these are correctness requirements
5. Use the anti-patterns reference table during code review
6. Run determinism checklist before committing engine code
7. Refer to this document for all architectural questions

**First Implementation Priority:**

Begin with the IR package (`internal/ir/`) as it has no internal dependencies:
1. `types.go` - Core IR types (ConceptSpec, ActionSig, SyncRule, Invocation, Completion, etc.)
2. `value.go` - Constrained IRValue types (no floats)
3. `json.go` - RFC 8785 canonical JSON marshaling with UTF-16 key ordering
4. `hash.go` - Content-addressed identity with domain separation
5. `version.go` - IR schema version constants

Then proceed to `internal/store/schema.sql` to establish the database schema, followed by the compiler and engine packages.

---

## Architecture Completion Summary

### Workflow Completion

**Architecture Decision Workflow:** COMPLETED ✅
**Total Steps Completed:** 8
**Date Completed:** 2025-12-12
**Document Location:** docs/architecture.md

### Final Architecture Deliverables

**📋 Complete Architecture Document**

- All architectural decisions documented with specific versions
- Implementation patterns ensuring AI agent consistency
- Complete project structure with all files and directories
- Requirements to architecture mapping
- Validation confirming coherence and completeness

**🏗️ Implementation Ready Foundation**

- 9 critical architectural decisions made (CRITICAL-1 through CRITICAL-3, HIGH-1 through HIGH-3, MEDIUM-1 through MEDIUM-3)
- 6 critical implementation patterns defined (CP-1 through CP-6)
- 10 internal packages specified with clear boundaries
- All 6 functional requirements and 4 non-functional requirement categories fully supported

**📚 AI Agent Implementation Guide**

- Technology stack: Go 1.25 + CUE SDK v0.15.1 + SQLite v1.14.32 + Cobra v1.10.2
- Consistency rules that prevent implementation conflicts
- 80+ file project structure with clear boundaries
- Package dependency DAG with 6 rules preventing cycles

### Implementation Handoff

**For AI Agents:**
This architecture document is your complete guide for implementing NYSM. Follow all decisions, patterns, and structures exactly as documented.

**First Implementation Priority:**
```bash
# Create project structure
mkdir -p nysm/cmd/nysm nysm/internal/{ir,compiler,engine,store,queryir,querysql,harness,cli,testutil}
mkdir -p nysm/specs nysm/testdata/{fixtures,golden,scenarios,e2e}
cd nysm && go mod init github.com/tyler/nysm
```

**Development Sequence:**

1. Initialize project using documented structure
2. Implement `internal/ir/` package first (no dependencies)
3. Implement `internal/store/schema.sql` to establish database schema
4. Implement `internal/compiler/` to parse CUE specs
5. Implement `internal/engine/` sync execution core
6. Implement `internal/harness/` for conformance testing
7. Build CLI commands in `internal/cli/` + `cmd/nysm/`
8. Maintain consistency with all documented patterns

### Quality Assurance Checklist

**✅ Architecture Coherence**

- [x] All decisions work together without conflicts
- [x] Technology choices are compatible
- [x] Patterns support the architectural decisions
- [x] Structure aligns with all choices

**✅ Requirements Coverage**

- [x] All functional requirements are supported
- [x] All non-functional requirements are addressed
- [x] Cross-cutting concerns are handled
- [x] Integration points are defined

**✅ Implementation Readiness**

- [x] Decisions are specific and actionable
- [x] Patterns prevent agent conflicts
- [x] Structure is complete and unambiguous
- [x] Examples are provided for clarity

### Project Success Factors

**🎯 Clear Decision Framework**
Every technology choice was made collaboratively with clear rationale, including three rounds of external Codex review to identify critical correctness issues.

**🔧 Consistency Guarantee**
Implementation patterns and rules ensure that multiple AI agents will produce compatible, consistent code that works together seamlessly.

**📋 Complete Coverage**
All project requirements are architecturally supported, with clear mapping from business needs to technical implementation.

**🏗️ Solid Foundation**
The architecture addresses the unique challenges of deterministic event-sourced systems with careful attention to idempotency, replay semantics, and query ordering.

---

**Architecture Status:** READY FOR IMPLEMENTATION ✅

**Next Phase:** Begin implementation using the architectural decisions and patterns documented herein.

**Document Maintenance:** Update this architecture when major technical decisions are made during implementation.
