---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - docs/analysis/product-brief-nysm-2025-12-12.md
  - docs/analysis/brainstorming-session-2025-12-03.md
  - initial-report.md
workflowType: 'prd'
lastStep: 6
project_name: 'NYSM'
user_name: 'Tyler'
date: '2025-12-12'
---

# Product Requirements Document - NYSM

**Author:** Tyler
**Date:** 2025-12-12
**Version:** 1.0

---

## 1. Executive Summary

NYSM (Now You See Me) is a software development framework implementing the "What You See Is What It Does" (WYSIWYG) pattern from arxiv.org/abs/2508.14511. The framework makes software inherently legible—both for humans and AI—by ensuring code structure directly corresponds to system behavior.

### Purpose

Enable developers to build and maintain software more efficiently with AI assistance, producing applications that AI can realistically maintain either fully or partially on its own.

### Scope

NYSM encompasses the framework core, reference implementation (canonical cart/inventory demo), and tooling needed to write software in this paradigm.

---

## 2. Problem Statement

### Core Problem

Software is fundamentally hard to read and maintain—for both humans and AI. This problem has existed since humans first started coding and has only worsened as the number of languages, frameworks, and surfaces for software has exponentially increased.

### Problem Manifestations

| Problem Area | Impact |
|--------------|--------|
| Documentation rot | Docs drift from code reality, becoming misleading |
| Test opacity | Tests verify behavior but don't explain intent or architecture |
| Log noise | Logs capture events but not causality |
| Code-behavior gap | What code looks like vs. what it does creates friction |
| Technical debt | Systems become opaque, changes become risky |

### Who Experiences This

1. **Solo Developers** — Lose context when returning to code after time away; complexity fights creativity
2. **AI Agents** — Struggle with implicit decisions, hidden dependencies, cascading failures from local changes

### Why Existing Solutions Fall Short

Clean code principles, documentation, testing, and observability tools all attempt to bridge the gap between code and behavior—but they're **bolted on, not structural**. None make the code itself legible.

---

## 3. Target Users

### Primary: The Solo Developer

**Profile:** Builders working on side projects and indie apps in the margins of their time.

**Pain Points:**
- Coming back to code after days/weeks and losing context
- Complexity scaling faster than comprehension
- Implementation details blocking architectural thinking

**Success Criteria:**
- Easy to understand how it works, what the rules are, and how to extend it
- Quick to learn, reveals depth with engagement
- Simplicity enables creativity

### Primary: The AI Agent

**Profile:** Coding assistants (Claude Code, etc.) and autonomous agents that write, review, and maintain software.

**Pain Points:**
- Human code filled with implicit decisions that can't be inferred
- Traditional interdependencies make safe changes difficult
- Massive token/compute overhead for accounting side effects

**Success Criteria:**
- Intuitive to navigate and code in
- Structure eliminates cascading dependency failures
- Enables prototyping, fixes, and creativity without traditional risks

### Human-AI Partnership Model

NYSM treats humans and AI as equal partners:
- Human writes code, AI reviews and fact-checks
- AI writes code, human reviews
- Either can lead—the framework doesn't care

---

## 4. Proposed Solution

### Core Pattern

NYSM implements a structural pattern where legibility is inherent, not added:

#### Concept Specs
Self-contained services with:
- **Purpose** — What the concept does
- **Relational State Schema** — The data model
- **Action Signatures** — Typed inputs, outputs, and error cases
- **Operational Principles** — Archetypal scenarios that become executable contract tests

#### Synchronizations
A 3-clause DSL for event-based coordination:
```
when (action completions) →
where (state queries + bindings) →
then (invocations)
```

#### Flow Tokens
Explicit scoping mechanism preventing accidental joins across concurrent requests.

#### Provenance Edges
Structural audit trail enabling:
- Idempotency
- Crash/restart replay
- Answering "why did this happen?"

### Key Constraints (from paper)

| Constraint | Rationale |
|------------|-----------|
| No transactions | Integrity via rule scheme + flow scoping + error-matching |
| No getters | Reads via query, writes via actions only |
| Named args + typed outputs | Enables deterministic replay and AI comprehension |

### Key Differentiators

- **AI-native legibility** — Code structure IS the documentation
- **Operational Principles as tests** — Documentation that can't rot
- **Deterministic replay** — Every derived action explainable via provenance
- **Structural integrity** — Architecture enforces correctness

---

## 5. Functional Requirements

### FR-1: Concept Specification System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1.1 | Support concept specs in CUE format with purpose, state schema, action signatures, operational principles | Must Have |
| FR-1.2 | Validate concept specs against canonical IR schema | Must Have |
| FR-1.3 | Compile CUE specs to canonical JSON IR | Must Have |
| FR-1.4 | Support typed action outputs with multiple cases (success, error variants) | Must Have |

### FR-2: Synchronization System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-2.1 | Support 3-clause sync rules: when → where → then | Must Have |
| FR-2.2 | Compile when-clause to match on action completions | Must Have |
| FR-2.3 | Compile where-clause to SQL queries over relational state | Must Have |
| FR-2.4 | Execute then-clause to generate invocations from bindings | Must Have |
| FR-2.5 | Maintain abstraction boundary for future SPARQL migration | Should Have |

### FR-3: Flow Token System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-3.1 | Generate unique flow tokens for request scoping | Must Have |
| FR-3.2 | Propagate flow tokens through all invocations/completions | Must Have |
| FR-3.3 | Enforce sync rules only match records with same flow token | Must Have |

### FR-4: Provenance System

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-4.1 | Record provenance edges: (completion) -[sync-id]-> (invocation) | Must Have |
| FR-4.2 | Support idempotency check via sync edges | Must Have |
| FR-4.3 | Enable crash/restart replay with identical results | Must Have |

### FR-5: Durable Engine

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-5.1 | SQLite-backed append-only log for invocations/completions | Must Have |
| FR-5.2 | Store provenance edges with query support | Must Have |
| FR-5.3 | Support crash recovery and replay | Must Have |

### FR-6: Conformance Harness

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-6.1 | Load concept specs and sync rules for test execution | Must Have |
| FR-6.2 | Run scenarios with assertions on action traces | Must Have |
| FR-6.3 | Validate operational principles as executable tests | Must Have |
| FR-6.4 | Generate golden trace snapshots | Should Have |

---

## 6. Non-Functional Requirements

### NFR-1: Legibility

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-1.1 | Concept specs readable without external documentation | 100% self-documenting |
| NFR-1.2 | Sync rules understandable from DSL alone | No implicit behavior |
| NFR-1.3 | Provenance queryable for any action | Full trace reconstruction |

### NFR-2: Correctness

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-2.1 | Crash/restart replay produces identical results | Deterministic |
| NFR-2.2 | Sync rules fire exactly once per matching completion | Idempotent |
| NFR-2.3 | Flow tokens prevent cross-request pollution | Isolated |

### NFR-3: Extensibility

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-3.1 | IR supports versioning for forward compatibility | Versioned schemas |
| NFR-3.2 | Query substrate abstracted for SPARQL migration | Clean boundary |
| NFR-3.3 | Surface format abstracted (CUE now, custom DSL later) | Compile to same IR |

### NFR-4: Developer Experience

| ID | Requirement | Target |
|----|-------------|--------|
| NFR-4.1 | Concept spec validation errors actionable | Clear error messages |
| NFR-4.2 | Sync rule evaluation traceable | Debug output available |
| NFR-4.3 | Test failures show trace diff | Golden comparison |

---

## 7. Technical Architecture Overview

### Core IR Types

```
ConceptSpec
├── purpose: string
├── stateSchema: StateSchema[]
├── actions: ActionSig[]
└── operationalPrinciples: Scenario[]

ActionSig
├── name: string
├── args: NamedArg[]
└── outputs: OutputCase[]  // success + error variants

SyncRule
├── id: string
├── when: WhenClause      // action completion pattern
├── where: WhereClause    // query + bindings
└── then: ThenClause      // invocation template

Invocation
├── flow: FlowToken
├── action: ActionRef
└── args: Record<string, Value>

Completion
├── flow: FlowToken
├── action: ActionRef
├── args: Record<string, Value>
└── result: OutputCase

ProvenanceEdge
├── completion: CompletionRef
├── syncId: string
└── invocation: InvocationRef
```

### Data Flow

```
[CUE Specs] → [Compiler] → [Canonical IR]
                              ↓
[Request] → [Flow Token] → [Invocation]
                              ↓
[Action Impl] ←─────────────┘
      ↓
[Completion] → [Sync Engine] → [Where Query (SQL)]
                                      ↓
                              [Bindings Set]
                                      ↓
                              [Then Invocations]
                                      ↓
                              [Provenance Edge]
```

### Storage Schema (SQLite)

```sql
-- Append-only event log
CREATE TABLE invocations (
  id INTEGER PRIMARY KEY,
  flow_token TEXT NOT NULL,
  action_uri TEXT NOT NULL,
  args JSON NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE completions (
  id INTEGER PRIMARY KEY,
  flow_token TEXT NOT NULL,
  action_uri TEXT NOT NULL,
  args JSON NOT NULL,
  result JSON NOT NULL,
  invocation_id INTEGER REFERENCES invocations(id),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE provenance_edges (
  id INTEGER PRIMARY KEY,
  completion_id INTEGER REFERENCES completions(id),
  sync_id TEXT NOT NULL,
  invocation_id INTEGER REFERENCES invocations(id),
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(completion_id, sync_id)  -- idempotency
);

-- Relational state tables (per concept)
-- Generated from StateSchema
```

---

## 8. MVP Phasing

### Phase 1: Canonical Demo Specs
**Deliverables:**
- `Cart.concept.cue` — Add/remove items, checkout
- `Inventory.concept.cue` — Reserve/release stock
- `Web.concept.cue` — Request/respond actions
- `cart-inventory.sync.cue` — 3-clause coordination
- 1 operational principle per concept

**Exit Criteria:**
- Specs compile to valid IR
- Human-readable without external docs

### Phase 2: Minimal Core IR + Schema
**Deliverables:**
- Canonical IR types: ConceptSpec, ActionSig, StateSchema, SyncRule, Invocation, Completion, FlowToken, ProvenanceEdge
- JSON Schema for IR (versioned)
- CUE → IR compiler

**Exit Criteria:**
- Demo specs compile to IR
- IR validates against schema

### Phase 3: Durable Engine
**Deliverables:**
- SQLite storage for invocations/completions/provenance
- Idempotency via sync edges check
- Crash/restart recovery

**Exit Criteria:**
- Engine survives restart mid-flow
- Replay produces identical results

### Phase 4: SQL-first Where-Clause
**Deliverables:**
- DSL where-clause → SQL compiler
- Binding model: query → set of bindings
- State table generation from StateSchema

**Exit Criteria:**
- cart-inventory sync executes correctly
- Abstraction boundary documented

### Phase 5: Conformance Harness
**Deliverables:**
- Test runner for specs + syncs
- Trace assertions
- Operational principle validation
- Golden snapshots for demo

**Exit Criteria:**
- All demo operational principles pass
- Golden traces committed

---

## 9. Out of Scope (MVP)

| Item | Reason | Future Phase |
|------|--------|--------------|
| Custom DSL syntax | CUE sufficient for v0; add when semantics stabilize | Post-MVP |
| RDF/SPARQL | SQL-first for rapid iteration | Phase 6+ |
| Web as ordinary concept | HTTP adapter complexity | Phase 6 |
| Observability tooling | Flow graphs, "why" queries | Phase 7 |
| SDK/codegen | Core semantics first | Phase 7+ |
| Multiple real-world apps | One demo sufficient | Post-MVP |

---

## 10. Technical Decisions

| Decision | Choice | Rationale | Trade-offs |
|----------|--------|-----------|------------|
| Surface format (v0) | CUE | Typed constraints, validation, instant tooling | Less custom syntax |
| Canonical IR | JSON Schema | Version-controlled, language-agnostic | Verbose |
| Query substrate (v0) | SQL | Rapid iteration, familiar tooling | Not graph-native |
| Storage | SQLite | Embeddable, reliable, zero-config | Not distributed |
| Custom DSL | Deferred | Compile to same IR once semantics stable | CUE quirks in v0 |

---

## 11. Success Criteria

NYSM is a personal framework project. Success is defined by:

| Criterion | Measure |
|-----------|---------|
| It works | Framework implements WYSIWYG paper faithfully |
| It's usable | Tyler can build the canonical demo app |
| It's legible | Both humans and AI can understand/modify NYSM codebases without friction |

---

## 12. Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| CUE complexity | Medium | Medium | Document patterns, consider DSL earlier if blocking |
| SQL limitations vs graph queries | Low | Medium | Abstraction boundary; migrate to SPARQL if needed |
| Scope creep to SDK/DX | High | Medium | Strict phase discipline; core semantics first |
| Paper misinterpretation | Low | High | Re-read paper at phase boundaries; operational principles as validation |

---

## 13. Dependencies

### External
- CUE language and tooling
- SQLite library
- JSON Schema tooling

### Internal
- Product Brief (completed)
- Architecture Doc (next phase)

---

## 14. Glossary

| Term | Definition |
|------|------------|
| Concept | Self-contained service with purpose, state, actions, and operational principles |
| Sync Rule | 3-clause DSL (when → where → then) for event-based coordination |
| Flow Token | Unique identifier scoping all records to a single request |
| Provenance Edge | Record linking a completion to invocations it triggered |
| Operational Principle | Archetypal scenario that becomes an executable contract test |
| IR | Intermediate Representation - canonical JSON format for all NYSM artifacts |
| Where-Clause | Query portion of sync rule that produces bindings |

---

## 15. Document History

| Version | Date | Author | Changes |
|---------|------|--------|---------|
| 1.0 | 2025-12-12 | Tyler | Initial PRD |

---

## Appendix A: Canonical Demo Scenarios

### Cart Checkout with Inventory

**Setup:**
- Inventory has 5 units of item "widget"
- Cart is empty

**Flow:**
1. `Cart/addItem(item: "widget", qty: 3)` → success
2. `Cart/checkout()` triggers sync rule
3. Sync where-clause queries cart items
4. Sync then-clause invokes `Inventory/reserve(item: "widget", qty: 3)`
5. Inventory reserve succeeds
6. Cart checkout completes

**Operational Principle Validation:**
- Trace shows: addItem → checkout → reserve
- Provenance edge: checkout-completion -[cart-inventory-sync]-> reserve-invocation
- Final state: cart empty, inventory has 2 units

### Inventory Insufficient Stock

**Setup:**
- Inventory has 2 units of item "widget"
- Cart has 3 units requested

**Flow:**
1. `Cart/checkout()` triggers sync
2. `Inventory/reserve(qty: 3)` fails with `InsufficientStock` error
3. Sync error-matching propagates to cart
4. Cart checkout fails with appropriate error

**Validation:**
- No partial state changes
- Error case properly typed and propagated
