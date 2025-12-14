package engine

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// stubFlowGen is a test-only flow generator that returns fixed tokens.
type stubFlowGen struct {
	tokens []string
	idx    int
}

func (g *stubFlowGen) Generate() string {
	if g.idx >= len(g.tokens) {
		panic("stubFlowGen: no more tokens")
	}
	token := g.tokens[g.idx]
	g.idx++
	return token
}

func newStubFlowGen(tokens ...string) *stubFlowGen {
	return &stubFlowGen{tokens: tokens}
}

func setupTestStore(t *testing.T) *store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestEngine_New(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("test-flow")

	specs := []ir.ConceptSpec{{Name: "Test"}}
	syncs := []ir.SyncRule{{ID: "sync-1"}}

	engine := New(s, specs, syncs, flowGen)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.clock)
	assert.NotNil(t, engine.queue)
	assert.Equal(t, 1, len(engine.specs))
	assert.Equal(t, 1, len(engine.syncs))
}

func TestEngine_NewFlow(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1", "flow-2", "flow-3")

	engine := New(s, nil, nil, flowGen)

	assert.Equal(t, "flow-1", engine.NewFlow())
	assert.Equal(t, "flow-2", engine.NewFlow())
	assert.Equal(t, "flow-3", engine.NewFlow())
}

func TestEngine_Enqueue(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	inv := &ir.Invocation{ID: "inv-1", FlowToken: "flow-1"}
	ok := engine.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})

	assert.True(t, ok)
	assert.Equal(t, 1, engine.QueueLen())
}

func TestEngine_Enqueue_AfterStop(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	engine.Stop()

	inv := &ir.Invocation{ID: "inv-1", FlowToken: "flow-1"}
	ok := engine.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})

	assert.False(t, ok, "enqueue after stop should fail")
}

func TestEngine_ProcessInvocation(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a valid invocation
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID("flow-1", "Cart.addItem", args, 1)
	inv := &ir.Invocation{
		ID:        invID,
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
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

	// Start engine in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(ctx)
	}()

	// Enqueue the invocation
	engine.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Verify invocation was written to store
	// (Store read methods would verify this - for now just check no error)
	engine.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "engine should stop cleanly")
	case <-time.After(time.Second):
		t.Fatal("engine did not stop")
	}
}

func TestEngine_ProcessCompletion(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First write an invocation (completions require foreign key)
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID("flow-1", "Cart.addItem", args, 1)
	inv := ir.Invocation{
		ID:        invID,
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
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

	// Create completion
	result := ir.IRObject{"cart_id": ir.IRString("cart-123")}
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

	// Start engine in background
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(ctx)
	}()

	// Enqueue the completion
	engine.Enqueue(Event{Type: EventTypeCompletion, Completion: comp})

	// Wait for processing
	time.Sleep(50 * time.Millisecond)

	// Stop and verify no errors
	engine.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err, "engine should stop cleanly")
	case <-time.After(time.Second):
		t.Fatal("engine did not stop")
	}
}

func TestEngine_Run_StopsOnContext(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(ctx)
	}()

	// Cancel context
	cancel()

	select {
	case err := <-errCh:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("engine did not stop on context cancellation")
	}
}

func TestEngine_Run_ProcessesMultipleEvents(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start engine
	errCh := make(chan error, 1)
	go func() {
		errCh <- engine.Run(ctx)
	}()

	// Enqueue multiple invocations
	for i := int64(1); i <= 3; i++ {
		args := ir.IRObject{"seq": ir.IRInt(i)}
		invID := ir.MustInvocationID("flow-1", "Test.action", args, i)
		inv := &ir.Invocation{
			ID:        invID,
			FlowToken: "flow-1",
			ActionURI: "Test.action",
			Args:      args,
			Seq:       i,
			SecurityContext: ir.SecurityContext{
				TenantID: "tenant-1",
				UserID:   "user-1",
			},
			SpecHash:      "spec-hash-1",
			EngineVersion: ir.EngineVersion,
			IRVersion:     ir.IRVersion,
		}
		engine.Enqueue(Event{Type: EventTypeInvocation, Invocation: inv})
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	// Queue should be empty after processing
	assert.Equal(t, 0, engine.QueueLen(), "queue should be empty after processing")

	engine.Stop()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("engine did not stop")
	}
}

func TestEngine_Clock(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	c := engine.Clock()
	assert.NotNil(t, c)
	assert.Equal(t, int64(0), c.Current())

	// Clock is usable for stamping events
	seq := c.Next()
	assert.Equal(t, int64(1), seq)
}

func TestEngine_NewWithClock(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")

	// Start clock at 100 (for replay scenario)
	clock := NewClockAt(100)
	engine := NewWithClock(s, nil, nil, flowGen, clock)

	c := engine.Clock()
	assert.Equal(t, int64(100), c.Current())
	assert.Equal(t, int64(101), c.Next())
}

func TestEngine_SyncsPreservedInOrder(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")

	syncs := []ir.SyncRule{
		{ID: "sync-first"},
		{ID: "sync-second"},
		{ID: "sync-third"},
	}

	engine := New(s, nil, syncs, flowGen)

	// Verify syncs are stored in declaration order
	assert.Equal(t, 3, len(engine.Syncs()))
	assert.Equal(t, "sync-first", engine.Syncs()[0].ID)
	assert.Equal(t, "sync-second", engine.Syncs()[1].ID)
	assert.Equal(t, "sync-third", engine.Syncs()[2].ID)
}

// Story 3.2: RegisterSyncs tests

func TestRegisterSyncs_Empty(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	t.Run("nil_slice", func(t *testing.T) {
		err := engine.RegisterSyncs(nil)
		require.NoError(t, err)
		assert.Nil(t, engine.Syncs())
	})

	t.Run("empty_slice", func(t *testing.T) {
		err := engine.RegisterSyncs([]ir.SyncRule{})
		require.NoError(t, err)
		assert.NotNil(t, engine.Syncs())
		assert.Empty(t, engine.Syncs())
	})
}

func TestRegisterSyncs_Order(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	syncs := []ir.SyncRule{
		{ID: "sync-1"},
		{ID: "sync-2"},
		{ID: "sync-3"},
	}

	err := engine.RegisterSyncs(syncs)
	require.NoError(t, err)

	// Verify order is preserved
	require.Len(t, engine.Syncs(), 3)
	assert.Equal(t, "sync-1", engine.Syncs()[0].ID)
	assert.Equal(t, "sync-2", engine.Syncs()[1].ID)
	assert.Equal(t, "sync-3", engine.Syncs()[2].ID)
}

func TestRegisterSyncs_DuplicateID(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	syncs := []ir.SyncRule{
		{ID: "sync-1"},
		{ID: "sync-2"},
		{ID: "sync-1"}, // Duplicate
	}

	err := engine.RegisterSyncs(syncs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate sync ID: sync-1")
}

func TestRegisterSyncs_CopyPreventsExternalMutation(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	syncs := []ir.SyncRule{
		{ID: "sync-1"},
		{ID: "sync-2"},
	}

	err := engine.RegisterSyncs(syncs)
	require.NoError(t, err)

	// Mutate original slice
	syncs[0].ID = "mutated"

	// Engine should be unaffected
	assert.Equal(t, "sync-1", engine.Syncs()[0].ID, "engine syncs should be independent copy")
}

func TestRegisterSyncs_ReplacesPreviousRegistration(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	// First registration
	syncs1 := []ir.SyncRule{
		{ID: "sync-1"},
	}
	err := engine.RegisterSyncs(syncs1)
	require.NoError(t, err)
	assert.Len(t, engine.Syncs(), 1)

	// Second registration replaces
	syncs2 := []ir.SyncRule{
		{ID: "sync-2"},
		{ID: "sync-3"},
	}
	err = engine.RegisterSyncs(syncs2)
	require.NoError(t, err)
	assert.Len(t, engine.Syncs(), 2)
	assert.Equal(t, "sync-2", engine.Syncs()[0].ID)
}

func TestEvaluateSyncs_WithMatchingSyncRule(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")

	// Register a sync rule that matches Cart.addItem -> Success
	syncs := []ir.SyncRule{
		{
			ID: "sync-cart-to-inventory",
			When: ir.WhenClause{
				ActionRef:  "Cart.addItem",
				EventType:  "completed",
				OutputCase: "Success",
				Bindings: map[string]string{
					"cart_id": "cart_id",
				},
			},
			Then: ir.ThenClause{
				ActionRef: "Inventory.reserve",
				Args: map[string]string{
					"cart_id": "${bound.cart_id}",
				},
			},
		},
	}

	engine := New(s, nil, syncs, flowGen)

	ctx := context.Background()

	// Write an invocation
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID("flow-1", "Cart.addItem", args, 1)
	inv := ir.Invocation{
		ID:        invID,
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
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

	// Create completion that matches the sync rule
	result := ir.IRObject{
		"cart_id": ir.IRString("cart-123"),
	}
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
	// Must write completion first so FK constraint passes for sync firing
	require.NoError(t, s.WriteCompletion(ctx, *comp))

	// Evaluate syncs - should fire and generate new invocation
	err := engine.evaluateSyncs(ctx, comp)
	require.NoError(t, err)

	// Verify sync fired - check that a sync firing was recorded
	firings, err := s.ReadSyncFiringsForCompletion(ctx, compID)
	require.NoError(t, err)
	require.Len(t, firings, 1, "expected one sync firing")
	assert.Equal(t, "sync-cart-to-inventory", firings[0].SyncID)

	// Verify provenance edge was created
	edges, err := s.ReadAllProvenanceEdges(ctx)
	require.NoError(t, err)
	require.Len(t, edges, 1, "expected one provenance edge")
	assert.Equal(t, firings[0].ID, edges[0].SyncFiringID)

	// Verify generated invocation has correct flow token (inherited)
	generatedInv, err := s.ReadInvocation(ctx, edges[0].InvocationID)
	require.NoError(t, err)
	assert.Equal(t, "flow-1", generatedInv.FlowToken, "flow token must be inherited")
	assert.Equal(t, ir.ActionRef("Inventory.reserve"), generatedInv.ActionURI)
	assert.Equal(t, ir.IRString("cart-123"), generatedInv.Args["cart_id"])
}

func TestEvaluateSyncs_NoSyncs(t *testing.T) {
	s := setupTestStore(t)
	flowGen := newStubFlowGen("flow-1")
	engine := New(s, nil, nil, flowGen)

	ctx := context.Background()

	// First write an invocation (evaluateSyncs needs to look it up)
	args := ir.IRObject{"item": ir.IRString("widget")}
	invID := ir.MustInvocationID("flow-1", "Cart.addItem", args, 1)
	inv := ir.Invocation{
		ID:        invID,
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
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

	comp := &ir.Completion{
		ID:           "comp-1",
		InvocationID: invID, // Use the real invocation ID
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}

	err := engine.evaluateSyncs(ctx, comp)
	require.NoError(t, err)
}

func TestMatchWhen_Integration(t *testing.T) {
	// This test verifies the matchWhen function is accessible and works
	// Detailed tests are in matcher_test.go

	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "completed",
		OutputCase: "Success",
	}

	inv := &ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.checkout",
		Args:      ir.IRObject{},
	}

	comp := &ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}

	// Should match when action, event type, and output case all match
	matched := matchWhen(when, inv, comp)
	assert.True(t, matched, "should match when all conditions satisfied")
}

// TestDeclarationOrderDeterminism verifies that the same input order
// produces the same evaluation order (determinism guarantee)
func TestDeclarationOrderDeterminism(t *testing.T) {
	s1 := setupTestStore(t)
	s2 := setupTestStore(t)

	syncs := []ir.SyncRule{
		{ID: "alpha"},
		{ID: "beta"},
		{ID: "gamma"},
	}

	// Create two engines with same syncs
	e1 := New(s1, nil, nil, newStubFlowGen("flow-1"))
	e2 := New(s2, nil, nil, newStubFlowGen("flow-1"))

	err := e1.RegisterSyncs(syncs)
	require.NoError(t, err)

	err = e2.RegisterSyncs(syncs)
	require.NoError(t, err)

	// Both engines should have identical order
	require.Equal(t, len(e1.Syncs()), len(e2.Syncs()))
	for i := range e1.Syncs() {
		assert.Equal(t, e1.Syncs()[i].ID, e2.Syncs()[i].ID,
			"engines should have identical sync order at index %d", i)
	}
}
