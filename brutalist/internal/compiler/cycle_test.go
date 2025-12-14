package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

// TestAnalyzeCycles_Empty tests that empty input produces no warnings.
func TestAnalyzeCycles_Empty(t *testing.T) {
	warnings := AnalyzeCycles(nil, nil)
	assert.Empty(t, warnings, "no syncs should produce no warnings")
}

// TestAnalyzeCycles_EmptySyncs tests empty sync slice produces no warnings.
func TestAnalyzeCycles_EmptySyncs(t *testing.T) {
	specs := []ir.ConceptSpec{
		{Name: "Cart", Actions: []ir.ActionSig{{Name: "checkout"}}},
	}
	warnings := AnalyzeCycles(specs, []ir.SyncRule{})
	assert.Empty(t, warnings, "empty syncs should produce no warnings")
}

// TestAnalyzeCycles_DAG tests that a directed acyclic graph produces no warnings.
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
				ActionRef: "Cart.checkout",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Inventory.reserve",
			},
		},
		{
			ID: "checkout-payment",
			When: ir.WhenClause{
				ActionRef: "Cart.checkout",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Payment.charge",
			},
		},
	}

	warnings := AnalyzeCycles(specs, syncs)
	assert.Empty(t, warnings, "DAG should produce no cycle warnings")
}

// TestAnalyzeCycles_SelfLoop tests detection of a self-triggering sync rule.
func TestAnalyzeCycles_SelfLoop(t *testing.T) {
	specs := []ir.ConceptSpec{
		{Name: "Job", Actions: []ir.ActionSig{{Name: "retry"}}},
	}

	syncs := []ir.SyncRule{
		{
			ID: "retry-on-failure",
			When: ir.WhenClause{
				ActionRef:  "Job.retry",
				EventType:  "completed",
				OutputCase: "Failure",
			},
			Then: ir.ThenClause{
				ActionRef: "Job.retry",
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

// TestAnalyzeCycles_TwoNodeCycle tests detection of A → B → A cycle.
func TestAnalyzeCycles_TwoNodeCycle(t *testing.T) {
	specs := []ir.ConceptSpec{
		{Name: "Cart", Actions: []ir.ActionSig{{Name: "checkout"}}},
		{Name: "Inventory", Actions: []ir.ActionSig{{Name: "reserve"}}},
	}

	syncs := []ir.SyncRule{
		{
			ID: "cart-inventory",
			When: ir.WhenClause{
				ActionRef: "Cart.checkout",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Inventory.reserve",
			},
		},
		{
			ID: "inventory-cart",
			When: ir.WhenClause{
				ActionRef:  "Inventory.reserve",
				EventType:  "completed",
				OutputCase: "InsufficientStock",
			},
			Then: ir.ThenClause{
				ActionRef: "Cart.checkout",
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

// TestAnalyzeCycles_ThreeNodeCycle tests detection of A → B → C → A cycle.
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
				ActionRef: "A.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "B.action",
			},
		},
		{
			ID: "sync-b",
			When: ir.WhenClause{
				ActionRef: "B.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "C.action",
			},
		},
		{
			ID: "sync-c",
			When: ir.WhenClause{
				ActionRef: "C.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "A.action",
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

// TestAnalyzeCycles_MultipleIndependentCycles tests detection of multiple separate cycles.
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
				ActionRef: "A.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "B.action",
			},
		},
		{
			ID: "b-to-a",
			When: ir.WhenClause{
				ActionRef: "B.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "A.action",
			},
		},
		// Cycle 2: X ↔ Y
		{
			ID: "x-to-y",
			When: ir.WhenClause{
				ActionRef: "X.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Y.action",
			},
		},
		{
			ID: "y-to-x",
			When: ir.WhenClause{
				ActionRef: "Y.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "X.action",
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

// TestAnalyzeCycles_PathFormatting tests that cycle path message is formatted correctly.
func TestAnalyzeCycles_PathFormatting(t *testing.T) {
	specs := []ir.ConceptSpec{
		{Name: "A", Actions: []ir.ActionSig{{Name: "action"}}},
		{Name: "B", Actions: []ir.ActionSig{{Name: "action"}}},
	}

	syncs := []ir.SyncRule{
		{
			ID: "first-sync",
			When: ir.WhenClause{
				ActionRef: "A.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "B.action",
			},
		},
		{
			ID: "second-sync",
			When: ir.WhenClause{
				ActionRef: "B.action",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "A.action",
			},
		},
	}

	warnings := AnalyzeCycles(specs, syncs)
	require.Len(t, warnings, 1)

	warning := warnings[0]
	// Path should include both sync IDs
	pathContainsFirst := false
	pathContainsSecond := false
	for _, p := range warning.Path {
		if p == "first-sync" {
			pathContainsFirst = true
		}
		if p == "second-sync" {
			pathContainsSecond = true
		}
	}
	assert.True(t, pathContainsFirst, "path should contain first-sync")
	assert.True(t, pathContainsSecond, "path should contain second-sync")
	assert.Contains(t, warning.Message, "→", "message should contain arrow separator")
}

// TestAnalyzeCycles_SingleSyncNoLoop tests a single sync that doesn't loop.
func TestAnalyzeCycles_SingleSyncNoLoop(t *testing.T) {
	specs := []ir.ConceptSpec{
		{Name: "Cart", Actions: []ir.ActionSig{{Name: "checkout"}}},
		{Name: "Inventory", Actions: []ir.ActionSig{{Name: "reserve"}}},
	}

	syncs := []ir.SyncRule{
		{
			ID: "checkout-inventory",
			When: ir.WhenClause{
				ActionRef: "Cart.checkout",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Inventory.reserve",
			},
		},
	}

	warnings := AnalyzeCycles(specs, syncs)
	assert.Empty(t, warnings, "single non-looping sync should produce no warnings")
}

// TestAnalyzeCycles_SelfLoopMessage tests message format for self-loop.
func TestAnalyzeCycles_SelfLoopMessage(t *testing.T) {
	syncs := []ir.SyncRule{
		{
			ID: "auto-retry",
			When: ir.WhenClause{
				ActionRef: "Service.call",
				EventType: "completed",
			},
			Then: ir.ThenClause{
				ActionRef: "Service.call",
			},
		},
	}

	warnings := AnalyzeCycles(nil, syncs)
	require.Len(t, warnings, 1)

	warning := warnings[0]
	assert.Contains(t, warning.Message, "Self-triggering")
	assert.Contains(t, warning.Message, "auto-retry")
	assert.Contains(t, warning.Message, "→")
}

// TestAnalyzeCycles_CycleWithUnconnectedSyncs tests cycle detection ignores unconnected syncs.
func TestAnalyzeCycles_CycleWithUnconnectedSyncs(t *testing.T) {
	syncs := []ir.SyncRule{
		// Cycle: A → B → A
		{
			ID:   "sync-a",
			When: ir.WhenClause{ActionRef: "A.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "B.action"},
		},
		{
			ID:   "sync-b",
			When: ir.WhenClause{ActionRef: "B.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "A.action"},
		},
		// Unconnected syncs (no cycle)
		{
			ID:   "sync-x",
			When: ir.WhenClause{ActionRef: "X.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "Y.action"},
		},
		{
			ID:   "sync-y",
			When: ir.WhenClause{ActionRef: "Y.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "Z.action"},
		},
	}

	warnings := AnalyzeCycles(nil, syncs)
	require.Len(t, warnings, 1, "should only detect the A→B→A cycle")

	warning := warnings[0]
	// Should not include X or Y syncs in the path
	for _, p := range warning.Path {
		assert.NotEqual(t, "sync-x", p)
		assert.NotEqual(t, "sync-y", p)
	}
}

// TestAnalyzeCycles_LargerCycle tests detection of longer cycles.
func TestAnalyzeCycles_LargerCycle(t *testing.T) {
	syncs := []ir.SyncRule{
		{ID: "s1", When: ir.WhenClause{ActionRef: "A1", EventType: "completed"}, Then: ir.ThenClause{ActionRef: "A2"}},
		{ID: "s2", When: ir.WhenClause{ActionRef: "A2", EventType: "completed"}, Then: ir.ThenClause{ActionRef: "A3"}},
		{ID: "s3", When: ir.WhenClause{ActionRef: "A3", EventType: "completed"}, Then: ir.ThenClause{ActionRef: "A4"}},
		{ID: "s4", When: ir.WhenClause{ActionRef: "A4", EventType: "completed"}, Then: ir.ThenClause{ActionRef: "A5"}},
		{ID: "s5", When: ir.WhenClause{ActionRef: "A5", EventType: "completed"}, Then: ir.ThenClause{ActionRef: "A1"}},
	}

	warnings := AnalyzeCycles(nil, syncs)
	require.Len(t, warnings, 1)

	warning := warnings[0]
	assert.Len(t, warning.Path, 6, "5-cycle path should have 6 elements: s1 → s2 → s3 → s4 → s5 → s1")
	assert.Equal(t, warning.Path[0], warning.Path[len(warning.Path)-1], "cycle should return to start")
}

// TestAnalyzeCycles_CycleWarningStruct tests CycleWarning struct fields.
func TestAnalyzeCycles_CycleWarningStruct(t *testing.T) {
	syncs := []ir.SyncRule{
		{
			ID:   "test-sync",
			When: ir.WhenClause{ActionRef: "Test.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "Test.action"},
		},
	}

	warnings := AnalyzeCycles(nil, syncs)
	require.Len(t, warnings, 1)

	warning := warnings[0]
	assert.NotEmpty(t, warning.Path)
	assert.NotEmpty(t, warning.Message)
	assert.Equal(t, "warning", warning.Level)
}

// TestBuildDependencyGraph_Basic tests dependency graph construction.
func TestBuildDependencyGraph_Basic(t *testing.T) {
	syncs := []ir.SyncRule{
		{
			ID:   "sync-a",
			When: ir.WhenClause{ActionRef: "A.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "B.action"},
		},
		{
			ID:   "sync-b",
			When: ir.WhenClause{ActionRef: "B.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "C.action"},
		},
	}

	graph := buildDependencyGraph(syncs)

	// sync-a triggers B.action, which sync-b listens to
	assert.Contains(t, graph["sync-a"], "sync-b")

	// sync-b triggers C.action, nothing listens to it
	assert.Empty(t, graph["sync-b"])
}

// TestBuildDependencyGraph_MultipleListeners tests multiple syncs listening to same action.
func TestBuildDependencyGraph_MultipleListeners(t *testing.T) {
	syncs := []ir.SyncRule{
		{
			ID:   "trigger",
			When: ir.WhenClause{ActionRef: "Start.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "Common.action"},
		},
		{
			ID:   "listener-1",
			When: ir.WhenClause{ActionRef: "Common.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "End1.action"},
		},
		{
			ID:   "listener-2",
			When: ir.WhenClause{ActionRef: "Common.action", EventType: "completed"},
			Then: ir.ThenClause{ActionRef: "End2.action"},
		},
	}

	graph := buildDependencyGraph(syncs)

	// trigger should lead to both listeners
	assert.Contains(t, graph["trigger"], "listener-1")
	assert.Contains(t, graph["trigger"], "listener-2")
}

// TestHasSelfLoop tests self-loop detection.
func TestHasSelfLoop(t *testing.T) {
	graph := dependencyGraph{
		"self-loop": {"self-loop"},
		"no-loop":   {"other"},
		"no-edges":  {},
	}

	assert.True(t, hasSelfLoop("self-loop", graph))
	assert.False(t, hasSelfLoop("no-loop", graph))
	assert.False(t, hasSelfLoop("no-edges", graph))
}

// TestTarjanSCC_SingleNode tests Tarjan with single node.
func TestTarjanSCC_SingleNode(t *testing.T) {
	graph := dependencyGraph{
		"a": {},
	}

	sccs := tarjanSCC(graph)
	require.Len(t, sccs, 1)
	assert.Equal(t, []string{"a"}, sccs[0])
}

// TestTarjanSCC_TwoNodeCycle tests Tarjan with two-node cycle.
func TestTarjanSCC_TwoNodeCycle(t *testing.T) {
	graph := dependencyGraph{
		"a": {"b"},
		"b": {"a"},
	}

	sccs := tarjanSCC(graph)
	require.Len(t, sccs, 1)
	assert.Len(t, sccs[0], 2, "SCC should contain both nodes")
}

// TestTarjanSCC_DAG tests Tarjan with DAG (no cycles).
func TestTarjanSCC_DAG(t *testing.T) {
	graph := dependencyGraph{
		"a": {"b", "c"},
		"b": {"c"},
		"c": {},
	}

	sccs := tarjanSCC(graph)
	// Each node is its own SCC (all singletons)
	assert.Len(t, sccs, 3)
	for _, scc := range sccs {
		assert.Len(t, scc, 1, "each SCC should be a singleton")
	}
}

// TestReconstructCyclePath_Empty tests path reconstruction with empty SCC.
func TestReconstructCyclePath_Empty(t *testing.T) {
	graph := dependencyGraph{}
	path := reconstructCyclePath([]string{}, graph)
	assert.Empty(t, path)
}

// TestReconstructCyclePath_TwoNodes tests path reconstruction with 2-node cycle.
func TestReconstructCyclePath_TwoNodes(t *testing.T) {
	graph := dependencyGraph{
		"a": {"b"},
		"b": {"a"},
	}

	path := reconstructCyclePath([]string{"a", "b"}, graph)
	assert.Len(t, path, 3) // a → b → a
	assert.Equal(t, path[0], path[2])
}
