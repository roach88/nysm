# Story 5.5: Compile-Time Cycle Analysis

Status: done

## Story

As a **developer defining sync rules**,
I want **potential cycles detected at compile time**,
So that **I catch problems before runtime**.

## Acceptance Criteria

1. **`AnalyzeCycles(specs []ConceptSpec, syncs []SyncRule) []CycleWarning` in `internal/compiler/cycle.go`**
   - Takes compiled concept specs and sync rules
   - Returns warnings (not errors - cycles may be intentional)
   - Performs static dependency analysis on sync rules

2. **CycleWarning struct defined**
   ```go
   type CycleWarning struct {
       Path    []string `json:"path"`    // ["sync-a", "sync-b", "sync-c", "sync-a"]
       Message string   `json:"message"` // Human-readable description
       Level   string   `json:"level"`   // "warning" or "info"
   }
   ```
   - Path shows the cycle sequence
   - Last element equals first element to show cycle completion
   - Message explains the potential issue

3. **Static dependency graph built from sync rules**
   - Extract action URIs from when-clauses (triggers)
   - Extract action URIs from then-clauses (effects)
   - Build directed graph: action → sync → action
   - Handle concept.action format (e.g., "Cart.checkout")

4. **Cycle detection algorithm implemented**
   - Use Tarjan's algorithm or similar for strongly connected component (SCC) detection
   - Detect self-loops (A → A)
   - Detect transitive cycles (A → B → C → A)
   - Return all detected cycles as warnings

5. **Warnings include full cycle path**
   ```go
   // Example warning for A→B→A cycle:
   CycleWarning{
       Path: ["cart-inventory", "inventory-response", "cart-inventory"],
       Message: "Potential cycle detected: cart-inventory → inventory-response → cart-inventory",
       Level: "warning",
   }

   // Example warning for self-loop:
   CycleWarning{
       Path: ["retry-on-failure", "retry-on-failure"],
       Message: "Self-triggering sync rule detected: retry-on-failure → retry-on-failure",
       Level: "warning",
   }
   ```

6. **DAG (Directed Acyclic Graph) passes analysis with no warnings**
   - Non-cyclic sync rules produce empty warning list
   - Return `[]CycleWarning{}` for valid DAG

7. **`@allow_recursion` annotation suppresses warnings (future enhancement)**
   - For now, document annotation intent in comments
   - Implementation deferred - always show warnings in MVP
   - Future: Parse annotation from CUE and suppress specific cycles

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CRITICAL-3** | Stratification analysis - detect cycles at compile time |
| **FR-1.2** | Validate specs against canonical IR schema |

## Tasks / Subtasks

- [ ] Task 1: Define cycle analysis types (AC: #2)
  - [ ] 1.1 Create `internal/compiler/cycle.go`
  - [ ] 1.2 Define CycleWarning struct
  - [ ] 1.3 Document warning levels and interpretation

- [ ] Task 2: Build dependency graph (AC: #3)
  - [ ] 2.1 Extract when-clause action URIs (triggers)
  - [ ] 2.2 Extract then-clause action URIs (effects)
  - [ ] 2.3 Build adjacency list: map[string][]string (sync_id → triggered_sync_ids)
  - [ ] 2.4 Handle concept.action format parsing

- [ ] Task 3: Implement cycle detection (AC: #4)
  - [ ] 3.1 Implement Tarjan's SCC algorithm
  - [ ] 3.2 Detect self-loops (size-1 SCCs)
  - [ ] 3.3 Detect multi-node cycles (size-N SCCs)
  - [ ] 3.4 Extract cycle paths from SCCs

- [ ] Task 4: Generate warnings (AC: #5, #6)
  - [ ] 4.1 Convert detected cycles to CycleWarning structs
  - [ ] 4.2 Format human-readable messages
  - [ ] 4.3 Return empty slice for DAG
  - [ ] 4.4 Include full path in warning

- [ ] Task 5: Write comprehensive tests (all AC)
  - [ ] 5.1 Test direct self-loop (A → A)
  - [ ] 5.2 Test 2-cycle (A → B → A)
  - [ ] 5.3 Test 3-cycle (A → B → C → A)
  - [ ] 5.4 Test DAG (no warnings)
  - [ ] 5.5 Test multiple independent cycles
  - [ ] 5.6 Test cycle path formatting
  - [ ] 5.7 Test empty input (no syncs)

## Dev Notes

### Cycle Detection Algorithm

**Tarjan's Strongly Connected Component (SCC) Algorithm:**

```go
// internal/compiler/cycle.go
package compiler

import (
    "fmt"
    "strings"

    "github.com/tyler/nysm/internal/ir"
)

// CycleWarning represents a potential cycle in sync rules.
// Cycles are warnings, not errors, because they may be intentional
// (e.g., retry logic, recursive workflows with termination conditions).
type CycleWarning struct {
    Path    []string `json:"path"`    // Cycle path: ["sync-a", "sync-b", "sync-a"]
    Message string   `json:"message"` // Human-readable description
    Level   string   `json:"level"`   // "warning" or "info"
}

// AnalyzeCycles performs static cycle analysis on sync rules.
//
// This implements CRITICAL-3 stratification analysis. It builds a dependency
// graph from sync rules and detects strongly connected components (cycles).
//
// Cycles are reported as warnings (not errors) because they may be intentional:
// - Retry logic with error-case matching
// - Recursive workflows with termination conditions
// - Self-correcting feedback loops
//
// The algorithm:
// 1. Build action → sync dependency graph from when/then clauses
// 2. Use Tarjan's algorithm to find strongly connected components
// 3. Report each SCC as a potential cycle warning
//
// A DAG (no cycles) returns an empty warning list.
//
// TODO (future): Support @allow_recursion annotation to suppress specific warnings
func AnalyzeCycles(specs []ir.ConceptSpec, syncs []ir.SyncRule) []CycleWarning {
    if len(syncs) == 0 {
        return []CycleWarning{}
    }

    // Build dependency graph: sync_id → action triggered → syncs that match that action
    graph := buildDependencyGraph(specs, syncs)

    // Detect strongly connected components (cycles)
    sccs := tarjanSCC(graph)

    // Convert SCCs to warnings
    var warnings []CycleWarning
    for _, scc := range sccs {
        if len(scc) > 1 || (len(scc) == 1 && hasSelfLoop(scc[0], graph)) {
            warning := cycleSCCToWarning(scc, graph)
            warnings = append(warnings, warning)
        }
    }

    return warnings
}

// dependencyGraph maps sync_id → list of sync_ids that could be triggered
type dependencyGraph map[string][]string

// buildDependencyGraph constructs the sync rule dependency graph.
//
// For each sync:
// - Extract the action from then-clause (what it triggers)
// - Find all syncs with when-clauses matching that action
// - Add edges: this_sync → triggered_syncs
func buildDependencyGraph(specs []ir.ConceptSpec, syncs []ir.SyncRule) dependencyGraph {
    graph := make(dependencyGraph)

    // Build action → syncs mapping (which syncs match each action)
    actionToSyncs := make(map[string][]string)
    for _, sync := range syncs {
        action := sync.When.Action
        actionToSyncs[string(action)] = append(actionToSyncs[string(action)], sync.ID)
    }

    // For each sync, find what it triggers
    for _, sync := range syncs {
        triggeredAction := sync.Then.Action
        triggeredSyncs := actionToSyncs[string(triggeredAction)]
        graph[sync.ID] = triggeredSyncs
    }

    return graph
}

// hasSelfLoop checks if a node has an edge to itself
func hasSelfLoop(node string, graph dependencyGraph) bool {
    neighbors := graph[node]
    for _, neighbor := range neighbors {
        if neighbor == node {
            return true
        }
    }
    return false
}

// tarjanSCC finds strongly connected components using Tarjan's algorithm.
//
// Returns a list of SCCs, where each SCC is a list of sync IDs forming a cycle.
// Single-node SCCs without self-loops are NOT cycles.
func tarjanSCC(graph dependencyGraph) [][]string {
    var (
        index   = 0
        stack   []string
        indices = make(map[string]int)
        lowlink = make(map[string]int)
        onStack = make(map[string]bool)
        sccs    [][]string
    )

    var strongConnect func(string)
    strongConnect = func(v string) {
        // Set the depth index for v
        indices[v] = index
        lowlink[v] = index
        index++
        stack = append(stack, v)
        onStack[v] = true

        // Consider successors of v
        for _, w := range graph[v] {
            if _, visited := indices[w]; !visited {
                // Successor w has not yet been visited; recurse on it
                strongConnect(w)
                lowlink[v] = min(lowlink[v], lowlink[w])
            } else if onStack[w] {
                // Successor w is on stack and hence in the current SCC
                lowlink[v] = min(lowlink[v], indices[w])
            }
        }

        // If v is a root node, pop the stack and create an SCC
        if lowlink[v] == indices[v] {
            var scc []string
            for {
                w := stack[len(stack)-1]
                stack = stack[:len(stack)-1]
                onStack[w] = false
                scc = append(scc, w)
                if w == v {
                    break
                }
            }
            sccs = append(sccs, scc)
        }
    }

    // Visit all nodes
    for node := range graph {
        if _, visited := indices[node]; !visited {
            strongConnect(node)
        }
    }

    return sccs
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// cycleSCCToWarning converts an SCC to a CycleWarning.
//
// The path shows the cycle sequence by reconstructing a path through the SCC.
// For self-loops, the path is [sync-id, sync-id].
// For multi-node cycles, the path shows a cycle traversal.
func cycleSCCToWarning(scc []string, graph dependencyGraph) CycleWarning {
    if len(scc) == 1 {
        // Self-loop
        syncID := scc[0]
        return CycleWarning{
            Path:    []string{syncID, syncID},
            Message: fmt.Sprintf("Self-triggering sync rule detected: %s → %s", syncID, syncID),
            Level:   "warning",
        }
    }

    // Multi-node cycle - reconstruct a cycle path
    path := reconstructCyclePath(scc, graph)

    pathStr := strings.Join(path, " → ")
    return CycleWarning{
        Path:    path,
        Message: fmt.Sprintf("Potential cycle detected: %s", pathStr),
        Level:   "warning",
    }
}

// reconstructCyclePath builds a cycle path from an SCC.
//
// Strategy: Start at first node in SCC, follow edges to other SCC members,
// continue until we return to start node.
func reconstructCyclePath(scc []string, graph dependencyGraph) []string {
    if len(scc) == 0 {
        return []string{}
    }

    // Build set of SCC members for fast lookup
    sccSet := make(map[string]bool)
    for _, node := range scc {
        sccSet[node] = true
    }

    // Start at first node
    start := scc[0]
    current := start
    path := []string{current}
    visited := make(map[string]bool)

    // Follow edges within SCC until we return to start
    for {
        visited[current] = true

        // Find next SCC member reachable from current
        var next string
        for _, neighbor := range graph[current] {
            if sccSet[neighbor] && (!visited[neighbor] || neighbor == start) {
                next = neighbor
                break
            }
        }

        if next == "" {
            // No more unvisited neighbors in SCC
            break
        }

        path = append(path, next)

        if next == start {
            // Completed the cycle
            break
        }

        current = next
    }

    return path
}
```

### Graph Building Examples

```go
// Example 1: Direct self-loop (A → A)
// sync "retry-on-failure" {
//     when: ProcessJob.completed { case: "Failure" }
//     then: ProcessJob.retry
// }
//
// Graph: {"retry-on-failure": ["retry-on-failure"]}
// SCC: [["retry-on-failure"]]
// Warning: Self-triggering sync rule detected: retry-on-failure → retry-on-failure

// Example 2: Two-node cycle (A → B → A)
// sync "cart-inventory" {
//     when: Cart.checkout.completed
//     then: Inventory.reserve
// }
// sync "inventory-cart" {
//     when: Inventory.reserve.completed { case: "InsufficientStock" }
//     then: Cart.checkout.fail
// }
//
// Graph: {
//     "cart-inventory": ["inventory-cart"],
//     "inventory-cart": ["cart-inventory"]
// }
// SCC: [["cart-inventory", "inventory-cart"]]
// Warning: Potential cycle detected: cart-inventory → inventory-cart → cart-inventory

// Example 3: Three-node cycle (A → B → C → A)
// sync "a" { when: X.completed, then: Y.invoke }
// sync "b" { when: Y.completed, then: Z.invoke }
// sync "c" { when: Z.completed, then: X.invoke }
//
// Graph: {
//     "a": ["b"],
//     "b": ["c"],
//     "c": ["a"]
// }
// SCC: [["a", "b", "c"]]
// Warning: Potential cycle detected: a → b → c → a

// Example 4: DAG (no cycles)
// sync "checkout" { when: Cart.checkout.completed, then: Inventory.reserve }
// sync "payment" { when: Cart.checkout.completed, then: Payment.charge }
//
// Graph: {
//     "checkout": ["inventory-reserve"],  // assuming inventory-reserve matches Inventory.reserve
//     "payment": ["payment-charge"]       // assuming payment-charge matches Payment.charge
// }
// (No SCC with size > 1, no self-loops)
// Result: []CycleWarning{}
```

### Test Examples

```go
// internal/compiler/cycle_test.go
package compiler

import (
    "testing"

    "github.com/tyler/nysm/internal/ir"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAnalyzeCycles_Empty(t *testing.T) {
    warnings := AnalyzeCycles(nil, nil)
    assert.Empty(t, warnings, "no syncs should produce no warnings")
}

func TestAnalyzeCycles_DAG(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "Cart", Actions: []ir.ActionSig{{Name: "checkout"}}},
        {Name: "Inventory", Actions: []ir.ActionSig{{Name: "reserve"}}},
        {Name: "Payment", Actions: []ir.ActionSig{{Name: "charge"}}},
    }

    syncs := []ir.SyncRule{
        {
            ID: "checkout-inventory",
            When: ir.WhenClause{
                Action: "Cart.checkout",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "Inventory.reserve",
            },
        },
        {
            ID: "checkout-payment",
            When: ir.WhenClause{
                Action: "Cart.checkout",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "Payment.charge",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    assert.Empty(t, warnings, "DAG should produce no cycle warnings")
}

func TestAnalyzeCycles_SelfLoop(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "Job", Actions: []ir.ActionSig{{Name: "retry"}}},
    }

    syncs := []ir.SyncRule{
        {
            ID: "retry-on-failure",
            When: ir.WhenClause{
                Action:     "Job.retry",
                Event:      "completed",
                OutputCase: strPtr("Failure"),
            },
            Then: ir.ThenClause{
                Action: "Job.retry",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    require.Len(t, warnings, 1)

    warning := warnings[0]
    assert.Equal(t, []string{"retry-on-failure", "retry-on-failure"}, warning.Path)
    assert.Contains(t, warning.Message, "Self-triggering")
    assert.Equal(t, "warning", warning.Level)
}

func TestAnalyzeCycles_TwoNodeCycle(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "Cart", Actions: []ir.ActionSig{{Name: "checkout"}}},
        {Name: "Inventory", Actions: []ir.ActionSig{{Name: "reserve"}}},
    }

    syncs := []ir.SyncRule{
        {
            ID: "cart-inventory",
            When: ir.WhenClause{
                Action: "Cart.checkout",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "Inventory.reserve",
            },
        },
        {
            ID: "inventory-cart",
            When: ir.WhenClause{
                Action:     "Inventory.reserve",
                Event:      "completed",
                OutputCase: strPtr("InsufficientStock"),
            },
            Then: ir.ThenClause{
                Action: "Cart.checkout",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    require.Len(t, warnings, 1)

    warning := warnings[0]
    assert.Len(t, warning.Path, 3, "2-cycle path should have 3 elements: A → B → A")
    assert.Equal(t, warning.Path[0], warning.Path[len(warning.Path)-1], "cycle should return to start")
    assert.Contains(t, warning.Message, "Potential cycle")
    assert.Equal(t, "warning", warning.Level)
}

func TestAnalyzeCycles_ThreeNodeCycle(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "A", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "B", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "C", Actions: []ir.ActionSig{{Name: "action"}}},
    }

    syncs := []ir.SyncRule{
        {
            ID: "sync-a",
            When: ir.WhenClause{
                Action: "A.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "B.action",
            },
        },
        {
            ID: "sync-b",
            When: ir.WhenClause{
                Action: "B.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "C.action",
            },
        },
        {
            ID: "sync-c",
            When: ir.WhenClause{
                Action: "C.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "A.action",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    require.Len(t, warnings, 1)

    warning := warnings[0]
    assert.Len(t, warning.Path, 4, "3-cycle path should have 4 elements: A → B → C → A")
    assert.Equal(t, warning.Path[0], warning.Path[len(warning.Path)-1], "cycle should return to start")
    assert.Contains(t, warning.Message, "Potential cycle")
}

func TestAnalyzeCycles_MultipleIndependentCycles(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "A", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "B", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "X", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "Y", Actions: []ir.ActionSig{{Name: "action"}}},
    }

    syncs := []ir.SyncRule{
        // Cycle 1: A ↔ B
        {
            ID: "a-to-b",
            When: ir.WhenClause{
                Action: "A.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "B.action",
            },
        },
        {
            ID: "b-to-a",
            When: ir.WhenClause{
                Action: "B.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "A.action",
            },
        },
        // Cycle 2: X ↔ Y
        {
            ID: "x-to-y",
            When: ir.WhenClause{
                Action: "X.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "Y.action",
            },
        },
        {
            ID: "y-to-x",
            When: ir.WhenClause{
                Action: "Y.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "X.action",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    require.Len(t, warnings, 2, "should detect both independent cycles")

    // Both warnings should be for 2-node cycles
    for _, warning := range warnings {
        assert.Len(t, warning.Path, 3, "each 2-cycle should have 3 elements")
        assert.Equal(t, warning.Path[0], warning.Path[2], "each cycle should return to start")
    }
}

func TestAnalyzeCycles_PathFormatting(t *testing.T) {
    specs := []ir.ConceptSpec{
        {Name: "A", Actions: []ir.ActionSig{{Name: "action"}}},
        {Name: "B", Actions: []ir.ActionSig{{Name: "action"}}},
    }

    syncs := []ir.SyncRule{
        {
            ID: "first-sync",
            When: ir.WhenClause{
                Action: "A.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "B.action",
            },
        },
        {
            ID: "second-sync",
            When: ir.WhenClause{
                Action: "B.action",
                Event:  "completed",
            },
            Then: ir.ThenClause{
                Action: "A.action",
            },
        },
    }

    warnings := AnalyzeCycles(specs, syncs)
    require.Len(t, warnings, 1)

    warning := warnings[0]
    assert.Contains(t, warning.Message, "first-sync")
    assert.Contains(t, warning.Message, "second-sync")
    assert.Contains(t, warning.Message, "→")  // Arrow separator
}

// Helper function
func strPtr(s string) *string {
    return &s
}
```

### File List

Files to create/modify:

1. `internal/compiler/cycle.go` - AnalyzeCycles function, CycleWarning struct, Tarjan's SCC algorithm
2. `internal/compiler/cycle_test.go` - Comprehensive cycle detection tests

### Relationship to Other Stories

**Dependencies:**
- **Story 1.6 (CUE Concept Spec Parser)** - Uses ir.ConceptSpec for action definitions
- **Story 1.7 (CUE Sync Rule Parser)** - Uses ir.SyncRule for when/then clauses
- **Story 3.2 (Sync Rule Registration and Declaration Order)** - Cycle warnings inform registration decisions

**Enables:**
- **Story 5.3 (Cycle Detection per Flow)** - Runtime cycle detection uses similar concepts
- **Story 5.4 (Max-Steps Quota Enforcement)** - Quota prevents unbounded cycles
- **Story 1.8 (IR Schema Validation)** - Validation can include cycle warnings

**Related:**
- **Story 3.3 (When-Clause Matching)** - Understanding when-clauses helps build dependency graph
- **Story 4.5 (Then-Clause Invocation Generation)** - Understanding then-clauses helps build dependency graph

### Story Completion Checklist

- [ ] `internal/compiler/cycle.go` created
- [ ] CycleWarning struct defined
- [ ] AnalyzeCycles function signature implemented
- [ ] Dependency graph builder implemented
- [ ] When-clause action extraction working
- [ ] Then-clause action extraction working
- [ ] Tarjan's SCC algorithm implemented
- [ ] Self-loop detection working
- [ ] Multi-node cycle detection working
- [ ] Cycle path reconstruction working
- [ ] Warning message formatting complete
- [ ] DAG passes with no warnings
- [ ] Test: empty input
- [ ] Test: DAG (no cycles)
- [ ] Test: direct self-loop (A → A)
- [ ] Test: 2-node cycle (A → B → A)
- [ ] Test: 3-node cycle (A → B → C → A)
- [ ] Test: multiple independent cycles
- [ ] Test: path formatting
- [ ] All tests pass (`go test ./internal/compiler/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/compiler` passes
- [ ] Test coverage > 90% for cycle.go
- [ ] Documentation complete

### References

- [Source: docs/architecture.md#CRITICAL-3] - Sync engine termination semantics, stratification analysis
- [Source: docs/epics.md#Story 5.5] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-1.2] - Validate specs against canonical IR schema
- [Source: docs/architecture.md#Implementation Patterns] - Error handling and warning patterns

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: 2025-12-12

### Completion Notes

- Implements CRITICAL-3 stratification analysis at compile time
- Uses Tarjan's algorithm for strongly connected component detection
- Warnings (not errors) - cycles may be intentional for retry/recursive workflows
- Full cycle path reconstruction shows exact sequence of syncs
- Self-loops detected as size-1 SCCs with self-edges
- Multi-node cycles detected as size-N SCCs
- DAG produces empty warning list
- Graph built from when-clause (triggers) and then-clause (effects)
- Action URI format: "Concept.action" (e.g., "Cart.checkout")
- Future enhancement: @allow_recursion annotation to suppress specific warnings
- Complements runtime cycle detection (Story 5.3) and quota enforcement (Story 5.4)
