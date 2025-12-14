package compiler

import (
	"fmt"
	"strings"

	"github.com/roach88/nysm/internal/ir"
)

// CycleWarning represents a potential cycle in sync rules.
//
// Cycles are warnings, not errors, because they may be intentional:
//   - Retry logic with error-case matching
//   - Recursive workflows with termination conditions
//   - Self-correcting feedback loops
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
//   - Retry logic with error-case matching
//   - Recursive workflows with termination conditions
//   - Self-correcting feedback loops
//
// The algorithm:
//  1. Build action → sync dependency graph from when/then clauses
//  2. Use Tarjan's algorithm to find strongly connected components
//  3. Report each SCC with size > 1 or self-loops as a potential cycle warning
//
// A DAG (no cycles) returns an empty warning list.
//
// TODO (future): Support @allow_recursion annotation to suppress specific warnings
func AnalyzeCycles(specs []ir.ConceptSpec, syncs []ir.SyncRule) []CycleWarning {
	if len(syncs) == 0 {
		return []CycleWarning{}
	}

	// Build dependency graph: sync_id → syncs that could be triggered
	graph := buildDependencyGraph(syncs)

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

// dependencyGraph maps sync_id → list of sync_ids that could be triggered.
type dependencyGraph map[string][]string

// buildDependencyGraph constructs the sync rule dependency graph.
//
// For each sync:
//   - Extract the action from then-clause (what it triggers)
//   - Find all syncs with when-clauses matching that action
//   - Add edges: this_sync → triggered_syncs
func buildDependencyGraph(syncs []ir.SyncRule) dependencyGraph {
	graph := make(dependencyGraph)

	// Build action → syncs mapping (which syncs match each action)
	// Key is the ActionRef (e.g., "Cart.checkout")
	actionToSyncs := make(map[string][]string)
	for _, sync := range syncs {
		action := sync.When.ActionRef
		actionToSyncs[action] = append(actionToSyncs[action], sync.ID)
	}

	// For each sync, find what syncs could be triggered by its then-clause
	for _, sync := range syncs {
		triggeredAction := sync.Then.ActionRef
		triggeredSyncs := actionToSyncs[triggeredAction]

		// Initialize with empty slice if no edges (ensures node exists in graph)
		if graph[sync.ID] == nil {
			graph[sync.ID] = []string{}
		}

		// Add edges to triggered syncs
		graph[sync.ID] = append(graph[sync.ID], triggeredSyncs...)
	}

	return graph
}

// hasSelfLoop checks if a node has an edge to itself.
func hasSelfLoop(node string, graph dependencyGraph) bool {
	for _, neighbor := range graph[node] {
		if neighbor == node {
			return true
		}
	}
	return false
}

// tarjanSCC finds strongly connected components using Tarjan's algorithm.
//
// Returns a list of SCCs, where each SCC is a list of sync IDs.
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
