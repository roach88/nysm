package store

import (
	"context"
	"database/sql"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestWriteSyncFiring_Basic(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisite invocation and completion
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	if err := store.WriteInvocation(ctx, inv); err != nil {
		t.Fatalf("WriteInvocation failed: %v", err)
	}

	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	if err := store.WriteCompletion(ctx, comp); err != nil {
		t.Fatalf("WriteCompletion failed: %v", err)
	}

	// Write sync firing
	firing := ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-123",
		Seq:          3,
	}

	id, inserted, err := store.WriteSyncFiring(ctx, firing)
	if err != nil {
		t.Fatalf("WriteSyncFiring failed: %v", err)
	}

	if id <= 0 {
		t.Errorf("expected positive ID, got %d", id)
	}
	if !inserted {
		t.Error("expected inserted=true for new firing")
	}

	// Verify it was written
	readFiring, err := store.ReadSyncFiring(ctx, id)
	if err != nil {
		t.Fatalf("ReadSyncFiring failed: %v", err)
	}

	if readFiring.CompletionID != firing.CompletionID {
		t.Errorf("CompletionID = %q, want %q", readFiring.CompletionID, firing.CompletionID)
	}
	if readFiring.SyncID != firing.SyncID {
		t.Errorf("SyncID = %q, want %q", readFiring.SyncID, firing.SyncID)
	}
	if readFiring.BindingHash != firing.BindingHash {
		t.Errorf("BindingHash = %q, want %q", readFiring.BindingHash, firing.BindingHash)
	}
	if readFiring.Seq != firing.Seq {
		t.Errorf("Seq = %d, want %d", readFiring.Seq, firing.Seq)
	}
}

func TestWriteSyncFiring_Idempotency(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Write same firing twice
	firing := ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-123",
		Seq:          3,
	}

	id1, inserted1, err := store.WriteSyncFiring(ctx, firing)
	if err != nil {
		t.Fatalf("First WriteSyncFiring failed: %v", err)
	}

	id2, inserted2, err := store.WriteSyncFiring(ctx, firing)
	if err != nil {
		t.Fatalf("Second WriteSyncFiring failed: %v", err)
	}

	// Verify only one record exists
	firings, err := store.ReadSyncFiringsForCompletion(ctx, "comp-1")
	if err != nil {
		t.Fatalf("ReadSyncFiringsForCompletion failed: %v", err)
	}

	if len(firings) != 1 {
		t.Errorf("expected 1 firing, got %d", len(firings))
	}

	// First call should have inserted
	if id1 <= 0 {
		t.Errorf("first write should return positive ID, got %d", id1)
	}
	if !inserted1 {
		t.Error("first write should have inserted=true")
	}

	// Second call should return same ID but inserted=false
	if id2 != id1 {
		t.Errorf("second write should return same ID: got %d, want %d", id2, id1)
	}
	if inserted2 {
		t.Error("second write should have inserted=false")
	}
}

func TestWriteSyncFiring_DifferentBindingHash(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Write two firings with different binding hashes (multi-binding scenario)
	firing1 := ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-item-1",
		Seq:          3,
	}
	firing2 := ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-item-2",
		Seq:          4,
	}

	_, _, err := store.WriteSyncFiring(ctx, firing1)
	if err != nil {
		t.Fatalf("First WriteSyncFiring failed: %v", err)
	}

	_, _, err = store.WriteSyncFiring(ctx, firing2)
	if err != nil {
		t.Fatalf("Second WriteSyncFiring failed: %v", err)
	}

	// Both should be stored (different binding hashes per CP-1)
	firings, err := store.ReadSyncFiringsForCompletion(ctx, "comp-1")
	if err != nil {
		t.Fatalf("ReadSyncFiringsForCompletion failed: %v", err)
	}

	if len(firings) != 2 {
		t.Errorf("expected 2 firings, got %d", len(firings))
	}
}

func TestHasFiring(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Check before writing
	exists, err := store.HasFiring(ctx, "comp-1", "cart-inventory", "hash-123")
	if err != nil {
		t.Fatalf("HasFiring failed: %v", err)
	}
	if exists {
		t.Error("HasFiring returned true before writing")
	}

	// Write firing
	firing := ir.SyncFiring{
		CompletionID: "comp-1",
		SyncID:       "cart-inventory",
		BindingHash:  "hash-123",
		Seq:          3,
	}
	store.WriteSyncFiring(ctx, firing)

	// Check after writing
	exists, err = store.HasFiring(ctx, "comp-1", "cart-inventory", "hash-123")
	if err != nil {
		t.Fatalf("HasFiring failed: %v", err)
	}
	if !exists {
		t.Error("HasFiring returned false after writing")
	}

	// Different binding hash should not exist
	exists, err = store.HasFiring(ctx, "comp-1", "cart-inventory", "different-hash")
	if err != nil {
		t.Fatalf("HasFiring failed: %v", err)
	}
	if exists {
		t.Error("HasFiring returned true for different binding hash")
	}
}

func TestReadSyncFiringsForCompletion_Ordering(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites
	inv := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv)
	comp := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp)

	// Write firings out of order
	firings := []ir.SyncFiring{
		{CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h3", Seq: 5},
		{CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 3},
		{CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h2", Seq: 4},
	}

	for _, f := range firings {
		store.WriteSyncFiring(ctx, f)
	}

	// Read should be ordered by seq ASC
	result, err := store.ReadSyncFiringsForCompletion(ctx, "comp-1")
	if err != nil {
		t.Fatalf("ReadSyncFiringsForCompletion failed: %v", err)
	}

	if len(result) != 3 {
		t.Fatalf("expected 3 firings, got %d", len(result))
	}

	// Verify order: seq 3, 4, 5
	expectedSeqs := []int64{3, 4, 5}
	for i, f := range result {
		if f.Seq != expectedSeqs[i] {
			t.Errorf("result[%d].Seq = %d, want %d", i, f.Seq, expectedSeqs[i])
		}
	}
}

func TestReadAllSyncFirings(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	// Create prerequisites - two separate invocations (one completion per invocation)
	inv1 := createTestInvocation("inv-1", "flow-1", "Cart.addItem", 1)
	store.WriteInvocation(ctx, inv1)
	comp1 := createTestCompletion("comp-1", "inv-1", "Success", 2)
	store.WriteCompletion(ctx, comp1)

	inv2 := createTestInvocation("inv-2", "flow-1", "Cart.addItem", 3)
	store.WriteInvocation(ctx, inv2)
	comp2 := createTestCompletion("comp-2", "inv-2", "Success", 4)
	store.WriteCompletion(ctx, comp2)

	// Write firings for different completions
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-1", SyncID: "sync-1", BindingHash: "h1", Seq: 5,
	})
	store.WriteSyncFiring(ctx, ir.SyncFiring{
		CompletionID: "comp-2", SyncID: "sync-1", BindingHash: "h2", Seq: 6,
	})

	// Read all
	firings, err := store.ReadAllSyncFirings(ctx)
	if err != nil {
		t.Fatalf("ReadAllSyncFirings failed: %v", err)
	}

	if len(firings) != 2 {
		t.Errorf("expected 2 firings, got %d", len(firings))
	}
}

func TestReadSyncFiring_NotFound(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	_, err := store.ReadSyncFiring(ctx, 99999)
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestReadSyncFiringsForCompletion_Empty(t *testing.T) {
	store := createTestStore(t)
	ctx := context.Background()

	firings, err := store.ReadSyncFiringsForCompletion(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ReadSyncFiringsForCompletion failed: %v", err)
	}

	if firings == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(firings) != 0 {
		t.Errorf("expected 0 firings, got %d", len(firings))
	}
}
