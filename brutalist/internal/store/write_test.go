package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestWriteInvocation_Basic(t *testing.T) {
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
			"item_id":  ir.IRString("widget"),
			"quantity": ir.IRInt(3),
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

	err = s.WriteInvocation(context.Background(), inv)
	if err != nil {
		t.Fatalf("WriteInvocation() failed: %v", err)
	}

	// Verify stored correctly
	var storedID, flowToken, actionURI, argsJSON string
	var seq int64
	err = s.db.QueryRow(`
		SELECT id, flow_token, action_uri, args, seq
		FROM invocations
		WHERE id = ?
	`, inv.ID).Scan(&storedID, &flowToken, &actionURI, &argsJSON, &seq)

	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if storedID != inv.ID {
		t.Errorf("id = %q, want %q", storedID, inv.ID)
	}
	if flowToken != inv.FlowToken {
		t.Errorf("flow_token = %q, want %q", flowToken, inv.FlowToken)
	}
	if actionURI != string(inv.ActionURI) {
		t.Errorf("action_uri = %q, want %q", actionURI, inv.ActionURI)
	}
	if seq != inv.Seq {
		t.Errorf("seq = %d, want %d", seq, inv.Seq)
	}
}

func TestWriteInvocation_CanonicalJSON(t *testing.T) {
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
			"zebra": ir.IRString("z"),
			"apple": ir.IRString("a"),
			"mango": ir.IRString("m"),
		},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}

	err = s.WriteInvocation(context.Background(), inv)
	if err != nil {
		t.Fatalf("WriteInvocation() failed: %v", err)
	}

	var argsJSON string
	err = s.db.QueryRow("SELECT args FROM invocations WHERE id = ?", inv.ID).Scan(&argsJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Canonical JSON should have keys sorted alphabetically
	expected := `{"apple":"a","mango":"m","zebra":"z"}`
	if argsJSON != expected {
		t.Errorf("args JSON = %q, want %q (canonical order)", argsJSON, expected)
	}
}

func TestWriteInvocation_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       "flow-abc",
		ActionURI:       ir.ActionRef("Cart.addItem"),
		Args:            ir.IRObject{},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}

	// Write twice - should not error
	err = s.WriteInvocation(context.Background(), inv)
	if err != nil {
		t.Fatalf("first WriteInvocation() failed: %v", err)
	}

	err = s.WriteInvocation(context.Background(), inv)
	if err != nil {
		t.Fatalf("second WriteInvocation() failed: %v", err)
	}

	// Verify only one row exists
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM invocations WHERE id = ?", inv.ID).Scan(&count)
	if count != 1 {
		t.Errorf("count = %d, want 1 (idempotent write)", count)
	}
}

func TestWriteInvocation_SecurityContext(t *testing.T) {
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
		Args:      ir.IRObject{},
		Seq:       1,
		SecurityContext: ir.SecurityContext{
			TenantID:    "tenant-xyz",
			UserID:      "user-abc",
			Permissions: []string{"read", "write", "admin"},
		},
		SpecHash:      "hash",
		EngineVersion: "0.1.0",
		IRVersion:     "1",
	}

	err = s.WriteInvocation(context.Background(), inv)
	if err != nil {
		t.Fatalf("WriteInvocation() failed: %v", err)
	}

	var secCtxJSON string
	err = s.db.QueryRow("SELECT security_context FROM invocations WHERE id = ?", inv.ID).Scan(&secCtxJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Verify security context is stored as canonical JSON
	expected := `{"permissions":["read","write","admin"],"tenant_id":"tenant-xyz","user_id":"user-abc"}`
	if secCtxJSON != expected {
		t.Errorf("security_context = %q, want %q", secCtxJSON, expected)
	}
}

func TestWriteCompletion_Basic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocation first (foreign key requirement)
	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       "flow-abc",
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
			"item_id":      ir.IRString("widget"),
			"new_quantity": ir.IRInt(3),
		},
		Seq: 2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}

	err = s.WriteCompletion(context.Background(), comp)
	if err != nil {
		t.Fatalf("WriteCompletion() failed: %v", err)
	}

	// Verify stored correctly
	var storedID, invocationID, outputCase, resultJSON string
	var seq int64
	err = s.db.QueryRow(`
		SELECT id, invocation_id, output_case, result, seq
		FROM completions
		WHERE id = ?
	`, comp.ID).Scan(&storedID, &invocationID, &outputCase, &resultJSON, &seq)

	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if storedID != comp.ID {
		t.Errorf("id = %q, want %q", storedID, comp.ID)
	}
	if invocationID != comp.InvocationID {
		t.Errorf("invocation_id = %q, want %q", invocationID, comp.InvocationID)
	}
	if outputCase != comp.OutputCase {
		t.Errorf("output_case = %q, want %q", outputCase, comp.OutputCase)
	}
	if seq != comp.Seq {
		t.Errorf("seq = %d, want %d", seq, comp.Seq)
	}
}

func TestWriteCompletion_ErrorOutputCase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocation first
	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       "flow-abc",
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
		OutputCase:   "ItemNotFound",
		Result: ir.IRObject{
			"error_code": ir.IRString("NOT_FOUND"),
			"message":    ir.IRString("Item does not exist"),
		},
		Seq:             2,
		SecurityContext: ir.SecurityContext{},
	}

	err = s.WriteCompletion(context.Background(), comp)
	if err != nil {
		t.Fatalf("WriteCompletion() failed: %v", err)
	}

	var outputCase string
	err = s.db.QueryRow("SELECT output_case FROM completions WHERE id = ?", comp.ID).Scan(&outputCase)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if outputCase != "ItemNotFound" {
		t.Errorf("output_case = %q, want %q", outputCase, "ItemNotFound")
	}
}

func TestWriteCompletion_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocation first
	inv := ir.Invocation{
		ID:              "inv-123",
		FlowToken:       "flow-abc",
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
		ID:              "comp-456",
		InvocationID:    "inv-123",
		OutputCase:      "Success",
		Result:          ir.IRObject{},
		Seq:             2,
		SecurityContext: ir.SecurityContext{},
	}

	// Write twice - should not error
	err = s.WriteCompletion(context.Background(), comp)
	if err != nil {
		t.Fatalf("first WriteCompletion() failed: %v", err)
	}

	err = s.WriteCompletion(context.Background(), comp)
	if err != nil {
		t.Fatalf("second WriteCompletion() failed: %v", err)
	}

	// Verify only one row exists
	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM completions WHERE id = ?", comp.ID).Scan(&count)
	if count != 1 {
		t.Errorf("count = %d, want 1 (idempotent write)", count)
	}
}

func TestWriteCompletion_ForeignKeyViolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Try to write completion without invocation
	comp := ir.Completion{
		ID:              "comp-456",
		InvocationID:    "nonexistent-inv",
		OutputCase:      "Success",
		Result:          ir.IRObject{},
		Seq:             1,
		SecurityContext: ir.SecurityContext{},
	}

	err = s.WriteCompletion(context.Background(), comp)
	if err == nil {
		t.Error("WriteCompletion() should fail with foreign key violation")
	}
}

func TestWriteCompletion_SecurityContext(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write invocation first
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
		Result:       ir.IRObject{},
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID:    "tenant-xyz",
			UserID:      "user-abc",
			Permissions: []string{"read"},
		},
	}

	err = s.WriteCompletion(context.Background(), comp)
	if err != nil {
		t.Fatalf("WriteCompletion() failed: %v", err)
	}

	var secCtxJSON string
	err = s.db.QueryRow("SELECT security_context FROM completions WHERE id = ?", comp.ID).Scan(&secCtxJSON)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	expected := `{"permissions":["read"],"tenant_id":"tenant-xyz","user_id":"user-abc"}`
	if secCtxJSON != expected {
		t.Errorf("security_context = %q, want %q", secCtxJSON, expected)
	}
}

func TestWriteMultipleInvocations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Write multiple invocations
	for i := 1; i <= 5; i++ {
		inv := ir.Invocation{
			ID:              "inv-" + string(rune('0'+i)),
			FlowToken:       "flow-abc",
			ActionURI:       ir.ActionRef("Test.action"),
			Args:            ir.IRObject{"index": ir.IRInt(int64(i))},
			Seq:             int64(i),
			SecurityContext: ir.SecurityContext{},
			SpecHash:        "hash",
			EngineVersion:   "0.1.0",
			IRVersion:       "1",
		}
		err := s.WriteInvocation(context.Background(), inv)
		if err != nil {
			t.Fatalf("WriteInvocation() %d failed: %v", i, err)
		}
	}

	var count int
	s.db.QueryRow("SELECT COUNT(*) FROM invocations").Scan(&count)
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}
