package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// =============================================================================
// CycleDetector Unit Tests
// =============================================================================

func TestCycleDetector_NewCycleDetector(t *testing.T) {
	cd := NewCycleDetector()
	require.NotNil(t, cd)
	assert.Equal(t, 0, cd.HistorySize())
}

func TestCycleDetector_WouldCycle_FirstOccurrence(t *testing.T) {
	cd := NewCycleDetector()

	// First occurrence should not be a cycle
	result := cd.WouldCycle("flow-1", "sync-reserve", "hash-abc")
	assert.False(t, result, "first occurrence should not be a cycle")
}

func TestCycleDetector_WouldCycle_AfterRecord(t *testing.T) {
	cd := NewCycleDetector()

	// Record a firing
	cd.Record("flow-1", "sync-reserve", "hash-abc")

	// Same (sync, binding) should now be detected as cycle
	result := cd.WouldCycle("flow-1", "sync-reserve", "hash-abc")
	assert.True(t, result, "same (sync, binding) after record should be a cycle")
}

func TestCycleDetector_WouldCycle_DifferentFlows(t *testing.T) {
	cd := NewCycleDetector()

	// Record in flow-1
	cd.Record("flow-1", "sync-reserve", "hash-abc")

	// Same (sync, binding) in different flow should NOT be a cycle
	result := cd.WouldCycle("flow-2", "sync-reserve", "hash-abc")
	assert.False(t, result, "same (sync, binding) in different flow should not be a cycle")
}

func TestCycleDetector_WouldCycle_DifferentSyncs(t *testing.T) {
	cd := NewCycleDetector()

	// Record sync-reserve
	cd.Record("flow-1", "sync-reserve", "hash-abc")

	// Different sync with same binding should NOT be a cycle
	result := cd.WouldCycle("flow-1", "sync-ship", "hash-abc")
	assert.False(t, result, "different sync with same binding should not be a cycle")
}

func TestCycleDetector_WouldCycle_DifferentBindings(t *testing.T) {
	cd := NewCycleDetector()

	// Record hash-abc
	cd.Record("flow-1", "sync-reserve", "hash-abc")

	// Same sync with different binding should NOT be a cycle
	result := cd.WouldCycle("flow-1", "sync-reserve", "hash-xyz")
	assert.False(t, result, "same sync with different binding should not be a cycle")
}

func TestCycleDetector_Clear(t *testing.T) {
	cd := NewCycleDetector()

	// Record in flow-1
	cd.Record("flow-1", "sync-reserve", "hash-abc")
	assert.True(t, cd.WouldCycle("flow-1", "sync-reserve", "hash-abc"))

	// Clear flow-1
	cd.Clear("flow-1")

	// Should no longer be a cycle
	assert.False(t, cd.WouldCycle("flow-1", "sync-reserve", "hash-abc"),
		"after clear, same (sync, binding) should not be a cycle")
}

func TestCycleDetector_Clear_DoesNotAffectOtherFlows(t *testing.T) {
	cd := NewCycleDetector()

	// Record in both flows
	cd.Record("flow-1", "sync-reserve", "hash-abc")
	cd.Record("flow-2", "sync-reserve", "hash-abc")

	// Clear only flow-1
	cd.Clear("flow-1")

	// flow-1 should be cleared
	assert.False(t, cd.WouldCycle("flow-1", "sync-reserve", "hash-abc"))

	// flow-2 should still have history
	assert.True(t, cd.WouldCycle("flow-2", "sync-reserve", "hash-abc"))
}

func TestCycleDetector_HistorySize(t *testing.T) {
	cd := NewCycleDetector()

	assert.Equal(t, 0, cd.HistorySize())

	cd.Record("flow-1", "sync-a", "hash-1")
	assert.Equal(t, 1, cd.HistorySize())

	// Same flow, different key
	cd.Record("flow-1", "sync-b", "hash-2")
	assert.Equal(t, 1, cd.HistorySize(), "same flow should not increase history size")

	// Different flow
	cd.Record("flow-2", "sync-a", "hash-1")
	assert.Equal(t, 2, cd.HistorySize())

	// Clear one flow
	cd.Clear("flow-1")
	assert.Equal(t, 1, cd.HistorySize())
}

func TestCycleDetector_FlowHistorySize(t *testing.T) {
	cd := NewCycleDetector()

	// Empty flow
	assert.Equal(t, 0, cd.FlowHistorySize("flow-1"))

	// Add entries
	cd.Record("flow-1", "sync-a", "hash-1")
	assert.Equal(t, 1, cd.FlowHistorySize("flow-1"))

	cd.Record("flow-1", "sync-b", "hash-2")
	assert.Equal(t, 2, cd.FlowHistorySize("flow-1"))

	// Different flow should have its own count
	cd.Record("flow-2", "sync-a", "hash-1")
	assert.Equal(t, 2, cd.FlowHistorySize("flow-1"))
	assert.Equal(t, 1, cd.FlowHistorySize("flow-2"))
}

// =============================================================================
// RuntimeError Tests
// =============================================================================

func TestNewCycleError(t *testing.T) {
	err := NewCycleError("flow-123", "sync-reserve", "hash-abc")

	assert.Equal(t, ErrCodeCycleDetected, err.Code)
	assert.Equal(t, "flow-123", err.FlowToken)
	assert.Equal(t, "sync-reserve", err.SyncID)
	assert.Equal(t, "hash-abc", err.BindingHash)
	assert.Contains(t, err.Error(), "CYCLE_DETECTED")
	assert.Contains(t, err.Error(), "flow-123")
	assert.Contains(t, err.Error(), "sync-reserve")
}

func TestIsCycleError(t *testing.T) {
	cycleErr := NewCycleError("flow-1", "sync-1", "hash-1")
	quotaErr := NewQuotaError("flow-1", 100, 50)

	assert.True(t, IsCycleError(cycleErr))
	assert.False(t, IsCycleError(quotaErr))
	assert.False(t, IsCycleError(nil))
	assert.False(t, IsCycleError(assert.AnError))
}

func TestIsQuotaError(t *testing.T) {
	cycleErr := NewCycleError("flow-1", "sync-1", "hash-1")
	quotaErr := NewQuotaError("flow-1", 100, 50)

	assert.False(t, IsQuotaError(cycleErr))
	assert.True(t, IsQuotaError(quotaErr))
	assert.False(t, IsQuotaError(nil))
	assert.False(t, IsQuotaError(assert.AnError))
}

// =============================================================================
// Integration Tests with executeThen
// =============================================================================

func setupCycleTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)
	return e, s
}

func TestCycle_ExecuteThen_FirstFiringSucceeds(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup invocation and completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// First firing should succeed
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Should have recorded in cycle detector
	assert.Equal(t, 1, e.cycleDetector.FlowHistorySize("flow-1"))
}

func TestCycle_ExecuteThen_CycleDetected(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup invocation and completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// First firing succeeds
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Drain queue
	e.queue.TryDequeue()

	// Second firing with SAME (sync, binding) in SAME flow should fail
	err = e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.Error(t, err)
	assert.True(t, IsCycleError(err), "should be a cycle error")

	cycleErr := err.(*RuntimeError)
	assert.Equal(t, "flow-1", cycleErr.FlowToken)
	assert.Equal(t, "sync-reserve", cycleErr.SyncID)
}

func TestCycle_ExecuteThen_DifferentFlowsIndependent(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup for flow-1
	inv1 := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv1))

	comp1 := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp1))

	// Setup for flow-2
	inv2 := ir.Invocation{
		ID:        "inv-2",
		FlowToken: "flow-2",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       200,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv2))

	comp2 := ir.Completion{
		ID:           "comp-2",
		InvocationID: "inv-2",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          201,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp2))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// Fire in flow-1
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp1, sync)
	require.NoError(t, err)

	// Same (sync, binding) in flow-2 should succeed (different flow)
	err = e.executeThen(ctx, sync.Then, bindings, "flow-2", comp2, sync)
	require.NoError(t, err)

	// Both flows should have independent history
	assert.Equal(t, 1, e.cycleDetector.FlowHistorySize("flow-1"))
	assert.Equal(t, 1, e.cycleDetector.FlowHistorySize("flow-2"))
}

func TestCycle_ExecuteThen_DifferentBindingsSameFlowOK(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	// Fire with binding A
	bindingsA := []ir.IRObject{{"item_id": ir.IRString("widget-A")}}
	err := e.executeThen(ctx, sync.Then, bindingsA, "flow-1", comp, sync)
	require.NoError(t, err)

	// Fire with binding B (same sync, different binding) - should succeed
	bindingsB := []ir.IRObject{{"item_id": ir.IRString("widget-B")}}
	err = e.executeThen(ctx, sync.Then, bindingsB, "flow-1", comp, sync)
	require.NoError(t, err)

	// Both should be recorded
	assert.Equal(t, 2, e.cycleDetector.FlowHistorySize("flow-1"))
}

func TestCycle_ExecuteThen_ClearAfterFlowCompletes(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{{"item_id": ir.IRString("widget")}}

	// Fire once
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)
	assert.Equal(t, 1, e.cycleDetector.FlowHistorySize("flow-1"))

	// Simulate flow completion - clear history
	e.ClearFlowCycleHistory("flow-1")
	assert.Equal(t, 0, e.cycleDetector.FlowHistorySize("flow-1"))
	assert.Equal(t, 0, e.cycleDetector.HistorySize())
}

func TestCycle_ExecuteThen_MultipleBindingsCycleOnSecond(t *testing.T) {
	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	// First: fire binding A
	bindingsA := []ir.IRObject{{"item_id": ir.IRString("widget-A")}}
	err := e.executeThen(ctx, sync.Then, bindingsA, "flow-1", comp, sync)
	require.NoError(t, err)

	// Second call with both A and B - should fail on A (cycle)
	bindingsAB := []ir.IRObject{
		{"item_id": ir.IRString("widget-A")}, // Will cycle
		{"item_id": ir.IRString("widget-B")}, // Won't be reached
	}
	err = e.executeThen(ctx, sync.Then, bindingsAB, "flow-1", comp, sync)
	require.Error(t, err)
	assert.True(t, IsCycleError(err))

	// Only A should be in history (B wasn't processed due to cycle error)
	assert.Equal(t, 1, e.cycleDetector.FlowHistorySize("flow-1"))
}

// =============================================================================
// Scenario Tests
// =============================================================================

func TestCycle_Scenario_SelfReferentialSync(t *testing.T) {
	// Scenario: A sync rule that triggers itself
	// Order.Create → sync-create-order → Order.Create (cycle!)

	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup original invocation
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{"order_id": ir.IRString("order-123")},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{"order_id": ir.IRString("order-123")},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Self-referential sync: Order.Create triggers Order.Create
	selfSync := ir.SyncRule{
		ID: "sync-create-order",
		Then: ir.ThenClause{
			ActionRef: "Order.Create",
			Args:      map[string]string{"order_id": "bound.order_id"},
		},
	}

	bindings := []ir.IRObject{{"order_id": ir.IRString("order-123")}}

	// First firing succeeds
	err := e.executeThen(ctx, selfSync.Then, bindings, "flow-1", comp, selfSync)
	require.NoError(t, err)

	// Drain the generated invocation
	e.queue.TryDequeue()

	// Second firing with same binding would create infinite loop - should fail
	err = e.executeThen(ctx, selfSync.Then, bindings, "flow-1", comp, selfSync)
	require.Error(t, err)
	assert.True(t, IsCycleError(err))
}

func TestCycle_Scenario_MutuallyRecursive(t *testing.T) {
	// Scenario: Two syncs that trigger each other
	// Order.Create → sync-A → Inventory.Reserve
	// Inventory.Reserve → sync-B → Order.Create (same binding = cycle!)

	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup for sync-A
	inv1 := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Order.Create",
		Args:      ir.IRObject{"item_id": ir.IRString("widget")},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv1))

	comp1 := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{"item_id": ir.IRString("widget")},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp1))

	syncA := ir.SyncRule{
		ID: "sync-A",
		Then: ir.ThenClause{
			ActionRef: "Inventory.Reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	syncB := ir.SyncRule{
		ID: "sync-B",
		Then: ir.ThenClause{
			ActionRef: "Order.Create",
			Args:      map[string]string{"item_id": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{{"item_id": ir.IRString("widget")}}

	// Fire sync-A
	err := e.executeThen(ctx, syncA.Then, bindings, "flow-1", comp1, syncA)
	require.NoError(t, err)

	// Simulate Inventory.Reserve completion
	inv2 := ir.Invocation{
		ID:        "inv-2",
		FlowToken: "flow-1",
		ActionURI: "Inventory.Reserve",
		Args:      ir.IRObject{"item": ir.IRString("widget")},
		Seq:       102,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv2))

	comp2 := ir.Completion{
		ID:           "comp-2",
		InvocationID: "inv-2",
		OutputCase:   "Success",
		Result:       ir.IRObject{"item_id": ir.IRString("widget")},
		Seq:          103,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp2))

	// Fire sync-B (same binding, same flow = would cycle back to sync-A eventually)
	// But sync-B is different sync ID, so this should succeed
	err = e.executeThen(ctx, syncB.Then, bindings, "flow-1", comp2, syncB)
	require.NoError(t, err)

	// Now if sync-A fires again with same binding, it should be detected as cycle
	err = e.executeThen(ctx, syncA.Then, bindings, "flow-1", comp1, syncA)
	require.Error(t, err)
	assert.True(t, IsCycleError(err))
}

func TestCycle_Scenario_DistinctFlowsConcurrent(t *testing.T) {
	// Scenario: Two flows execute the same sync concurrently
	// Neither should interfere with the other

	e, s := setupCycleTestEngine(t)
	ctx := context.Background()

	// Setup flow-1
	inv1 := ir.Invocation{ID: "inv-1", FlowToken: "flow-1", ActionURI: "A", Args: ir.IRObject{}, Seq: 100}
	comp1 := ir.Completion{ID: "comp-1", InvocationID: "inv-1", OutputCase: "Success", Result: ir.IRObject{}, Seq: 101}
	require.NoError(t, s.WriteInvocation(ctx, inv1))
	require.NoError(t, s.WriteCompletion(ctx, comp1))

	// Setup flow-2
	inv2 := ir.Invocation{ID: "inv-2", FlowToken: "flow-2", ActionURI: "A", Args: ir.IRObject{}, Seq: 200}
	comp2 := ir.Completion{ID: "comp-2", InvocationID: "inv-2", OutputCase: "Success", Result: ir.IRObject{}, Seq: 201}
	require.NoError(t, s.WriteInvocation(ctx, inv2))
	require.NoError(t, s.WriteCompletion(ctx, comp2))

	sync := ir.SyncRule{
		ID:   "sync-x",
		Then: ir.ThenClause{ActionRef: "B", Args: map[string]string{"key": "bound.val"}},
	}

	bindings := []ir.IRObject{{"val": ir.IRString("same-value")}}

	// Fire in both flows - both should succeed
	err1 := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp1, sync)
	require.NoError(t, err1)

	err2 := e.executeThen(ctx, sync.Then, bindings, "flow-2", comp2, sync)
	require.NoError(t, err2)

	// Both have their own history
	assert.Equal(t, 2, e.cycleDetector.HistorySize())

	// Fire again in flow-1 - should cycle
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp1, sync)
	require.Error(t, err)
	assert.True(t, IsCycleError(err))

	// Fire again in flow-2 - should also cycle (independent)
	err = e.executeThen(ctx, sync.Then, bindings, "flow-2", comp2, sync)
	require.Error(t, err)
	assert.True(t, IsCycleError(err))
}
