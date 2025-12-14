package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// setupExecutorTestEngine creates a test engine with a real SQLite store.
func setupExecutorTestEngine(t *testing.T) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)
	return e, s
}

// TestResolveArgs_BoundVariableSubstitution tests bound.varName substitution.
func TestResolveArgs_BoundVariableSubstitution(t *testing.T) {
	argTemplates := map[string]string{
		"product":  "bound.item_id",
		"quantity": "bound.qty",
	}

	bindings := ir.IRObject{
		"item_id": ir.IRString("widget-x"),
		"qty":     ir.IRInt(10),
	}

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.NoError(t, err)

	expected := ir.IRObject{
		"product":  ir.IRString("widget-x"),
		"quantity": ir.IRInt(10),
	}

	assert.Equal(t, expected, resolvedArgs)
}

// TestResolveArgs_LiteralPassthrough tests literal values pass through unchanged.
func TestResolveArgs_LiteralPassthrough(t *testing.T) {
	argTemplates := map[string]string{
		"product":  "bound.item_id",
		"priority": "high", // Literal value
		"enabled":  "true", // Literal (not a boolean)
	}

	bindings := ir.IRObject{
		"item_id": ir.IRString("widget-x"),
	}

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.NoError(t, err)

	assert.Equal(t, ir.IRString("widget-x"), resolvedArgs["product"])
	assert.Equal(t, ir.IRString("high"), resolvedArgs["priority"])
	assert.Equal(t, ir.IRString("true"), resolvedArgs["enabled"])
}

// TestResolveArgs_MissingVariable tests error on missing binding variable.
func TestResolveArgs_MissingVariable(t *testing.T) {
	argTemplates := map[string]string{
		"product": "bound.item_id",
		"qty":     "bound.quantity", // Not in bindings
	}

	bindings := ir.IRObject{
		"item_id": ir.IRString("widget-x"),
		// "quantity" missing!
	}

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.Error(t, err)
	assert.Nil(t, resolvedArgs)
	assert.Contains(t, err.Error(), "quantity")
	assert.Contains(t, err.Error(), "not found")
}

// TestResolveArgs_PreservesTypes tests that IRValue types are preserved.
func TestResolveArgs_PreservesTypes(t *testing.T) {
	argTemplates := map[string]string{
		"str":  "bound.str_val",
		"int":  "bound.int_val",
		"bool": "bound.bool_val",
		"obj":  "bound.obj_val",
		"arr":  "bound.arr_val",
	}

	bindings := ir.IRObject{
		"str_val":  ir.IRString("hello"),
		"int_val":  ir.IRInt(123),
		"bool_val": ir.IRBool(true),
		"obj_val":  ir.IRObject{"nested": ir.IRString("value")},
		"arr_val":  ir.IRArray{ir.IRInt(1), ir.IRInt(2)},
	}

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.NoError(t, err)

	// Verify types preserved
	assert.IsType(t, ir.IRString(""), resolvedArgs["str"])
	assert.IsType(t, ir.IRInt(0), resolvedArgs["int"])
	assert.IsType(t, ir.IRBool(false), resolvedArgs["bool"])
	assert.IsType(t, ir.IRObject{}, resolvedArgs["obj"])
	assert.IsType(t, ir.IRArray{}, resolvedArgs["arr"])

	// Verify values
	assert.Equal(t, ir.IRString("hello"), resolvedArgs["str"])
	assert.Equal(t, ir.IRInt(123), resolvedArgs["int"])
	assert.Equal(t, ir.IRBool(true), resolvedArgs["bool"])
}

// TestResolveArgs_EmptyTemplates tests empty templates return empty result.
func TestResolveArgs_EmptyTemplates(t *testing.T) {
	argTemplates := map[string]string{}
	bindings := ir.IRObject{"item_id": ir.IRString("widget-x")}

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.NoError(t, err)
	assert.NotNil(t, resolvedArgs)
	assert.Empty(t, resolvedArgs)
}

// TestResolveArgs_NilBindings tests error when bindings are nil and bound var referenced.
func TestResolveArgs_NilBindings(t *testing.T) {
	argTemplates := map[string]string{
		"product": "bound.item_id",
	}

	resolvedArgs, err := resolveArgs(argTemplates, nil)
	require.Error(t, err)
	assert.Nil(t, resolvedArgs)
	assert.Contains(t, err.Error(), "item_id")
}

// TestResolveArgs_AllLiterals tests when all args are literals.
func TestResolveArgs_AllLiterals(t *testing.T) {
	argTemplates := map[string]string{
		"mode":     "async",
		"timeout":  "30",
		"template": "order_confirmation",
	}

	bindings := ir.IRObject{} // Empty bindings

	resolvedArgs, err := resolveArgs(argTemplates, bindings)
	require.NoError(t, err)

	assert.Equal(t, ir.IRString("async"), resolvedArgs["mode"])
	assert.Equal(t, ir.IRString("30"), resolvedArgs["timeout"])
	assert.Equal(t, ir.IRString("order_confirmation"), resolvedArgs["template"])
}

// TestExecuteThen_EmptyBindings tests that empty bindings generates no invocations.
func TestExecuteThen_EmptyBindings(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{},
		},
	}

	// Empty bindings
	bindings := []ir.IRObject{}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err, "empty bindings should not error")

	// Verify no new invocations created (only original)
	invs, _, err := s.ReadFlow(ctx, "flow-1")
	require.NoError(t, err)
	require.Len(t, invs, 1, "should only have original invocation")
}

// TestExecuteThen_SingleBinding tests single binding generates one invocation.
func TestExecuteThen_SingleBinding(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion - use high seq so generated invocations are distinguishable
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       100, // High seq so generated invocations (seq=1,2,...) come first
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

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"cart": "bound.cart_id",
			},
		},
	}

	// Single binding
	bindings := []ir.IRObject{
		{"cart_id": ir.IRString("cart-123")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify invocation created
	invs, _, err := s.ReadFlow(ctx, "flow-1")
	require.NoError(t, err)
	require.Len(t, invs, 2, "should have original + generated invocation")

	// Find generated invocation by ActionURI (ordering may vary)
	var generated *ir.Invocation
	for i := range invs {
		if invs[i].ActionURI == "Inventory.reserve" {
			generated = &invs[i]
			break
		}
	}
	require.NotNil(t, generated, "should find generated invocation")
	assert.Equal(t, ir.IRString("cart-123"), generated.Args["cart"])
	assert.Equal(t, "flow-1", generated.FlowToken)
}

// TestExecuteThen_MultipleBindings tests multiple bindings generate multiple invocations.
func TestExecuteThen_MultipleBindings(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion - use high seq so generated invocations are distinguishable
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

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-reserve-items",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
				"qty":  "bound.quantity",
			},
		},
	}

	// Three bindings (three cart items)
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget"), "quantity": ir.IRInt(10)},
		{"item_id": ir.IRString("gadget"), "quantity": ir.IRInt(5)},
		{"item_id": ir.IRString("doohickey"), "quantity": ir.IRInt(7)},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify 3 invocations created
	invs, _, err := s.ReadFlow(ctx, "flow-1")
	require.NoError(t, err)
	require.Len(t, invs, 4, "should have original + 3 generated invocations")

	// Collect generated invocations by ActionURI
	var generated []ir.Invocation
	for _, inv := range invs {
		if inv.ActionURI == "Inventory.reserve" {
			generated = append(generated, inv)
		}
	}
	require.Len(t, generated, 3, "should have 3 generated invocations")

	// Verify all expected items are present (order may vary due to content-addressed IDs)
	itemFound := map[string]bool{"widget": false, "gadget": false, "doohickey": false}
	for _, g := range generated {
		itemStr, ok := g.Args["item"].(ir.IRString)
		require.True(t, ok)
		itemFound[string(itemStr)] = true
	}
	for item, found := range itemFound {
		assert.True(t, found, "should find item: %s", item)
	}
}

// TestExecuteThen_IdempotencySkip tests that already-fired bindings are skipped.
func TestExecuteThen_IdempotencySkip(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Three bindings
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
		{"item_id": ir.IRString("gadget")},
		{"item_id": ir.IRString("doohickey")},
	}

	// First execution - all bindings fire
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify 3 invocations created
	invs1, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs1, 4, "should have original + 3 generated")

	// Simulate crash-restart: clear cycle detector (as if engine restarted)
	e.ClearFlowCycleHistory("flow-1")

	// Second execution (replay scenario) - all bindings should be skipped
	err = e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify no new invocations created
	invs2, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs2, 4, "should still have only 4 invocations")

	// Verify invocation IDs unchanged (exact same invocations)
	for i, inv1 := range invs1 {
		assert.Equal(t, inv1.ID, invs2[i].ID, "invocation IDs must match")
	}
}

// TestExecuteThen_PartialIdempotency tests partial replay (some bindings fired, some not).
func TestExecuteThen_PartialIdempotency(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// First execution with 2 bindings
	bindings1 := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
		{"item_id": ir.IRString("gadget")},
	}
	err := e.executeThen(ctx, sync.Then, bindings1, "flow-1", comp, sync)
	require.NoError(t, err)

	invs1, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs1, 3, "should have original + 2 generated")

	// Simulate crash-restart: clear cycle detector (as if engine restarted)
	e.ClearFlowCycleHistory("flow-1")

	// Second execution with 3 bindings (first 2 already fired, third is new)
	bindings2 := []ir.IRObject{
		{"item_id": ir.IRString("widget")},    // Already fired
		{"item_id": ir.IRString("gadget")},    // Already fired
		{"item_id": ir.IRString("doohickey")}, // New
	}
	err = e.executeThen(ctx, sync.Then, bindings2, "flow-1", comp, sync)
	require.NoError(t, err)

	invs2, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs2, 4, "should have original + 3 generated (1 new)")
}

// TestExecuteThen_SyncFiringCreated tests that sync firing records are created.
func TestExecuteThen_SyncFiringCreated(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Single binding
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify firing was recorded via HasFiring
	bindingHash := ir.MustBindingHash(bindings[0])
	hasFired, err := s.HasFiring(ctx, comp.ID, sync.ID, bindingHash)
	require.NoError(t, err)
	assert.True(t, hasFired, "sync firing should be recorded")
}

// TestExecuteThen_ProvenanceCreated tests that provenance edges are created.
func TestExecuteThen_ProvenanceCreated(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion - use high seq so generated invocations are distinguishable
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

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-reserve",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Single binding
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Find the generated invocation by ActionURI
	invs, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs, 2)
	var generated *ir.Invocation
	for i := range invs {
		if invs[i].ActionURI == "Inventory.reserve" {
			generated = &invs[i]
			break
		}
	}
	require.NotNil(t, generated, "should find generated invocation")

	// Check provenance exists
	prov, err := s.ReadProvenance(ctx, generated.ID)
	require.NoError(t, err)
	require.Len(t, prov, 1, "should have one provenance record")
	assert.NotZero(t, prov[0].SyncFiringID)
	assert.Equal(t, generated.ID, prov[0].InvocationID)
}

// TestExecuteThen_InvocationEnqueued tests that invocations are enqueued.
func TestExecuteThen_InvocationEnqueued(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Two bindings
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
		{"item_id": ir.IRString("gadget")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Check queue has events (use TryDequeue to verify)
	event1, ok1 := e.queue.TryDequeue()
	require.True(t, ok1, "should have first queued event")
	assert.Equal(t, EventTypeInvocation, event1.Type)
	assert.NotNil(t, event1.Invocation)
	assert.Equal(t, ir.ActionRef("Inventory.reserve"), event1.Invocation.ActionURI)

	event2, ok2 := e.queue.TryDequeue()
	require.True(t, ok2, "should have second queued event")
	assert.Equal(t, EventTypeInvocation, event2.Type)
}

// TestExecuteThen_ContextCancellation tests context cancellation is respected.
func TestExecuteThen_ContextCancellation(t *testing.T) {
	e, s := setupExecutorTestEngine(t)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	s.WriteInvocation(context.Background(), inv)

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	s.WriteCompletion(context.Background(), comp)

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{},
		},
	}

	// Bindings
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// Execute with cancelled context
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context cancelled")
}

// TestExecuteThen_ResolveArgsError tests error propagation from resolveArgs.
func TestExecuteThen_ResolveArgsError(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule with missing binding reference
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.missing_var", // Not in bindings
			},
		},
	}

	// Binding without the referenced var
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")}, // Missing "missing_var"
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve args")
	assert.Contains(t, err.Error(), "missing_var")
}

// TestExecuteThen_SecurityContextPropagated tests security context propagation.
func TestExecuteThen_SecurityContextPropagated(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion with security context - use high seq
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
		SecurityContext: ir.SecurityContext{
			TenantID:    "tenant-123",
			UserID:      "user-alice",
			Permissions: []string{"cart:write", "inventory:read"},
		},
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{},
		},
	}

	// Single binding
	bindings := []ir.IRObject{{}}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Find generated invocation by ActionURI
	invs, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs, 2)
	var generated *ir.Invocation
	for i := range invs {
		if invs[i].ActionURI == "Inventory.reserve" {
			generated = &invs[i]
			break
		}
	}
	require.NotNil(t, generated, "should find generated invocation")

	// Verify security context propagated
	assert.Equal(t, "tenant-123", generated.SecurityContext.TenantID)
	assert.Equal(t, "user-alice", generated.SecurityContext.UserID)
	assert.Equal(t, []string{"cart:write", "inventory:read"}, generated.SecurityContext.Permissions)
}

// TestExecuteThen_ContentAddressedID tests invocation IDs are content-addressed.
func TestExecuteThen_ContentAddressedID(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion - use high seq
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

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Single binding
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Find generated invocation by ActionURI
	invs, _, _ := s.ReadFlow(ctx, "flow-1")
	require.Len(t, invs, 2)
	var generated *ir.Invocation
	for i := range invs {
		if invs[i].ActionURI == "Inventory.reserve" {
			generated = &invs[i]
			break
		}
	}
	require.NotNil(t, generated, "should find generated invocation")

	// Verify ID is content-addressed (non-empty hash)
	assert.NotEmpty(t, generated.ID)
	assert.Len(t, generated.ID, 64, "SHA-256 hex should be 64 chars")
}

// TestExecuteThen_UniqueBindingHashes tests that different bindings produce different hashes.
func TestExecuteThen_UniqueBindingHashes(t *testing.T) {
	e, s := setupExecutorTestEngine(t)
	ctx := context.Background()

	// Setup completion
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
		Seq:       1,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))

	// Sync rule
	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"item": "bound.item_id",
			},
		},
	}

	// Three different bindings
	bindings := []ir.IRObject{
		{"item_id": ir.IRString("widget")},
		{"item_id": ir.IRString("gadget")},
		{"item_id": ir.IRString("doohickey")},
	}

	// Execute then-clause
	err := e.executeThen(ctx, sync.Then, bindings, "flow-1", comp, sync)
	require.NoError(t, err)

	// Verify all firing records have unique binding hashes
	firings, err := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
	require.NoError(t, err)
	require.Len(t, firings, 3)

	hashes := make(map[string]bool)
	for _, f := range firings {
		hashes[f.BindingHash] = true
	}
	assert.Len(t, hashes, 3, "all binding hashes must be unique")
}
