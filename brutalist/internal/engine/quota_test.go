package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// TestQuotaEnforcer_WithinLimit tests normal operation within quota.
func TestQuotaEnforcer_WithinLimit(t *testing.T) {
	q := NewQuotaEnforcer(10)

	// Should allow 10 checks
	for i := 0; i < 10; i++ {
		err := q.Check("flow-1")
		assert.NoError(t, err, "step %d should be allowed", i+1)
	}

	assert.Equal(t, 10, q.Current())
	assert.Equal(t, 10, q.MaxSteps())
}

// TestQuotaEnforcer_ExceedsLimit tests quota exceeded error.
func TestQuotaEnforcer_ExceedsLimit(t *testing.T) {
	q := NewQuotaEnforcer(5)

	// First 5 should pass
	for i := 0; i < 5; i++ {
		err := q.Check("flow-1")
		require.NoError(t, err)
	}

	// 6th should fail
	err := q.Check("flow-1")
	require.Error(t, err)

	// Verify error type
	var stepsErr *StepsExceededError
	require.ErrorAs(t, err, &stepsErr)
	assert.Equal(t, "flow-1", stepsErr.FlowToken)
	assert.Equal(t, 6, stepsErr.Steps)
	assert.Equal(t, 5, stepsErr.Limit)
}

// TestQuotaEnforcer_Reset tests resetting the counter.
func TestQuotaEnforcer_Reset(t *testing.T) {
	q := NewQuotaEnforcer(5)

	// Use up quota
	for i := 0; i < 5; i++ {
		q.Check("flow-1")
	}
	assert.Equal(t, 5, q.Current())

	// Reset
	q.Reset()
	assert.Equal(t, 0, q.Current())

	// Should allow 5 more
	for i := 0; i < 5; i++ {
		err := q.Check("flow-1")
		assert.NoError(t, err)
	}
}

// TestStepsExceededError_Error tests error message formatting.
func TestStepsExceededError_Error(t *testing.T) {
	err := &StepsExceededError{
		FlowToken: "flow-abc",
		Steps:     1001,
		Limit:     1000,
	}

	msg := err.Error()
	assert.Contains(t, msg, "flow-abc")
	assert.Contains(t, msg, "1001")
	assert.Contains(t, msg, "1000")
}

// TestStepsExceededError_RuntimeError tests error type method.
func TestStepsExceededError_RuntimeError(t *testing.T) {
	err := &StepsExceededError{
		FlowToken: "flow-1",
		Steps:     100,
		Limit:     50,
	}

	assert.Equal(t, "StepsExceededError", err.RuntimeError())
}

// TestIsStepsExceededError tests error type checking.
func TestIsStepsExceededError(t *testing.T) {
	stepsErr := &StepsExceededError{FlowToken: "flow-1", Steps: 10, Limit: 5}

	assert.True(t, IsStepsExceededError(stepsErr))
	assert.False(t, IsStepsExceededError(nil))
	assert.False(t, IsStepsExceededError(assert.AnError))
}

// TestEngine_WithMaxSteps tests custom max steps option.
func TestEngine_WithMaxSteps(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	// Default
	e1 := New(s, nil, nil, nil)
	assert.Equal(t, DefaultMaxSteps, e1.MaxSteps())

	// Custom
	e2 := New(s, nil, nil, nil, WithMaxSteps(500))
	assert.Equal(t, 500, e2.MaxSteps())
}

// TestEngine_QuotaPerFlow tests independent quotas per flow.
func TestEngine_QuotaPerFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil, WithMaxSteps(10))

	// Get quotas for different flows
	q1 := e.QuotaFor("flow-1")
	q2 := e.QuotaFor("flow-2")

	// They should be independent
	assert.NotSame(t, q1, q2)

	// Use up flow-1's quota
	for i := 0; i < 10; i++ {
		q1.Check("flow-1")
	}
	assert.Equal(t, 10, q1.Current())
	assert.Equal(t, 0, q2.Current())

	// flow-2 should still work
	err = q2.Check("flow-2")
	assert.NoError(t, err)
}

// TestEngine_QuotaFor_SameFlow tests same flow returns same quota.
func TestEngine_QuotaFor_SameFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)

	q1 := e.QuotaFor("flow-1")
	q2 := e.QuotaFor("flow-1")

	assert.Same(t, q1, q2)
}

// TestEngine_CleanupFlow tests quota cleanup.
func TestEngine_CleanupFlow(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)

	// Create quota
	q1 := e.QuotaFor("flow-1")
	q1.Check("flow-1")
	assert.Equal(t, 1, q1.Current())
	assert.Equal(t, 1, e.QuotaCount())

	// Cleanup
	e.CleanupFlow("flow-1")
	assert.Equal(t, 0, e.QuotaCount())

	// New quota should be fresh
	q2 := e.QuotaFor("flow-1")
	assert.Equal(t, 0, q2.Current())
	assert.NotSame(t, q1, q2)
}

// setupQuotaTestEngine creates an engine with store and test data for quota tests.
func setupQuotaTestEngine(t *testing.T, maxSteps int) (*Engine, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil, WithMaxSteps(maxSteps))
	return e, s
}

// TestQuota_EnforcedInProcessCompletion tests quota enforcement during completion processing.
func TestQuota_EnforcedInProcessCompletion(t *testing.T) {
	e, s := setupQuotaTestEngine(t, 3)
	ctx := context.Background()

	// Create invocation
	inv := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Test.action",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv))

	// Process completions up to limit
	for i := 0; i < 3; i++ {
		compID := fmt.Sprintf("comp-%d", i)
		comp := ir.Completion{
			ID:           compID,
			InvocationID: "inv-1",
			OutputCase:   "Success",
			Result:       ir.IRObject{},
			Seq:          int64(101 + i),
		}
		require.NoError(t, s.WriteCompletion(ctx, comp))

		err := e.processCompletion(ctx, &comp)
		require.NoError(t, err, "completion %d should succeed", i)
	}

	// 4th completion should fail due to quota
	comp4 := ir.Completion{
		ID:           "comp-4",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          104,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp4))

	err := e.processCompletion(ctx, &comp4)
	require.Error(t, err)
	assert.True(t, IsStepsExceededError(err))

	var stepsErr *StepsExceededError
	require.ErrorAs(t, err, &stepsErr)
	assert.Equal(t, "flow-1", stepsErr.FlowToken)
	assert.Equal(t, 4, stepsErr.Steps)
	assert.Equal(t, 3, stepsErr.Limit)
}

// TestQuota_IndependentFlows tests that different flows have independent quotas.
func TestQuota_IndependentFlows(t *testing.T) {
	e, s := setupQuotaTestEngine(t, 2)
	ctx := context.Background()

	// Create invocations for two flows
	inv1 := ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Test.action",
		Args:      ir.IRObject{},
		Seq:       100,
	}
	inv2 := ir.Invocation{
		ID:        "inv-2",
		FlowToken: "flow-2",
		ActionURI: "Test.action",
		Args:      ir.IRObject{},
		Seq:       200,
	}
	require.NoError(t, s.WriteInvocation(ctx, inv1))
	require.NoError(t, s.WriteInvocation(ctx, inv2))

	// Use up flow-1's quota
	for i := 0; i < 2; i++ {
		compID := fmt.Sprintf("comp-flow1-%d", i)
		comp := ir.Completion{
			ID:           compID,
			InvocationID: "inv-1",
			OutputCase:   "Success",
			Result:       ir.IRObject{},
			Seq:          int64(101 + i),
		}
		require.NoError(t, s.WriteCompletion(ctx, comp))
		err := e.processCompletion(ctx, &comp)
		require.NoError(t, err)
	}

	// flow-1 should be at quota
	assert.Equal(t, 2, e.QuotaFor("flow-1").Current())

	// flow-2 should still work
	comp := ir.Completion{
		ID:           "comp-flow2-1",
		InvocationID: "inv-2",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          201,
	}
	require.NoError(t, s.WriteCompletion(ctx, comp))
	err := e.processCompletion(ctx, &comp)
	require.NoError(t, err)
	assert.Equal(t, 1, e.QuotaFor("flow-2").Current())
}

// TestQuota_HighLimit tests large quota limit.
func TestQuota_HighLimit(t *testing.T) {
	q := NewQuotaEnforcer(10000)

	// Should allow many checks
	for i := 0; i < 10000; i++ {
		err := q.Check("flow-1")
		require.NoError(t, err)
	}

	// 10001st should fail
	err := q.Check("flow-1")
	require.Error(t, err)
	assert.True(t, IsStepsExceededError(err))
}

// TestQuota_ZeroLimit tests zero quota (immediate failure).
func TestQuota_ZeroLimit(t *testing.T) {
	q := NewQuotaEnforcer(0)

	// First check should fail (0 > 0 is false, but 1 > 0 is true)
	err := q.Check("flow-1")
	require.Error(t, err)
	assert.True(t, IsStepsExceededError(err))
}

// TestQuota_SingleStep tests quota of 1.
func TestQuota_SingleStep(t *testing.T) {
	q := NewQuotaEnforcer(1)

	// First should pass
	err := q.Check("flow-1")
	require.NoError(t, err)

	// Second should fail
	err = q.Check("flow-1")
	require.Error(t, err)
	assert.True(t, IsStepsExceededError(err))
}

// TestQuota_ErrorMatchesIsQuotaError tests IsQuotaError helper.
func TestQuota_ErrorMatchesIsQuotaError(t *testing.T) {
	q := NewQuotaEnforcer(1)
	q.Check("flow-1")
	err := q.Check("flow-1")

	require.Error(t, err)
	assert.True(t, IsQuotaError(err), "StepsExceededError should match IsQuotaError")
}

// TestEngine_QuotaCount tests counting active quotas.
func TestEngine_QuotaCount(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Open(dir + "/test.db")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })

	e := New(s, nil, nil, nil)
	assert.Equal(t, 0, e.QuotaCount())

	e.QuotaFor("flow-1")
	assert.Equal(t, 1, e.QuotaCount())

	e.QuotaFor("flow-2")
	assert.Equal(t, 2, e.QuotaCount())

	e.QuotaFor("flow-1") // Same flow
	assert.Equal(t, 2, e.QuotaCount())

	e.CleanupFlow("flow-1")
	assert.Equal(t, 1, e.QuotaCount())
}

// TestQuota_DefaultMaxSteps tests default constant value.
func TestQuota_DefaultMaxSteps(t *testing.T) {
	assert.Equal(t, 1000, DefaultMaxSteps, "default max steps should be 1000")
}
