package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

// Story 3.6: Flow Token Propagation tests
// These tests verify that flow tokens are properly inherited throughout the
// sync chain, never generated mid-flow.

func TestGenerateInvocation_FlowTokenPropagation(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-test-123"
	then := ir.ThenClause{
		ActionRef: "Inventory.ReserveStock",
		Args: map[string]string{
			"product_id": "${bound.product}",
			"quantity":   "5",
		},
	}
	bindings := ir.IRObject{
		"product": ir.IRString("widget"),
	}

	// Generate invocation
	inv, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	// Verify flow token inherited
	assert.Equal(t, flowToken, inv.FlowToken, "flow token must be inherited from completion")
	assert.Equal(t, ir.ActionRef("Inventory.ReserveStock"), inv.ActionURI)
	assert.Equal(t, ir.IRString("widget"), inv.Args["product_id"])
	assert.Equal(t, ir.IRString("5"), inv.Args["quantity"])
}

func TestGenerateInvocation_MissingFlowToken(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	then := ir.ThenClause{
		ActionRef: "Test.Action",
		Args:      map[string]string{},
	}
	bindings := ir.IRObject{}

	// Attempt to generate invocation with empty flow token
	_, err := engine.generateInvocation("", then, bindings)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flow token is required")
}

func TestGenerateInvocation_ArgResolution(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-args-test"
	then := ir.ThenClause{
		ActionRef: "Order.Process",
		Args: map[string]string{
			"order_id": "${bound.order_id}",
			"product":  "${bound.product_name}",
			"quantity": "10",            // Not a binding reference
			"priority": "high",          // Not a binding reference
		},
	}
	bindings := ir.IRObject{
		"order_id":     ir.IRString("ord-123"),
		"product_name": ir.IRString("widget"),
	}

	inv, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	// Verify args resolved correctly
	assert.Equal(t, ir.IRString("ord-123"), inv.Args["order_id"],
		"order_id should be resolved from bindings")
	assert.Equal(t, ir.IRString("widget"), inv.Args["product"],
		"product should be resolved from bindings")
	assert.Equal(t, ir.IRString("10"), inv.Args["quantity"],
		"quantity should pass through (not a binding reference)")
	assert.Equal(t, ir.IRString("high"), inv.Args["priority"],
		"priority should pass through (not a binding reference)")
}

func TestGenerateInvocation_MissingBinding(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-missing-binding"
	then := ir.ThenClause{
		ActionRef: "Test.Action",
		Args: map[string]string{
			"field": "${bound.nonexistent}",
		},
	}
	bindings := ir.IRObject{
		// "nonexistent" binding not provided
	}

	_, err := engine.generateInvocation(flowToken, then, bindings)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "binding \"nonexistent\" not found")
}

func TestFlowToken_NeverGeneratedMidFlow(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	// The key insight: flow token is passed as a parameter to generateInvocation,
	// NOT generated from the flow generator. This test verifies that pattern.

	originalFlow := "flow-original-123"
	then := ir.ThenClause{
		ActionRef: "NextAction",
		Args:      map[string]string{},
	}
	bindings := ir.IRObject{}

	// Generate invocation - must use provided flow token
	inv, err := engine.generateInvocation(originalFlow, then, bindings)
	require.NoError(t, err)

	// CRITICAL: Flow token MUST match the provided parameter
	assert.Equal(t, originalFlow, inv.FlowToken,
		"flow token must be inherited, never generated")
}

func TestFireSyncRule_FlowTokenPropagation(t *testing.T) {
	s := setupTestStore(t)
	flowGen := NewFixedGenerator("flow-propagation-test")
	engine := New(s, nil, nil, flowGen)

	ctx := context.Background()
	flowToken := "flow-abc-123"

	// Create root invocation with flow token
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID(flowToken, "Cart.checkout", args, 1)
	inv := ir.Invocation{
		ID:        invID,
		FlowToken: flowToken,
		ActionURI: "Cart.checkout",
		Args:      args,
		Seq:       1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
		SpecHash:      "spec-hash-1",
		EngineVersion: ir.EngineVersion,
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	// Create completion for the invocation
	result := ir.IRObject{"order_id": ir.IRString("order-456")}
	compID := ir.MustCompletionID(invID, "Success", result, 2)
	comp := &ir.Completion{
		ID:           compID,
		InvocationID: invID,
		OutputCase:   "Success",
		Result:       result,
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
	require.NoError(t, s.WriteCompletion(ctx, *comp))

	// Sync rule to fire
	sync := ir.SyncRule{
		ID: "sync-process-order",
		When: ir.WhenClause{
			ActionRef:  "Cart.checkout",
			EventType:  "completed",
			OutputCase: "Success",
			Bindings: map[string]string{
				"order_id": "order_id",
			},
		},
		Then: ir.ThenClause{
			ActionRef: "Order.Process",
			Args: map[string]string{
				"order_id": "${bound.order_id}",
			},
		},
	}

	bindings := ir.IRObject{
		"order_id": ir.IRString("order-456"),
	}

	// Fire sync rule with inherited flow token
	err := engine.fireSyncRule(ctx, sync, comp, flowToken, bindings)
	require.NoError(t, err)

	// Verify sync firing recorded
	firings, err := s.ReadSyncFiringsForCompletion(ctx, compID)
	require.NoError(t, err)
	require.Len(t, firings, 1)

	// Verify provenance edge created
	edges, err := s.ReadAllProvenanceEdges(ctx)
	require.NoError(t, err)
	require.Len(t, edges, 1)

	// Verify generated invocation has same flow token
	generatedInv, err := s.ReadInvocation(ctx, edges[0].InvocationID)
	require.NoError(t, err)
	assert.Equal(t, flowToken, generatedInv.FlowToken,
		"generated invocation must have same flow token as triggering completion")
	assert.Equal(t, ir.ActionRef("Order.Process"), generatedInv.ActionURI)
}

func TestFireSyncRule_Idempotency(t *testing.T) {
	s := setupTestStore(t)
	flowGen := NewFixedGenerator("flow-idempotency-test")
	engine := New(s, nil, nil, flowGen)

	ctx := context.Background()
	flowToken := "flow-idem-123"

	// Setup invocation and completion
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID(flowToken, "Test.action", args, 1)
	inv := ir.Invocation{
		ID:        invID,
		FlowToken: flowToken,
		ActionURI: "Test.action",
		Args:      args,
		Seq:       1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
		SpecHash:      "spec-hash-1",
		EngineVersion: ir.EngineVersion,
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	result := ir.IRObject{"result": ir.IRString("success")}
	compID := ir.MustCompletionID(invID, "Success", result, 2)
	comp := &ir.Completion{
		ID:           compID,
		InvocationID: invID,
		OutputCase:   "Success",
		Result:       result,
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
	require.NoError(t, s.WriteCompletion(ctx, *comp))

	sync := ir.SyncRule{
		ID: "sync-test",
		Then: ir.ThenClause{
			ActionRef: "Next.Action",
			Args:      map[string]string{},
		},
	}
	bindings := ir.IRObject{}

	// Fire sync rule first time
	err := engine.fireSyncRule(ctx, sync, comp, flowToken, bindings)
	require.NoError(t, err)

	// Fire sync rule second time with same bindings - should be idempotent
	err = engine.fireSyncRule(ctx, sync, comp, flowToken, bindings)
	require.NoError(t, err)

	// Verify only one firing recorded
	firings, err := s.ReadSyncFiringsForCompletion(ctx, compID)
	require.NoError(t, err)
	assert.Len(t, firings, 1, "should only have one firing due to idempotency")

	// Verify only one provenance edge
	edges, err := s.ReadAllProvenanceEdges(ctx)
	require.NoError(t, err)
	assert.Len(t, edges, 1, "should only have one provenance edge")
}

func TestFlowTokenChain_TwoLevelsDeep(t *testing.T) {
	s := setupTestStore(t)
	flowGen := NewFixedGenerator("flow-chain-xyz")
	engine := New(s, nil, nil, flowGen)

	ctx := context.Background()
	flowToken := "flow-chain-xyz"

	// Level 1: User initiates Order.Create
	args1 := ir.IRObject{"product": ir.IRString("widget"), "qty": ir.IRInt(5)}
	invID1 := ir.MustInvocationID(flowToken, "Order.Create", args1, 1)
	inv1 := ir.Invocation{
		ID:        invID1,
		FlowToken: flowToken,
		ActionURI: "Order.Create",
		Args:      args1,
		Seq:       1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
		SpecHash:      "spec-hash-1",
		EngineVersion: ir.EngineVersion,
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv1))

	// Level 1 completion
	result1 := ir.IRObject{"order_id": ir.IRString("order-123")}
	compID1 := ir.MustCompletionID(invID1, "Success", result1, 2)
	comp1 := &ir.Completion{
		ID:           compID1,
		InvocationID: invID1,
		OutputCase:   "Success",
		Result:       result1,
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
	require.NoError(t, s.WriteCompletion(ctx, *comp1))

	// Level 2: Sync generates Inventory.ReserveStock
	sync1 := ir.SyncRule{
		ID: "sync-reserve",
		When: ir.WhenClause{
			ActionRef:  "Order.Create",
			EventType:  "completed",
			OutputCase: "Success",
			Bindings: map[string]string{
				"order_id": "order_id",
			},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.ReserveStock",
			Args: map[string]string{
				"order_id": "${bound.order_id}",
			},
		},
	}
	bindings1 := ir.IRObject{
		"order_id": ir.IRString("order-123"),
	}

	// Fire sync 1
	err := engine.fireSyncRule(ctx, sync1, comp1, flowToken, bindings1)
	require.NoError(t, err)

	// Find inv2 (generated by sync1)
	edges1, err := s.ReadAllProvenanceEdges(ctx)
	require.NoError(t, err)
	require.Len(t, edges1, 1)

	inv2, err := s.ReadInvocation(ctx, edges1[0].InvocationID)
	require.NoError(t, err)
	assert.Equal(t, flowToken, inv2.FlowToken, "level 2 invocation must inherit flow token")

	// Level 2 completion
	result2 := ir.IRObject{"reservation_id": ir.IRString("res-456")}
	compID2 := ir.MustCompletionID(inv2.ID, "Success", result2, inv2.Seq+1)
	comp2 := &ir.Completion{
		ID:           compID2,
		InvocationID: inv2.ID,
		OutputCase:   "Success",
		Result:       result2,
		Seq:          inv2.Seq + 1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
	require.NoError(t, s.WriteCompletion(ctx, *comp2))

	// Level 3: Sync generates Notification.Send
	sync2 := ir.SyncRule{
		ID: "sync-notify",
		When: ir.WhenClause{
			ActionRef:  "Inventory.ReserveStock",
			EventType:  "completed",
			OutputCase: "Success",
			Bindings: map[string]string{
				"reservation_id": "reservation_id",
			},
		},
		Then: ir.ThenClause{
			ActionRef: "Notification.Send",
			Args: map[string]string{
				"reservation_id": "${bound.reservation_id}",
			},
		},
	}
	bindings2 := ir.IRObject{
		"reservation_id": ir.IRString("res-456"),
	}

	// Fire sync 2
	err = engine.fireSyncRule(ctx, sync2, comp2, flowToken, bindings2)
	require.NoError(t, err)

	// Find inv3 (generated by sync2)
	edges2, err := s.ReadAllProvenanceEdges(ctx)
	require.NoError(t, err)
	require.Len(t, edges2, 2)

	// Get the second edge (sync2's provenance)
	var inv3 ir.Invocation
	for _, edge := range edges2 {
		if edge.InvocationID != inv2.ID {
			inv3, err = s.ReadInvocation(ctx, edge.InvocationID)
			require.NoError(t, err)
			break
		}
	}
	assert.Equal(t, flowToken, inv3.FlowToken, "level 3 invocation must inherit flow token")

	// Verify entire chain has same flow token
	assert.Equal(t, flowToken, inv1.FlowToken, "inv1 flow token")
	assert.Equal(t, flowToken, inv2.FlowToken, "inv2 flow token")
	assert.Equal(t, flowToken, inv3.FlowToken, "inv3 flow token")
}

func TestResolveArgs_EmptyArgs(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	args := map[string]string{}
	bindings := ir.IRObject{}

	result, err := engine.resolveArgs(args, bindings)
	require.NoError(t, err)
	assert.Equal(t, ir.IRObject{}, result)
}

func TestResolveArgs_LiteralOnly(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	args := map[string]string{
		"status":   "pending",
		"priority": "high",
	}
	bindings := ir.IRObject{}

	result, err := engine.resolveArgs(args, bindings)
	require.NoError(t, err)
	assert.Equal(t, ir.IRString("pending"), result["status"])
	assert.Equal(t, ir.IRString("high"), result["priority"])
}

func TestResolveArgs_BindingsOnly(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	args := map[string]string{
		"product_id": "${bound.pid}",
		"quantity":   "${bound.qty}",
	}
	bindings := ir.IRObject{
		"pid": ir.IRString("prod-123"),
		"qty": ir.IRInt(10),
	}

	result, err := engine.resolveArgs(args, bindings)
	require.NoError(t, err)
	assert.Equal(t, ir.IRString("prod-123"), result["product_id"])
	assert.Equal(t, ir.IRInt(10), result["quantity"])
}

func TestResolveArgs_MixedLiteralAndBindings(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	args := map[string]string{
		"product_id": "${bound.pid}",
		"status":     "active",
		"quantity":   "${bound.qty}",
		"priority":   "normal",
	}
	bindings := ir.IRObject{
		"pid": ir.IRString("prod-123"),
		"qty": ir.IRInt(5),
	}

	result, err := engine.resolveArgs(args, bindings)
	require.NoError(t, err)
	assert.Equal(t, ir.IRString("prod-123"), result["product_id"])
	assert.Equal(t, ir.IRString("active"), result["status"])
	assert.Equal(t, ir.IRInt(5), result["quantity"])
	assert.Equal(t, ir.IRString("normal"), result["priority"])
}

func TestSubstituteBinding_AllIRValueTypes(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	testCases := []struct {
		name     string
		binding  ir.IRValue
		expected ir.IRValue
	}{
		{"string", ir.IRString("hello"), ir.IRString("hello")},
		{"int", ir.IRInt(42), ir.IRInt(42)},
		{"bool", ir.IRBool(true), ir.IRBool(true)},
		{"array", ir.IRArray{ir.IRInt(1), ir.IRInt(2)}, ir.IRArray{ir.IRInt(1), ir.IRInt(2)}},
		{"object", ir.IRObject{"key": ir.IRString("value")}, ir.IRObject{"key": ir.IRString("value")}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bindings := ir.IRObject{
				"var": tc.binding,
			}

			result, err := engine.substituteBinding("${bound.var}", bindings)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGenerateInvocation_ContentAddressedID(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-content-addr"
	then := ir.ThenClause{
		ActionRef: "Test.Action",
		Args: map[string]string{
			"key": "value",
		},
	}
	bindings := ir.IRObject{}

	inv, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	// ID should be content-addressed (64 hex chars = SHA256)
	assert.Len(t, inv.ID, 64, "ID should be SHA256 hex string")

	// Generate again with same inputs - should produce same ID
	// Note: seq will be different, so ID will differ
	// We verify the ID format is correct
	assert.Regexp(t, `^[0-9a-f]{64}$`, inv.ID, "ID should be hex string")
}

func TestGenerateInvocation_SetsVersionFields(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-version-test"
	then := ir.ThenClause{
		ActionRef: "Test.Action",
		Args:      map[string]string{},
	}
	bindings := ir.IRObject{}

	inv, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	assert.Equal(t, ir.EngineVersion, inv.EngineVersion)
	assert.Equal(t, ir.IRVersion, inv.IRVersion)
	assert.NotEmpty(t, inv.SecurityContext.TenantID, "should have security context")
}

func TestGenerateInvocation_SequenceNumber(t *testing.T) {
	engine := setupTestEngineMinimal(t)

	flowToken := "flow-seq-test"
	then := ir.ThenClause{
		ActionRef: "Test.Action",
		Args:      map[string]string{},
	}
	bindings := ir.IRObject{}

	// Generate first invocation
	inv1, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	// Generate second invocation
	inv2, err := engine.generateInvocation(flowToken, then, bindings)
	require.NoError(t, err)

	// Sequence numbers should be increasing
	assert.Greater(t, inv2.Seq, inv1.Seq, "sequence numbers should be increasing")
}

// Helper function to set up a minimal engine for unit tests
func setupTestEngineMinimal(t *testing.T) *Engine {
	t.Helper()
	s := setupTestStore(t)
	flowGen := NewFixedGenerator("flow-test")
	return New(s, nil, nil, flowGen)
}
