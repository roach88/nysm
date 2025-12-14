package store

import (
	"path/filepath"
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

// createTestStore creates a new in-memory store for testing.
func createTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// createTestInvocation creates a test invocation with minimal required fields.
func createTestInvocation(id, flowToken, actionURI string, seq int64) ir.Invocation {
	return ir.Invocation{
		ID:              id,
		FlowToken:       flowToken,
		ActionURI:       ir.ActionRef(actionURI),
		Args:            ir.IRObject{},
		Seq:             seq,
		SecurityContext: ir.SecurityContext{},
		SpecHash:        "test-hash",
		EngineVersion:   "0.1.0",
		IRVersion:       "1",
	}
}

// createTestCompletion creates a test completion with minimal required fields.
func createTestCompletion(id, invocationID, outputCase string, seq int64) ir.Completion {
	return ir.Completion{
		ID:              id,
		InvocationID:    invocationID,
		OutputCase:      outputCase,
		Result:          ir.IRObject{},
		Seq:             seq,
		SecurityContext: ir.SecurityContext{},
	}
}
