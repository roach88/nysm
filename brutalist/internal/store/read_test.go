package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestReadFlow_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	invocations, completions, err := s.ReadFlow(context.Background(), "nonexistent-flow")
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	// Should return empty slices, not nil
	if invocations == nil {
		t.Error("invocations is nil, want empty slice")
	}
	if completions == nil {
		t.Error("completions is nil, want empty slice")
	}
	if len(invocations) != 0 {
		t.Errorf("len(invocations) = %d, want 0", len(invocations))
	}
	if len(completions) != 0 {
		t.Errorf("len(completions) = %d, want 0", len(completions))
	}
}

func TestReadFlow_SingleInvocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	flowToken := "flow-abc"
	inv := ir.Invocation{
		ID:        "inv-123",
		FlowToken: flowToken,
		ActionURI: ir.ActionRef("Cart.addItem"),
		Args: ir.IRObject{
			"item_id": ir.IRString("widget"),
		},
		Seq: 1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
		SpecHash:      "hash-abc",
		EngineVersion: "0.1.0",
		IRVersion:     "1",
	}
	s.WriteInvocation(context.Background(), inv)

	invocations, completions, err := s.ReadFlow(context.Background(), flowToken)
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(invocations) != 1 {
		t.Fatalf("len(invocations) = %d, want 1", len(invocations))
	}
	if len(completions) != 0 {
		t.Errorf("len(completions) = %d, want 0", len(completions))
	}

	// Verify invocation fields
	got := invocations[0]
	if got.ID != inv.ID {
		t.Errorf("ID = %q, want %q", got.ID, inv.ID)
	}
	if got.FlowToken != inv.FlowToken {
		t.Errorf("FlowToken = %q, want %q", got.FlowToken, inv.FlowToken)
	}
	if got.ActionURI != inv.ActionURI {
		t.Errorf("ActionURI = %q, want %q", got.ActionURI, inv.ActionURI)
	}
	if got.Seq != inv.Seq {
		t.Errorf("Seq = %d, want %d", got.Seq, inv.Seq)
	}
	if got.SecurityContext.TenantID != inv.SecurityContext.TenantID {
		t.Errorf("SecurityContext.TenantID = %q, want %q", got.SecurityContext.TenantID, inv.SecurityContext.TenantID)
	}
}

func TestReadFlow_InvocationWithCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	flowToken := "flow-abc"

	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       flowToken,
		ActionURI:       ir.ActionRef("Cart.addItem"),
		Args:            ir.IRObject{},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}
	s.WriteInvocation(context.Background(), inv)

	comp := ir.Completion{
		ID:           "comp-456",
		InvocationID: "inv-123",
		OutputCase:   "Success",
		Result: ir.IRObject{
			"status": ir.IRString("ok"),
		},
		Seq: 2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
		},
	}
	s.WriteCompletion(context.Background(), comp)

	invocations, completions, err := s.ReadFlow(context.Background(), flowToken)
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(invocations) != 1 {
		t.Fatalf("len(invocations) = %d, want 1", len(invocations))
	}
	if len(completions) != 1 {
		t.Fatalf("len(completions) = %d, want 1", len(completions))
	}

	// Verify completion fields
	gotComp := completions[0]
	if gotComp.ID != comp.ID {
		t.Errorf("completion ID = %q, want %q", gotComp.ID, comp.ID)
	}
	if gotComp.InvocationID != comp.InvocationID {
		t.Errorf("InvocationID = %q, want %q", gotComp.InvocationID, comp.InvocationID)
	}
	if gotComp.OutputCase != comp.OutputCase {
		t.Errorf("OutputCase = %q, want %q", gotComp.OutputCase, comp.OutputCase)
	}
	if gotComp.Seq != comp.Seq {
		t.Errorf("Seq = %d, want %d", gotComp.Seq, comp.Seq)
	}
}

func TestReadFlow_DeterministicOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	flowToken := "flow-abc"

	// Write invocations in non-sequential order
	seqs := []int64{5, 1, 3, 2, 4}
	for _, seq := range seqs {
		inv := ir.Invocation{
			ID:              fmt.Sprintf("inv-%d", seq),
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             seq,
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv)
	}

	invocations, _, err := s.ReadFlow(context.Background(), flowToken)
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(invocations) != 5 {
		t.Fatalf("len(invocations) = %d, want 5", len(invocations))
	}

	// Verify ordering is deterministic (seq ASC)
	for i, inv := range invocations {
		expectedSeq := int64(i + 1)
		if inv.Seq != expectedSeq {
			t.Errorf("invocations[%d].Seq = %d, want %d (deterministic ordering)", i, inv.Seq, expectedSeq)
		}
	}
}

func TestReadFlow_DeterministicOrderingWithSameSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	flowToken := "flow-abc"

	// Write invocations with same seq but different IDs
	ids := []string{"inv-z", "inv-a", "inv-m"}
	for _, id := range ids {
		inv := ir.Invocation{
			ID:              id,
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             1, // Same seq for all
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv)
	}

	invocations, _, err := s.ReadFlow(context.Background(), flowToken)
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(invocations) != 3 {
		t.Fatalf("len(invocations) = %d, want 3", len(invocations))
	}

	// Verify secondary ordering is by id ASC COLLATE BINARY
	expectedOrder := []string{"inv-a", "inv-m", "inv-z"}
	for i, inv := range invocations {
		if inv.ID != expectedOrder[i] {
			t.Errorf("invocations[%d].ID = %q, want %q (id ASC tiebreaker)", i, inv.ID, expectedOrder[i])
		}
	}
}

func TestReadFlow_CompletionOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	flowToken := "flow-abc"

	// Write invocations and completions
	for i := 1; i <= 3; i++ {
		inv := ir.Invocation{
			ID:              fmt.Sprintf("inv-%d", i),
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             int64(i * 2 - 1), // 1, 3, 5
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv)

		comp := ir.Completion{
			ID:              fmt.Sprintf("comp-%d", i),
			InvocationID:    fmt.Sprintf("inv-%d", i),
			OutputCase:      "Success",
			Result:          ir.IRObject{},
			Seq:             int64(i * 2), // 2, 4, 6
			SecurityContext: ir.SecurityContext{},
		}
		s.WriteCompletion(context.Background(), comp)
	}

	_, completions, err := s.ReadFlow(context.Background(), flowToken)
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(completions) != 3 {
		t.Fatalf("len(completions) = %d, want 3", len(completions))
	}

	// Verify ordering by seq ASC
	expectedSeqs := []int64{2, 4, 6}
	for i, comp := range completions {
		if comp.Seq != expectedSeqs[i] {
			t.Errorf("completions[%d].Seq = %d, want %d", i, comp.Seq, expectedSeqs[i])
		}
	}
}

func TestReadFlow_MultipleFlows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocations for two different flows
	for i := 1; i <= 3; i++ {
		inv1 := ir.Invocation{
			ID:              fmt.Sprintf("inv-flow1-%d", i),
			FlowToken:       "flow-1",
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             int64(i),
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv1)

		inv2 := ir.Invocation{
			ID:              fmt.Sprintf("inv-flow2-%d", i),
			FlowToken:       "flow-2",
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             int64(i + 10),
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv2)
	}

	// Read flow-1
	invocations1, _, err := s.ReadFlow(context.Background(), "flow-1")
	if err != nil {
		t.Fatalf("ReadFlow(flow-1) failed: %v", err)
	}
	if len(invocations1) != 3 {
		t.Errorf("flow-1 has %d invocations, want 3", len(invocations1))
	}

	// Read flow-2
	invocations2, _, err := s.ReadFlow(context.Background(), "flow-2")
	if err != nil {
		t.Fatalf("ReadFlow(flow-2) failed: %v", err)
	}
	if len(invocations2) != 3 {
		t.Errorf("flow-2 has %d invocations, want 3", len(invocations2))
	}

	// Verify flow isolation
	for _, inv := range invocations1 {
		if inv.FlowToken != "flow-1" {
			t.Errorf("flow-1 returned invocation with FlowToken %q", inv.FlowToken)
		}
	}
	for _, inv := range invocations2 {
		if inv.FlowToken != "flow-2" {
			t.Errorf("flow-2 returned invocation with FlowToken %q", inv.FlowToken)
		}
	}
}

func TestReadInvocation_Exists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	inv := ir.Invocation{
		ID:        "inv-123",
		FlowToken: "flow-abc",
		ActionURI: ir.ActionRef("Cart.addItem"),
		Args: ir.IRObject{
			"item": ir.IRString("widget"),
		},
		Seq: 1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
		},
		SpecHash:      "hash",
		EngineVersion: "0.1.0",
		IRVersion:     "1",
	}
	s.WriteInvocation(context.Background(), inv)

	got, err := s.ReadInvocation(context.Background(), "inv-123")
	if err != nil {
		t.Fatalf("ReadInvocation() failed: %v", err)
	}

	if got.ID != inv.ID {
		t.Errorf("ID = %q, want %q", got.ID, inv.ID)
	}
	if got.FlowToken != inv.FlowToken {
		t.Errorf("FlowToken = %q, want %q", got.FlowToken, inv.FlowToken)
	}
	if got.SecurityContext.TenantID != inv.SecurityContext.TenantID {
		t.Errorf("SecurityContext.TenantID = %q, want %q", got.SecurityContext.TenantID, inv.SecurityContext.TenantID)
	}

	// Verify args
	item, ok := got.Args["item"].(ir.IRString)
	if !ok || item != "widget" {
		t.Errorf("Args[item] = %v, want widget", got.Args["item"])
	}
}

func TestReadInvocation_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	_, err = s.ReadInvocation(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("ReadInvocation() error = %v, want sql.ErrNoRows", err)
	}
}

func TestReadCompletion_Exists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       "flow-abc",
		ActionURI:       ir.ActionRef("Test.action"),
		Args:            ir.IRObject{},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}
	s.WriteInvocation(context.Background(), inv)

	comp := ir.Completion{
		ID:           "comp-456",
		InvocationID: "inv-123",
		OutputCase:   "Success",
		Result: ir.IRObject{
			"status": ir.IRString("ok"),
		},
		Seq: 2,
		SecurityContext: ir.SecurityContext{
			UserID: "user-1",
		},
	}
	s.WriteCompletion(context.Background(), comp)

	got, err := s.ReadCompletion(context.Background(), "comp-456")
	if err != nil {
		t.Fatalf("ReadCompletion() failed: %v", err)
	}

	if got.ID != comp.ID {
		t.Errorf("ID = %q, want %q", got.ID, comp.ID)
	}
	if got.InvocationID != comp.InvocationID {
		t.Errorf("InvocationID = %q, want %q", got.InvocationID, comp.InvocationID)
	}
	if got.OutputCase != comp.OutputCase {
		t.Errorf("OutputCase = %q, want %q", got.OutputCase, comp.OutputCase)
	}

	// Verify result
	status, ok := got.Result["status"].(ir.IRString)
	if !ok || status != "ok" {
		t.Errorf("Result[status] = %v, want ok", got.Result["status"])
	}
}

func TestReadCompletion_NotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	_, err = s.ReadCompletion(context.Background(), "nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("ReadCompletion() error = %v, want sql.ErrNoRows", err)
	}
}

func TestReadAllInvocations_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	invocations, err := s.ReadAllInvocations(context.Background())
	if err != nil {
		t.Fatalf("ReadAllInvocations() failed: %v", err)
	}

	if invocations == nil {
		t.Error("invocations is nil, want empty slice")
	}
	if len(invocations) != 0 {
		t.Errorf("len(invocations) = %d, want 0", len(invocations))
	}
}

func TestReadAllInvocations_DeterministicOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocations in non-sequential order across multiple flows
	seqs := []int64{5, 1, 3, 2, 4}
	for _, seq := range seqs {
		inv := ir.Invocation{
			ID:              fmt.Sprintf("inv-%d", seq),
			FlowToken:       fmt.Sprintf("flow-%d", seq%2),
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             seq,
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv)
	}

	invocations, err := s.ReadAllInvocations(context.Background())
	if err != nil {
		t.Fatalf("ReadAllInvocations() failed: %v", err)
	}

	if len(invocations) != 5 {
		t.Fatalf("len(invocations) = %d, want 5", len(invocations))
	}

	// Verify ordering is deterministic (seq ASC)
	for i, inv := range invocations {
		expectedSeq := int64(i + 1)
		if inv.Seq != expectedSeq {
			t.Errorf("invocations[%d].Seq = %d, want %d (deterministic ordering)", i, inv.Seq, expectedSeq)
		}
	}
}

func TestReadAllCompletions_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	completions, err := s.ReadAllCompletions(context.Background())
	if err != nil {
		t.Fatalf("ReadAllCompletions() failed: %v", err)
	}

	if completions == nil {
		t.Error("completions is nil, want empty slice")
	}
	if len(completions) != 0 {
		t.Errorf("len(completions) = %d, want 0", len(completions))
	}
}

func TestReadAllCompletions_DeterministicOrdering(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocations and completions in non-sequential order
	seqs := []int64{6, 2, 4}
	for i, seq := range seqs {
		inv := ir.Invocation{
			ID:              fmt.Sprintf("inv-%d", i+1),
			FlowToken:       "flow-1",
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{},
			Seq:             seq - 1,
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		s.WriteInvocation(context.Background(), inv)

		comp := ir.Completion{
			ID:              fmt.Sprintf("comp-%d", i+1),
			InvocationID:    fmt.Sprintf("inv-%d", i+1),
			OutputCase:      "Success",
			Result:          ir.IRObject{},
			Seq:             seq,
			SecurityContext: ir.SecurityContext{},
		}
		s.WriteCompletion(context.Background(), comp)
	}

	completions, err := s.ReadAllCompletions(context.Background())
	if err != nil {
		t.Fatalf("ReadAllCompletions() failed: %v", err)
	}

	if len(completions) != 3 {
		t.Fatalf("len(completions) = %d, want 3", len(completions))
	}

	// Verify ordering is deterministic (seq ASC)
	expectedSeqs := []int64{2, 4, 6}
	for i, comp := range completions {
		if comp.Seq != expectedSeqs[i] {
			t.Errorf("completions[%d].Seq = %d, want %d", i, comp.Seq, expectedSeqs[i])
		}
	}
}

func TestReadFlow_ArgsUnmarshaling(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	inv := ir.Invocation{
		ID:        "inv-123",
		FlowToken: "flow-abc",
		ActionURI: ir.ActionRef("Test.action"),
		Args: ir.IRObject{
			"string":  ir.IRString("hello"),
			"int":     ir.IRInt(42),
			"bool":    ir.IRBool(true),
			"array":   ir.IRArray{ir.IRInt(1), ir.IRInt(2), ir.IRInt(3)},
			"nested":  ir.IRObject{"inner": ir.IRString("value")},
		},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}
	s.WriteInvocation(context.Background(), inv)

	invocations, _, err := s.ReadFlow(context.Background(), "flow-abc")
	if err != nil {
		t.Fatalf("ReadFlow() failed: %v", err)
	}

	if len(invocations) != 1 {
		t.Fatalf("len(invocations) = %d, want 1", len(invocations))
	}

	args := invocations[0].Args

	// Verify string
	str, ok := args["string"].(ir.IRString)
	if !ok || str != "hello" {
		t.Errorf("args[string] = %v, want hello", args["string"])
	}

	// Verify int
	intVal, ok := args["int"].(ir.IRInt)
	if !ok || intVal != 42 {
		t.Errorf("args[int] = %v, want 42", args["int"])
	}

	// Verify bool
	boolVal, ok := args["bool"].(ir.IRBool)
	if !ok || boolVal != true {
		t.Errorf("args[bool] = %v, want true", args["bool"])
	}

	// Verify array
	arr, ok := args["array"].(ir.IRArray)
	if !ok || len(arr) != 3 {
		t.Errorf("args[array] = %v, want array of 3", args["array"])
	}

	// Verify nested
	nested, ok := args["nested"].(ir.IRObject)
	if !ok {
		t.Errorf("args[nested] = %T, want IRObject", args["nested"])
	} else {
		inner, ok := nested["inner"].(ir.IRString)
		if !ok || inner != "value" {
			t.Errorf("nested[inner] = %v, want value", nested["inner"])
		}
	}
}
