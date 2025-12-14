package store

import (
	"context"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestWriteProvenanceEdge_Basic(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisite chain: invocation -> completion -> sync_firing -> new invocation
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv1)

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	firingID, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-123",
		Seq:          3,
	})

	// Create the triggered invocation
	inv2 := createTestInvocation("inv-2", "flow-1", "Inventory.reserve", 4)
	store.WriteInvocation(ctx, inv2)

	// Write provenance edge
	err := store.WriteProvenanceEdge(ctx, firingID, "inv-2")
	if err != nil {
		t.Fatalf("WriteProvenanceEdge failed: %v", err)
	}

	// Verify by reading provenance
	edges, err := store.ReadProvenance(ctx, "inv-2")
	if err != nil {
		t.Fatalf("ReadProvenance failed: %v", err)
	}

	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}

	if edges[0].SyncFiringID != firingID {
		t.Errorf("SyncFiringID = %d, want %d", edges[0].SyncFiringID, firingID)
	}
	if edges[0].InvocationID != "inv-2" {
		t.Errorf("InvocationID = %q, want %q", edges[0].InvocationID, "inv-2")
	}
}

func TestWriteProvenanceEdge_Idempotency(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv1)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)
	firingID, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})
	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 4)
	store.WriteInvocation(ctx, inv2)

	// Write edge twice
	err := store.WriteProvenanceEdge(ctx, firingID, "inv-2")
	if err != nil {
		t.Fatalf("First WriteProvenanceEdge failed: %v", err)
	}

	err = store.WriteProvenanceEdge(ctx, firingID, "inv-2")
	if err != nil {
		t.Fatalf("Second WriteProvenanceEdge should not fail: %v", err)
	}

	// Should only have one edge (UNIQUE constraint)
	edges, _ := store.ReadProvenance(ctx, "inv-2")
	if len(edges) != 1 {
		t.Errorf("expected 1 edge after duplicate insert, got %d", len(edges))
	}
}

func TestReadTriggered(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create a chain: checkout completion triggers two reserve invocations
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv1)

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Two firings (different bindings)
	firingID1, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "cart-inventory", BindingHash: "item-1", Seq: 3,
	})
	firingID2, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "cart-inventory", BindingHash: "item-2", Seq: 4,
	})

	// Two triggered invocations
	inv2 := createTestInvocation("inv-2", "flow-1", "Inventory.reserve", 5)
	store.WriteInvocation(ctx, inv2)
	inv3 := createTestInvocation("inv-3", "flow-1", "Inventory.reserve", 6)
	store.WriteInvocation(ctx, inv3)

	// Link them
	store.WriteProvenanceEdge(ctx, firingID1, "inv-2")
	store.WriteProvenanceEdge(ctx, firingID2, "inv-3")

	// Query forward: what did comp-1 trigger?
	triggered, err := store.ReadTriggered(ctx, "comp-1")
	if err != nil {
		t.Fatalf("ReadTriggered failed: %v", err)
	}

	if len(triggered) != 2 {
		t.Fatalf("expected 2 triggered invocations, got %d", len(triggered))
	}

	// Should be ordered by firing seq
	if triggered[0].ID != "inv-2" {
		t.Errorf("first triggered ID = %q, want %q", triggered[0].ID, "inv-2")
	}
	if triggered[1].ID != "inv-3" {
		t.Errorf("second triggered ID = %q, want %q", triggered[1].ID, "inv-3")
	}
}

func TestReadProvenance_Empty(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	edges, err := store.ReadProvenance(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ReadProvenance failed: %v", err)
	}

	if edges == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(edges) != 0 {
		t.Errorf("expected 0 edges, got %d", len(edges))
	}
}

func TestReadTriggered_Empty(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create a completion with no triggered invocations
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	triggered, err := store.ReadTriggered(ctx, "comp-1")
	if err != nil {
		t.Fatalf("ReadTriggered failed: %v", err)
	}

	if triggered == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(triggered) != 0 {
		t.Errorf("expected 0 triggered, got %d", len(triggered))
	}
}

func TestReadAllProvenanceEdges(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create chain
	inv1 := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)
	firingID, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})
	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 4)
	store.WriteInvocation(ctx, inv2)
	store.WriteProvenanceEdge(ctx, firingID, "inv-2")

	// Read all edges
	edges, err := store.ReadAllProvenanceEdges(ctx)
	if err != nil {
		t.Fatalf("ReadAllProvenanceEdges failed: %v", err)
	}

	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
}

func TestProvenanceChain(t *testing.T) {
	// Test a multi-hop provenance chain:
	// A completes -> fires sync -> B invoked -> B completes -> fires sync -> C invoked
	store := createTestStore(t)
	ctx := context.Background()

	// Hop 1: A
	invA := createTestInvocation("inv-a", "flow-1", "Action.A", 1)
	store.WriteInvocation(ctx, invA)
	compA := createTestCompletion("comp-a", "inv-a", "Success", 2)
	store.WriteCompletion(ctx, compA)

	firingAB, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-a", SyncID: "sync-a-to-b", BindingHash: "h1", Seq: 3,
	})

	// Hop 2: B
	invB := createTestInvocation("inv-b", "flow-1", "Action.B", 4)
	store.WriteInvocation(ctx, invB)
	store.WriteProvenanceEdge(ctx, firingAB, "inv-b")

	compB := createTestCompletion("comp-b", "inv-b", "Success", 5)
	store.WriteCompletion(ctx, compB)

	firingBC, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-b", SyncID: "sync-b-to-c", BindingHash: "h2", Seq: 6,
	})

	// Hop 3: C
	invC := createTestInvocation("inv-c", "flow-1", "Action.C", 7)
	store.WriteInvocation(ctx, invC)
	store.WriteProvenanceEdge(ctx, firingBC, "inv-c")

	// Trace backward from C
	edgesC, _ := store.ReadProvenance(ctx, "inv-c")
	if len(edgesC) != 1 {
		t.Fatalf("expected 1 edge to C, got %d", len(edgesC))
	}
	if edgesC[0].SyncFiringID != firingBC {
		t.Errorf("C edge firing = %d, want %d", edgesC[0].SyncFiringID, firingBC)
	}

	// Trace forward from A
	triggeredA, _ := store.ReadTriggered(ctx, "comp-a")
	if len(triggeredA) != 1 {
		t.Fatalf("expected 1 triggered from A, got %d", len(triggeredA))
	}
	if triggeredA[0].ID != "inv-b" {
		t.Errorf("triggered from A = %q, want %q", triggeredA[0].ID, "inv-b")
	}
}
