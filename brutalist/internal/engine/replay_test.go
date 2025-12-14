package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// setupReplayTestEngine creates a test engine with store for replay tests.
func setupReplayTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)
	return e, s
}

// TestReplay_PartialReplaySkipsExistingFirings tests that replay skips already-fired bindings.
func TestReplay_PartialReplaySkipsExistingFirings(t *testing.T) {
	e, s := setupReplayTestEngine(t)
	ctx := context.Background()

	// Setup: invocation and completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{"cart_id": ir.IRString("cart-123")},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{"cart_id": ir.IRString("cart-123")},
		Seq:          101,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule that generates invocations
	sync := ir.SyncRule{
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Simulate partial execution: 2 of 3 bindings already fired
	binding1 := ir.IRObject{"item_id": ir.IRString("item-A")}
	binding2 := ir.IRObject{"item_id": ir.IRString("item-B")}
	binding3 := ir.IRObject{"item_id": ir.IRString("item-C")}

	// Fire first two bindings manually (simulate prior execution)
	err := e.executeThen(ctx, sync.Then, []ir.IRObject{binding1, binding2}, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify 2 firings exist
	firings1, err := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.NoError(t, err)
	require.Len(t, firings1, 2, "should have 2 firings from initial execution")

	// Simulate crash-restart: clear cycle detector (as if engine restarted)
	// In a real scenario, the engine would be a new instance with empty cycle detector
	e.ClearFlowCycleHistory("flow-1")

	// Now simulate replay with all 3 bindings
	allBindings := []ir.IRObject{binding1, binding2, binding3}
	err = e.executeThen(ctx, sync.Then, allBindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify only 1 new firing (binding3)
	firings2, err := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.NoError(t, err)
	require.Len(t, firings2, 3, "should have 3 total firings after replay")

	// Verify first 2 firings are unchanged
	assert.Equal(t, firings1[0].BindingHash, firings2[0].BindingHash)
	assert.Equal(t, firings1[1].BindingHash, firings2[1].BindingHash)
}

// TestReplay_FullReplayProducesIdenticalResults tests that full replay produces same results.
func TestReplay_FullReplayProducesIdenticalResults(t *testing.T) {
	e1, s1 := setupReplayTestEngine(t)
	e2, s2 := setupReplayTestEngine(t)
	ctx := context.Background()

	// Same setup in both engines
	for _, pair := range []struct {
		e *Engine
		s *store.Store
	}{{e1, s1}, {e2, s2}} {
		inv := ir.Invocation{
			ID:        "inv-1",
			FlowToken: "flow-1",
			ActionURI: "Cart.checkout",
			Args:      ir.IRObject{},
			Seq:       100,
		}
		require.NoError(t, pair.s.WriteInvocation(ctx, inv))

		comp := ir.Completion{
			ID:           "comp-1",
			InvocationID: "inv-1",
			OutputCase:   "Success",
			Result:       ir.IRObject{},
			Seq:          101,
		}
		require.NoError(t, pair.s.WriteCompletion(ctx, comp))
	}

	// Same sync rule
	sync := ir.SyncRule{
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	// Same bindings
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
		{"item_id": ir.IRString("gadget")},
	}

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          101,
	}

	// Execute on both engines
	err := e1.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)
	err = e2.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Compare results
	firings1, _ := s1.ReadSyncFiringsForCompletion(ctx, comp.ID)
	firings2, _ := s2.ReadSyncFiringsForCompletion(ctx, comp.ID)

	require.Len(t, firings1, 2)
	require.Len(t, firings2, 2)

	// Binding hashes must be identical
	for i := range firings1 {
		assert.Equal(t, firings1[i].BindingHash, firings2[i].BindingHash,
			"binding hash %d must be identical across runs", i)
	}
}

// TestReplay_MultiCrashRecovery tests idempotency across multiple replay attempts.
func TestReplay_MultiCrashRecovery(t *testing.T) {
	e, s := setupReplayTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
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
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("item-A")},
		{"item_id": ir.IRString("item-B")},
		{"item_id": ir.IRString("item-C")},
	}

	// Simulate multiple crash/replay cycles
	for i := 0; i < 10; i++ {
		// Simulate crash-restart: clear cycle detector (as if engine restarted)
		// Skip first iteration (initial execution, not replay)
		if i > 0 {
			e.ClearFlowCycleHistory("flow-1")
		}

		err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
		require.NoError(t, err, "replay %d failed", i)

		// Verify always 3 firings (no duplicates)
		firings, err := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
		require.NoError(t, err)
		require.Len(t, firings, 3, "replay %d: should always have exactly 3 firings", i)
	}
}

// TestReplay_BindingHashDeterministic tests that binding hash is deterministic.
func TestReplay_BindingHashDeterministic(t *testing.T) {
	// RFC 8785 canonical JSON ensures key ordering
	// Same keys/values → same hash regardless of Go map iteration order

	binding1 := ir.IRObject{
		"item_id":  ir.IRString("widget"),
		"quantity": ir.IRInt(5),
		"price":    ir.IRInt(100),
	}

	binding2 := ir.IRObject{
		"price":    ir.IRInt(100), // Different insertion order
		"item_id":  ir.IRString("widget"),
		"quantity": ir.IRInt(5),
	}

	hash1, err := ir.BindingHash(binding1)
	require.NoError(t, err)

	hash2, err := ir.BindingHash(binding2)
	require.NoError(t, err)

	// Hashes MUST be identical (canonical JSON sorts keys)
	assert.Equal(t, hash1, hash2, "binding hash must be deterministic regardless of key order")
}

// TestReplay_DifferentValuesProduceDifferentHashes tests hash uniqueness.
func TestReplay_DifferentValuesProduceDifferentHashes(t *testing.T) {
	binding1 := ir.IRObject{"item_id": ir.IRString("widget")}
	binding2 := ir.IRObject{"item_id": ir.IRString("gadget")}

	hash1, _ := ir.BindingHash(binding1)
	hash2, _ := ir.BindingHash(binding2)

	assert.NotEqual(t, hash1, hash2, "different values must produce different hashes")
}

// TestReplay_QueueNotDuplicatedOnReplay tests that replay doesn't re-enqueue.
func TestReplay_QueueNotDuplicatedOnReplay(t *testing.T) {
	e, s := setupReplayTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
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
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// First execution - should enqueue
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	event1, ok := e.queue.TryDequeue()
	require.True(t, ok, "first execution should enqueue")
	assert.Equal(t, EventTypeInvocation, event1.Type)

	// Simulate crash-restart: clear cycle detector (as if engine restarted)
	e.ClearFlowCycleHistory("flow-1")

	// Second execution (replay) - should NOT enqueue
	err = e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	_, ok = e.queue.TryDequeue()
	assert.False(t, ok, "replay should NOT enqueue duplicate")
}

// TestReplay_NewBindingsDuringReplayFire tests that new bindings fire on replay.
func TestReplay_NewBindingsDuringReplayFire(t *testing.T) {
	e, s := setupReplayTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
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
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	// First execution with 2 bindings
	bindings1 := []ir.IRObject{
		{"item_id": ir.IRString("item-A")},
		{"item_id": ir.IRString("item-B")},
	}
	err := e.executeThen(ctx, sync.Then, bindings1, "flow-1", comp, sync)
	require.NoError(t, err)

	firings1, _ := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.Len(t, firings1, 2)

	// Drain queue
	e.queue.TryDequeue()
	e.queue.TryDequeue()

	// Simulate crash-restart: clear cycle detector (as if engine restarted)
	e.ClearFlowCycleHistory("flow-1")

	// Replay with 3 bindings (1 new)
	bindings2 := []ir.IRObject{
		{"item_id": ir.IRString("item-A")}, // Existing
		{"item_id": ir.IRString("item-B")}, // Existing
		{"item_id": ir.IRString("item-C")}, // NEW
	}
	err = e.executeThen(ctx, sync.Then, bindings2, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify 3 firings
	firings2, _ := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.Len(t, firings2, 3, "should have 3 firings (2 old + 1 new)")

	// Verify only 1 new event enqueued
	event, ok := e.queue.TryDequeue()
	require.True(t, ok, "new binding should enqueue")
	assert.Equal(t, ir.IRString("item-C"), event.Invocation.Args["item"])

	_, ok = e.queue.TryDequeue()
	assert.False(t, ok, "should only have 1 new enqueue")
}

// TestReplay_ZeroExistingFiringsBehavesNormally tests fresh execution.
func TestReplay_ZeroExistingFiringsBehavesNormally(t *testing.T) {
	e, s := setupReplayTestEngine(t)
	ctx := context.Background()

	// Setup
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
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
		ID: "reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"item": "bound.item_id"},
		},
	}

	bindings := []ir.IRObject{
		{"item_id": ir.IRString("item-A")},
		{"item_id": ir.IRString("item-B")},
	}

	// Execute on empty DB - should work like normal execution
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify 2 firings created
	firings, _ := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.Len(t, firings, 2)

	// Verify 2 events enqueued
	event1, ok1 := e.queue.TryDequeue()
	assert.True(t, ok1)
	event2, ok2 := e.queue.TryDequeue()
	assert.True(t, ok2)
	_, ok3 := e.queue.TryDequeue()
	assert.False(t, ok3)

	// Verify invocations
	assert.Equal(t, ir.ActionRef("Inventory.reserve"), event1.Invocation.ActionURI)
	assert.Equal(t, ir.ActionRef("Inventory.reserve"), event2.Invocation.ActionURI)
}
