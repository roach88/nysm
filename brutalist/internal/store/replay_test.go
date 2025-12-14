package store

import (
	"context"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestGetFlowState_Basic(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create a simple flow
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	state, err := store.GetFlowState(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetFlowState failed: %v", err)
	}

	if state.FlowToken != "flow-1" {
		t.Errorf("FlowToken = %q, want %q", state.FlowToken, "flow-1")
	}
	if len(state.Invocations) != 1 {
		t.Errorf("len(Invocations) = %d, want 1", len(state.Invocations))
	}
	if len(state.Completions) != 1 {
		t.Errorf("len(Completions) = %d, want 1", len(state.Completions))
	}
	if !state.IsComplete {
		t.Error("IsComplete = false, want true")
	}
	if state.PendingCount != 0 {
		t.Errorf("PendingCount = %d, want 0", state.PendingCount)
	}
	if state.LastSeq != 2 {
		t.Errorf("LastSeq = %d, want 2", state.LastSeq)
	}
}

func TestGetFlowState_Incomplete(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create incomplete flow (invocation without completion)
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)

	state, err := store.GetFlowState(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetFlowState failed: %v", err)
	}

	if state.IsComplete {
		t.Error("IsComplete = true, want false")
	}
	if state.PendingCount != 1 {
		t.Errorf("PendingCount = %d, want 1", state.PendingCount)
	}
}

func TestGetFlowState_WithSyncFirings(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with sync firings
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv1)

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h2", Seq: 4,
	})

	// Add triggered invocations with completions
	inv2 := createTestInvocation("inv-2", "flow-1", "Inventory.reserve", 5)
	store.WriteInvocation(ctx, inv2)
	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 6)
	store.WriteCompletion(ctx, comp2)

	inv3 := createTestInvocation("inv-3", "flow-1", "Inventory.reserve", 7)
	store.WriteInvocation(ctx, inv3)
	comp3 := createTestCompletion("comp-3", "inv-3", "Success", 8)
	store.WriteCompletion(ctx, comp3)

	state, err := store.GetFlowState(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetFlowState failed: %v", err)
	}

	if len(state.SyncFirings) != 2 {
		t.Errorf("len(SyncFirings) = %d, want 2", len(state.SyncFirings))
	}
	if state.LastSeq != 8 {
		t.Errorf("LastSeq = %d, want 8", state.LastSeq)
	}
}

func TestFindIncompleteFlows(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create complete flow
	inv1 := createTestInvocation("inv-1", "flow-complete", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	// Create incomplete flow
	inv2 := createTestInvocation("inv-2", "flow-incomplete", "Action.two", 3)
	store.WriteInvocation(ctx, inv2)

	// Create another incomplete flow
	inv3 := createTestInvocation("inv-3", "flow-partial", "Action.three", 4)
	store.WriteInvocation(ctx, inv3)
	comp3 := createTestCompletion("comp-3", "inv-3", "Success", 5)
	store.WriteCompletion(ctx, comp3)
	inv4 := createTestInvocation("inv-4", "flow-partial", "Action.four", 6)
	store.WriteInvocation(ctx, inv4) // No completion for this one

	incomplete, err := store.FindIncompleteFlows(ctx)
	if err != nil {
		t.Fatalf("FindIncompleteFlows failed: %v", err)
	}

	if len(incomplete) != 2 {
		t.Fatalf("len(incomplete) = %d, want 2", len(incomplete))
	}

	// Verify the incomplete flows
	tokens := make(map[string]bool)
	for _, state := range incomplete {
		tokens[state.FlowToken] = true
	}

	if !tokens["flow-incomplete"] {
		t.Error("expected flow-incomplete in results")
	}
	if !tokens["flow-partial"] {
		t.Error("expected flow-partial in results")
	}
	if tokens["flow-complete"] {
		t.Error("unexpected flow-complete in results")
	}
}

func TestGetPendingInvocations(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with some completed and some pending invocations
	inv1 := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 3)
	store.WriteInvocation(ctx, inv2) // No completion

	inv3 := createTestInvocation("inv-3", "flow-1", "Action.three", 4)
	store.WriteInvocation(ctx, inv3) // No completion

	pending, err := store.GetPendingInvocations(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetPendingInvocations failed: %v", err)
	}

	if len(pending) != 2 {
		t.Fatalf("len(pending) = %d, want 2", len(pending))
	}

	// Should be ordered by seq
	if pending[0].ID != "inv-2" {
		t.Errorf("pending[0].ID = %q, want %q", pending[0].ID, "inv-2")
	}
	if pending[1].ID != "inv-3" {
		t.Errorf("pending[1].ID = %q, want %q", pending[1].ID, "inv-3")
	}
}

func TestGetPendingInvocations_Empty(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Complete flow
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	pending, err := store.GetPendingInvocations(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetPendingInvocations failed: %v", err)
	}

	if pending == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(pending) != 0 {
		t.Errorf("len(pending) = %d, want 0", len(pending))
	}
}

func TestReplayFlow(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create a multi-step flow
	inv1 := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)

	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 3)
	store.WriteInvocation(ctx, inv2)

	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 4)
	store.WriteCompletion(ctx, comp2)

	events, err := store.ReplayFlow(ctx, "flow-1")
	if err != nil {
		t.Fatalf("ReplayFlow failed: %v", err)
	}

	if len(events) != 4 {
		t.Fatalf("len(events) = %d, want 4", len(events))
	}

	// Verify order: inv1, comp1, inv2, comp2
	expectedSeqs := []int64{1, 2, 3, 4}
	expectedTypes := []FlowEventType{EventInvocation, EventCompletion, EventInvocation, EventCompletion}

	for i, event := range events {
		if event.Seq != expectedSeqs[i] {
			t.Errorf("events[%d].Seq = %d, want %d", i, event.Seq, expectedSeqs[i])
		}
		if event.Type != expectedTypes[i] {
			t.Errorf("events[%d].Type = %v, want %v", i, event.Type, expectedTypes[i])
		}
	}
}

func TestReplayFlow_SameSeqOrdering(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create events with same seq (edge case - shouldn't happen normally but test determinism)
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv)

	// Completion with same seq as a theoretical other invocation
	comp := createTestCompletion("comp-1", "inv-1", "Success", 1)
	store.WriteCompletion(ctx, comp)

	events, err := store.ReplayFlow(ctx, "flow-1")
	if err != nil {
		t.Fatalf("ReplayFlow failed: %v", err)
	}

	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}

	// Invocation should come before completion for same seq
	if events[0].Type != EventInvocation {
		t.Error("expected invocation first for same seq")
	}
	if events[1].Type != EventCompletion {
		t.Error("expected completion second for same seq")
	}
}

func TestGetLastSeq(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Empty store
	seq, err := store.GetLastSeq(ctx)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 0 {
		t.Errorf("GetLastSeq on empty store = %d, want 0", seq)
	}

	// Add some records
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 5)
	store.WriteInvocation(ctx, inv)

	seq, err = store.GetLastSeq(ctx)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 5 {
		t.Errorf("GetLastSeq = %d, want 5", seq)
	}

	// Add completion with higher seq
	comp := createTestCompletion("comp-1", "inv-1", "Success", 10)
	store.WriteCompletion(ctx, comp)

	seq, err = store.GetLastSeq(ctx)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 10 {
		t.Errorf("GetLastSeq = %d, want 10", seq)
	}

	// Add sync firing with highest seq
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 15,
	})

	seq, err = store.GetLastSeq(ctx)
	if err != nil {
		t.Fatalf("GetLastSeq failed: %v", err)
	}
	if seq != 15 {
		t.Errorf("GetLastSeq = %d, want 15", seq)
	}
}

func TestGetLastSeqForFlow(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create two flows
	inv1 := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	inv2 := createTestInvocation("inv-2", "flow-2", "Action.two", 10)
	store.WriteInvocation(ctx, inv2)
	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 20)
	store.WriteCompletion(ctx, comp2)

	// Get last seq for flow-1
	seq, err := store.GetLastSeqForFlow(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetLastSeqForFlow failed: %v", err)
	}
	if seq != 2 {
		t.Errorf("GetLastSeqForFlow(flow-1) = %d, want 2", seq)
	}

	// Get last seq for flow-2
	seq, err = store.GetLastSeqForFlow(ctx, "flow-2")
	if err != nil {
		t.Fatalf("GetLastSeqForFlow failed: %v", err)
	}
	if seq != 20 {
		t.Errorf("GetLastSeqForFlow(flow-2) = %d, want 20", seq)
	}

	// Get last seq for nonexistent flow
	seq, err = store.GetLastSeqForFlow(ctx, "flow-nonexistent")
	if err != nil {
		t.Fatalf("GetLastSeqForFlow failed: %v", err)
	}
	if seq != 0 {
		t.Errorf("GetLastSeqForFlow(nonexistent) = %d, want 0", seq)
	}
}

func TestFlowEventType_String(t *testing.T) {
	if EventInvocation.String() != "invocation" {
		t.Errorf("EventInvocation.String() = %q, want %q", EventInvocation.String(), "invocation")
	}
	if EventCompletion.String() != "completion" {
		t.Errorf("EventCompletion.String() = %q, want %q", EventCompletion.String(), "completion")
	}
	if FlowEventType(99).String() != "unknown" {
		t.Errorf("Unknown type String() = %q, want %q", FlowEventType(99).String(), "unknown")
	}
}

func TestGetFlowState_OrphanedFirings(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with a sync firing but NO provenance edge (simulates crash)
	inv := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv)

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Write sync firing but don't create provenance edge (crash scenario)
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})

	state, err := store.GetFlowState(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetFlowState failed: %v", err)
	}

	// Flow should NOT be complete because of orphaned firing
	if state.IsComplete {
		t.Error("IsComplete = true, want false (orphaned firing)")
	}
	if state.OrphanedFirings != 1 {
		t.Errorf("OrphanedFirings = %d, want 1", state.OrphanedFirings)
	}
	if state.PendingCount != 0 {
		t.Errorf("PendingCount = %d, want 0", state.PendingCount)
	}
}

func TestGetFlowState_CompleteWithProvenance(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with proper sync firing AND provenance edge
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.checkout", 1)
	store.WriteInvocation(ctx, inv1)

	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	// Write sync firing
	firingID, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})

	// Create triggered invocation with provenance edge
	inv2 := createTestInvocation("inv-2", "flow-1", "Inventory.reserve", 4)
	store.WriteInvocation(ctx, inv2)
	store.WriteProvenanceEdge(ctx, firingID, "inv-2")

	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 5)
	store.WriteCompletion(ctx, comp2)

	state, err := store.GetFlowState(ctx, "flow-1")
	if err != nil {
		t.Fatalf("GetFlowState failed: %v", err)
	}

	// Flow should be complete
	if !state.IsComplete {
		t.Error("IsComplete = false, want true")
	}
	if state.OrphanedFirings != 0 {
		t.Errorf("OrphanedFirings = %d, want 0", state.OrphanedFirings)
	}
	if state.PendingCount != 0 {
		t.Errorf("PendingCount = %d, want 0", state.PendingCount)
	}
}

func TestFindIncompleteFlows_OrphanedFirings(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create complete flow
	inv1 := createTestInvocation("inv-1", "flow-complete", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	// Create flow with orphaned sync firing (crash scenario)
	inv2 := createTestInvocation("inv-2", "flow-orphaned", "Action.two", 3)
	store.WriteInvocation(ctx, inv2)
	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 4)
	store.WriteCompletion(ctx, comp2)
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-2", SyncID: "sync-1", BindingHash: "h1", Seq: 5,
	})
	// Note: No provenance edge created

	incomplete, err := store.FindIncompleteFlows(ctx)
	if err != nil {
		t.Fatalf("FindIncompleteFlows failed: %v", err)
	}

	if len(incomplete) != 1 {
		t.Fatalf("len(incomplete) = %d, want 1", len(incomplete))
	}
	if incomplete[0].FlowToken != "flow-orphaned" {
		t.Errorf("FlowToken = %q, want %q", incomplete[0].FlowToken, "flow-orphaned")
	}
	if incomplete[0].OrphanedFirings != 1 {
		t.Errorf("OrphanedFirings = %d, want 1", incomplete[0].OrphanedFirings)
	}
}

func TestFindOrphanedSyncFirings(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with mixed firings
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Firing with provenance edge (complete)
	firingID1, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})
	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 4)
	store.WriteInvocation(ctx, inv2)
	store.WriteProvenanceEdge(ctx, firingID1, "inv-2")

	// Firing without provenance edge (orphaned)
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h2", Seq: 5,
	})

	orphans, err := store.FindOrphanedSyncFirings(ctx)
	if err != nil {
		t.Fatalf("FindOrphanedSyncFirings failed: %v", err)
	}

	if len(orphans) != 1 {
		t.Fatalf("len(orphans) = %d, want 1", len(orphans))
	}
	if orphans[0].BindingHash != "h2" {
		t.Errorf("orphan BindingHash = %q, want %q", orphans[0].BindingHash, "h2")
	}
}

func TestFindOrphanedSyncFirings_Empty(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create flow with properly linked firing
	inv := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	firingID, _, _ := store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3,
	})
	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 4)
	store.WriteInvocation(ctx, inv2)
	store.WriteProvenanceEdge(ctx, firingID, "inv-2")

	orphans, err := store.FindOrphanedSyncFirings(ctx)
	if err != nil {
		t.Fatalf("FindOrphanedSyncFirings failed: %v", err)
	}

	if orphans == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(orphans) != 0 {
		t.Errorf("len(orphans) = %d, want 0", len(orphans))
	}
}

func TestReplayDeterminism(t *testing.T) {
	// Test that replaying the same flow produces identical results
	store := createTestStore(t)
	ctx := context.Background()

	// Create a flow
	inv1 := createTestInvocation("inv-1", "flow-1", "Action.one", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	inv2 := createTestInvocation("inv-2", "flow-1", "Action.two", 3)
	store.WriteInvocation(ctx, inv2)
	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 4)
	store.WriteCompletion(ctx, comp2)

	// Replay multiple times and compare
	events1, _ := store.ReplayFlow(ctx, "flow-1")
	events2, _ := store.ReplayFlow(ctx, "flow-1")
	events3, _ := store.ReplayFlow(ctx, "flow-1")

	if len(events1) != len(events2) || len(events2) != len(events3) {
		t.Fatal("replay produced different number of events")
	}

	for i := range events1 {
		if events1[i].Seq != events2[i].Seq || events2[i].Seq != events3[i].Seq {
			t.Errorf("replay[%d].Seq differs between runs", i)
		}
		if events1[i].Type != events2[i].Type || events2[i].Type != events3[i].Type {
			t.Errorf("replay[%d].Type differs between runs", i)
		}
		if events1[i].ID != events2[i].ID || events2[i].ID != events3[i].ID {
			t.Errorf("replay[%d].ID differs between runs", i)
		}
	}
}
