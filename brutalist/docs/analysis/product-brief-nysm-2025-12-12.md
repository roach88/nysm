---
stepsCompleted: [1, 2, 3, 4, 5, 6]
inputDocuments:
  - docs/analysis/brainstorming-session-2025-12-03.md
  - initial-report.md
workflowType: 'product-brief'
lastStep: 1
project_name: 'nysm'
user_name: 'Tyler'
date: '2025-12-12'
---

# Product Brief: NYSM

**Date:** 2025-12-12
**Author:** Tyler

---

## Executive Summary

NYSM (Now You See Me) is a software development framework implementing the "What You See Is What It Does" (WYSIWYG) pattern from the academic paper at arxiv.org/abs/2508.14511. The framework makes software inherently legible—both for humans and AI—by ensuring code structure directly corresponds to system behavior.

The goal is to enable developers to build and maintain software more efficiently with AI assistance, producing applications that AI can realistically maintain either fully or partially on its own. NYSM encompasses the framework, reference implementation, and tooling needed to write software in this paradigm.

---

## Core Vision

### Problem Statement

Software is fundamentally hard to read and maintain—for both humans and AI. This problem has existed since humans first started coding and has only worsened as the number of languages, frameworks, and surfaces for software has exponentially increased. Documentation rots, tests don't explain intent, logs are noise, and the gap between what code looks like and what it does creates endless friction.

### Problem Impact

- Technical debt spirals as systems become opaque
- AI assistants struggle to understand and safely modify code
- Onboarding new developers takes forever
- Debugging distributed systems remains painful
- "Why did this happen?" is often unanswerable

### Why Existing Solutions Fall Short

Clean code principles, documentation, testing, and observability tools all attempt to bridge the gap between code and behavior—but they're bolted on, not structural. Documentation drifts from reality. Tests verify behavior but don't explain architecture. Logs capture events but not causality. None of these make the code itself legible.

### Proposed Solution

NYSM implements a structural pattern where legibility is inherent, not added:

- **Concept Specs** — Self-contained services with purpose, relational state schema, action signatures (with typed outputs and error cases), and operational principles that become executable contract tests
- **Synchronizations** — A 3-clause DSL (`when` → `where` → `then`) for event-based coordination between concepts
- **Flow Tokens** — Explicit scoping mechanism preventing accidental joins across concurrent requests
- **Provenance Edges** — Structural audit trail enabling idempotency, replay, and answering "why did this happen?"
- **No transactions** — Integrity via rule scheme + flow scoping + error-matching
- **No getters** — Reads via query, writes via actions only

### Key Differentiators

- **AI-native legibility** — Code structure IS the documentation; AI can read, understand, and maintain it
- **Operational Principles as tests** — Documentation that can't rot because it's validated
- **Deterministic replay** — Every derived action is explainable via provenance path
- **Structural integrity** — No transactions needed; the architecture enforces correctness

### Technical Approach Note

**Query Substrate Strategy:** The initial implementation will use SQL-first for the `where` clause (compiling DSL queries to SQL over relational state tables) to enable rapid iteration with familiar tooling. The architecture will maintain a clean abstraction boundary to support migration to RDF + SPARQL (the paper's native substrate) in a future phase, preserving full fidelity to the paper's graph-based query semantics.

---

## Target Users

### Primary Users

**The Solo Developer**

Builders working on side projects and indie apps in the margins of their time. They're building the thing they've always wanted to build, but the traditional pain of software development fights them at every turn.

**Their Pain:**
- Coming back to code after days or weeks away and losing context
- The more complex the project gets, the harder it is to remember what does what
- Getting gummed up in implementation details instead of thinking about software the way humans naturally think

**What Success Looks Like:**
- Easy to understand how it works, what the rules are, and how to extend it
- Like sitting down to play chess: quick to learn the pieces and rules, but reveals tremendous depth the more you engage
- Simplicity of the working space enables creativity rather than fighting complexity

---

**The AI Agent**

Coding assistants (like Claude Code) and autonomous agents that write, review, and maintain software.

**Their Pain:**
- Human-written code is filled with implicit decisions that can't be inferred
- Traditional interdependencies make it easy to make a locally-sensible change that breaks something unexpected elsewhere
- The token/compute burn of accounting for all potential side effects on every code change is massive overhead

**What Success Looks Like:**
- Intuitive to navigate and code in
- Structure eliminates risk of collapsing the platform through cascading dependency failures
- Enables AI to prototype features, make fixes, and express creativity without traditional risks
- Shifts the paradigm toward software that's built with coding assistance and maintained by AI long-term

### Human-AI Partnership

NYSM treats humans and AI as equal partners in the development sandbox:

- Human writes code, AI reviews and fact-checks
- AI writes code, human reviews
- Either can take the lead - the framework doesn't care

The structural legibility eliminates the misunderstandings that plague human-AI collaboration today: implicit decisions, hidden dependencies, and the constant risk of unexpected breakage.

---

## Success Metrics

NYSM is a personal framework project. Success is defined by:

- **It works:** The framework implements the WYSIWYG paper faithfully
- **It's usable:** Tyler can build real applications with it
- **It's legible:** Both humans and AI can understand and modify NYSM codebases without friction

No formal KPIs or business objectives apply at this stage.

---

## MVP Scope

### Core Features

**Phase 1: Canonical Demo Specs**
- `Cart.concept.cue`, `Inventory.concept.cue`, `Web.concept.cue`
- `cart-inventory.sync.cue` demonstrating the 3-clause DSL
- 1 Operational Principle per concept describing end-to-end flow

**Phase 2: Minimal Core IR + Schema**
- Canonical IR: ConceptSpec, ActionSig, StateSchema, SyncRule, Invocation, Completion, FlowToken, ProvenanceEdge
- JSON Schema for IR (versioned, forward-compatible)
- CUE as v0 surface format — compiles to canonical IR
- Formal execution semantics for when-match, where-binding, then-invoke

**Phase 3: Durable Engine**
- SQLite-backed append-only log for invocations/completions/provenance
- Idempotency via sync edges check
- Crash/restart/replay yields identical results

**Phase 4: SQL-first Where-Clause**
- Compile DSL where-clause to SQL over relational state tables
- Clean abstraction boundary for future SPARQL migration
- Binding model: query returns set of bindings for then-invocations

**Phase 5: Conformance Harness**
- Test runner that loads specs + syncs and runs scenarios
- Assertions on action traces + final queried state
- Operational Principles as executable contract tests
- Golden trace snapshots for the canonical demo

### Out of Scope for MVP

- **Custom DSL syntax** — CUE serves as v0 surface; custom `.concept`/`.sync` DSL deferred until semantics stabilize
- **RDF/SPARQL** — Planned for future phase; abstraction boundary maintained in IR
- **Web bootstrap as ordinary concept** — HTTP adapter deferred to Phase 6
- **Observability tooling** — Flow graphs and "why" queries deferred to Phase 7
- **SDK/codegen ergonomics** — Focus on core semantics first
- **Multiple real-world apps** — One canonical demo is sufficient for MVP

### Technical Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Surface format (v0) | CUE | Typed constraints, validation, instant tooling |
| Canonical IR | JSON Schema | Version-controlled, language-agnostic |
| Query substrate (v0) | SQL | Rapid iteration; abstraction boundary for SPARQL later |
| Custom DSL | Deferred | Add once semantics stabilize; compiles to same IR |

### Future Vision

- **Custom DSL** — HCL/KDL-style syntax (keyword + braces) that compiles to canonical IR
- Migration to RDF + SPARQL for full paper fidelity
- Web/HTTP as an ordinary concept (not middleware)
- First-class observability: flow timeline, provenance graphs, "why did X happen?" queries
- SDK with typed stubs generated from concept specs
- Real applications built on NYSM demonstrating human-AI collaborative development
