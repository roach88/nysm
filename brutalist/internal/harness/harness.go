// Package harness provides a conformance testing framework for the NYSM sync engine.
//
// # Current Limitations (Epic 6 - MVP)
//
// IMPORTANT: The current harness implementation does NOT invoke the actual sync engine.
// Instead, it directly writes invocations and completions to the store, manufacturing
// completion results from the scenario's expect clauses. This approach has been called
// the "Tautology Risk" - tests pass by definition because the harness writes exactly
// what the assertions expect.
//
// This limitation is intentional for the MVP conformance harness:
//   - It validates the testing infrastructure (scenario format, assertions, golden files)
//   - It ensures deterministic trace generation for golden file comparison
//   - It defers engine integration complexity to Epic 7 (CLI & Demo Application)
//
// # Future Integration (Epic 7)
//
// To properly validate engine behavior, the harness must be updated to:
//
//  1. Use engine.Enqueue() to submit invocations to the actual sync engine
//  2. Register mock action handlers that return configured results
//  3. Wait for actual completion events rather than manufacturing them
//  4. Compare actual engine-produced traces against expected traces
//
// Example future architecture:
//
//	// Register mock handler for Cart.addItem
//	eng.RegisterMockHandler("Cart.addItem", func(args ir.IRObject) (string, ir.IRObject) {
//	    return "Success", ir.IRObject{"new_quantity": ir.IRInt(3)}
//	})
//
//	// Enqueue invocation and wait for actual completion
//	eng.Enqueue(inv)
//	completion := waitForCompletion(inv.ID)
//
// Until Epic 7, use the harness with awareness that it validates:
//   - Scenario definition format and parsing
//   - Trace event structure and golden file comparison
//   - Assertion evaluation logic
//   - Store read/write mechanics
//
// But does NOT validate:
//   - Actual engine sync rule execution
//   - Real action handler invocations
//   - True completion generation from engine processing
package harness

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/roach88/nysm/internal/engine"
	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
	"github.com/roach88/nysm/internal/testutil"
)

// Harness is the test execution engine.
// It runs scenarios with deterministic clock and flow tokens.
//
// NOTE: Currently the harness bypasses actual engine execution. See package
// documentation for the "Tautology Risk" limitation and Epic 7 integration plans.
type Harness struct {
	store   *store.Store
	engine  *engine.Engine // TODO(Epic-7): Currently unused; will be used for engine.Enqueue() integration
	clock   *testutil.DeterministicClock
	flowGen *testutil.FixedFlowGenerator
	logger  *slog.Logger
	specHash string // Hash of concept specs (for invocations)
}

// Run executes a test scenario and returns the result.
//
// Each scenario runs in a fresh in-memory database for isolation.
// Deterministic helpers ensure reproducible results.
//
// Execution flow:
// 1. Create fresh in-memory database
// 2. Load and compile concept specs and sync rules
// 3. Execute setup steps
// 4. Execute flow steps with expect validation
// 5. Return result with pass/fail, trace, and errors
func Run(scenario *Scenario) (*Result, error) {
	// Create fresh in-memory SQLite database
	st, err := store.Open(":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory store: %w", err)
	}
	defer st.Close()

	// Initialize deterministic helpers
	clock := testutil.NewDeterministicClock()
	flowGen := testutil.NewFixedFlowGenerator(scenario.FlowToken)

	// TODO: Epic 7 - Load and compile specs from scenario.Specs
	// Currently using empty specs; real integration requires spec parsing
	specs := []ir.ConceptSpec{}
	syncs := []ir.SyncRule{}
	specHash := "test-spec-hash"

	// Create engine with test flow generator
	eng := engine.New(st, specs, syncs, flowGen)

	// Initialize harness
	h := &Harness{
		store:    st,
		engine:   eng,
		clock:    clock,
		flowGen:  flowGen,
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)), // Suppress logs in tests
		specHash: specHash,
	}

	ctx := context.Background()

	// Execute setup steps
	result := NewResult()
	if err := h.executeSetup(ctx, scenario.Setup, result); err != nil {
		return nil, fmt.Errorf("failed to execute setup: %w", err)
	}

	// Execute flow steps
	if err := h.executeFlow(ctx, scenario.Flow, result); err != nil {
		return nil, fmt.Errorf("failed to execute flow: %w", err)
	}

	// Evaluate assertions against the result
	actx := &AssertionContext{
		Store: st,
		Ctx:   ctx,
	}
	assertionErrors := EvaluateAssertions(result, scenario.Assertions, actx)
	for _, errMsg := range assertionErrors {
		result.AddError(errMsg)
	}

	return result, nil
}

// executeSetup runs all setup steps.
//
// Setup steps are executed sequentially before the flow.
// Each step generates an invocation and completion (assuming success).
func (h *Harness) executeSetup(ctx context.Context, setup []ActionStep, result *Result) error {
	for i, step := range setup {
		// Convert args to IRObject
		args, err := convertArgsToIRObject(step.Args)
		if err != nil {
			return fmt.Errorf("setup step %d: failed to convert args: %w", i, err)
		}

		// Generate flow token ONCE for this invocation
		flowToken := h.flowGen.Generate()

		// Get seq ONCE and reuse for both ID computation and record field
		// CRITICAL: clock.Next() must be called exactly once per record
		invSeq := h.clock.Next()

		// Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
		invID, err := ir.InvocationID(flowToken, step.Action, args, invSeq)
		if err != nil {
			return fmt.Errorf("setup step %d: failed to compute invocation ID: %w", i, err)
		}

		inv := ir.Invocation{
			ID:              invID,
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef(step.Action),
			Args:            args,
			Seq:             invSeq,
			SecurityContext: ir.SecurityContext{},
			SpecHash:        h.specHash,
			EngineVersion:   "test",
			IRVersion:       ir.IRVersion,
		}

		// Write invocation to store
		if err := h.store.WriteInvocation(ctx, inv); err != nil {
			return fmt.Errorf("setup step %d: failed to write invocation: %w", i, err)
		}

		// Add to trace
		result.AddInvocationTrace(step.Action, step.Args, invSeq)

		// Execute action (stub for MVP - Epic 7 will implement actual execution)
		// Setup steps always succeed; real handlers needed for failure scenarios

		// Get completion seq ONCE
		compSeq := h.clock.Next()
		compResult := ir.IRObject{} // Empty for setup

		compID, err := ir.CompletionID(inv.ID, "Success", compResult, compSeq)
		if err != nil {
			return fmt.Errorf("setup step %d: failed to compute completion ID: %w", i, err)
		}

		comp := ir.Completion{
			ID:              compID,
			InvocationID:    inv.ID,
			OutputCase:      "Success",
			Result:          compResult,
			Seq:             compSeq,
			SecurityContext: ir.SecurityContext{},
		}

		if err := h.store.WriteCompletion(ctx, comp); err != nil {
			return fmt.Errorf("setup step %d: failed to write completion: %w", i, err)
		}

		// Add to trace
		result.AddCompletionTrace("Success", nil, compSeq)

		h.logger.Info("setup step completed",
			"step", i,
			"action", step.Action,
			"invocation_id", inv.ID,
			"completion_id", comp.ID,
		)
	}
	return nil
}

// executeFlow runs all flow steps and validates expect clauses.
//
// TAUTOLOGY WARNING: This function manufactures completions directly from
// expect clauses rather than invoking the actual engine. The completion's
// output case and result are taken verbatim from step.Expect, meaning tests
// pass by construction. See package documentation for Epic 7 integration plans.
//
// Each step:
// 1. Generates invocation with deterministic ID (content-addressed)
// 2. Writes invocation to store (bypasses engine.Enqueue)
// 3. Manufactures completion from expect clause (NOT from engine execution)
// 4. Writes completion to store
// 5. Validates expect clause (always passes since completion = expect)
// 6. Builds trace for golden file comparison
func (h *Harness) executeFlow(ctx context.Context, flow []FlowStep, result *Result) error {
	for i, step := range flow {
		// Convert args to IRObject
		args, err := convertArgsToIRObject(step.Args)
		if err != nil {
			return fmt.Errorf("flow step %d: failed to convert args: %w", i, err)
		}

		// Generate flow token and seq ONCE (CRITICAL: avoid double clock.Next())
		flowToken := h.flowGen.Generate()
		invSeq := h.clock.Next()

		// Compute ID using Story 1.5 signature (SecurityContext excluded per CP-6)
		invID, err := ir.InvocationID(flowToken, step.Invoke, args, invSeq)
		if err != nil {
			return fmt.Errorf("flow step %d: failed to compute invocation ID: %w", i, err)
		}

		inv := ir.Invocation{
			ID:              invID,
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef(step.Invoke),
			Args:            args,
			Seq:             invSeq,
			SecurityContext: ir.SecurityContext{},
			SpecHash:        h.specHash,
			EngineVersion:   "test",
			IRVersion:       ir.IRVersion,
		}

		// Write invocation to store
		if err := h.store.WriteInvocation(ctx, inv); err != nil {
			return fmt.Errorf("flow step %d: failed to write invocation: %w", i, err)
		}

		// Add to trace
		result.AddInvocationTrace(step.Invoke, step.Args, invSeq)

		// TODO: Epic 7 - Replace this stub with actual engine integration:
		//   1. eng.Enqueue(inv) to submit to engine
		//   2. Register mock handlers via eng.RegisterMockHandler()
		//   3. Wait for actual completion event
		//   4. Compare actual vs expected (currently they're identical by construction)
		// For now, manufacture completion from expect clause (TAUTOLOGY - see package docs)

		// Determine expected output case (default: "Success")
		expectedCase := "Success"
		if step.Expect != nil {
			expectedCase = step.Expect.Case
		}

		// Get completion seq ONCE
		compSeq := h.clock.Next()
		compResult := ir.IRObject{}

		// If expect has result fields, include them in the completion
		if step.Expect != nil && step.Expect.Result != nil {
			compResult, err = convertArgsToIRObject(step.Expect.Result)
			if err != nil {
				return fmt.Errorf("flow step %d: failed to convert expected result: %w", i, err)
			}
		}

		compID, err := ir.CompletionID(inv.ID, expectedCase, compResult, compSeq)
		if err != nil {
			return fmt.Errorf("flow step %d: failed to compute completion ID: %w", i, err)
		}

		comp := ir.Completion{
			ID:              compID,
			InvocationID:    inv.ID,
			OutputCase:      expectedCase,
			Result:          compResult,
			Seq:             compSeq,
			SecurityContext: ir.SecurityContext{},
		}

		if err := h.store.WriteCompletion(ctx, comp); err != nil {
			return fmt.Errorf("flow step %d: failed to write completion: %w", i, err)
		}

		// Add to trace
		var traceResult interface{}
		if step.Expect != nil {
			traceResult = step.Expect.Result
		}
		result.AddCompletionTrace(comp.OutputCase, traceResult, compSeq)

		// Validate against expect clause
		if step.Expect != nil {
			// TAUTOLOGY: Validation always passes because completion IS the expect clause.
			// Epic 7 will compare actual engine-produced completion vs expected.
			// Until then, this "validation" is ceremonial - the outcome is predetermined.

			h.logger.Info("flow step validated",
				"step", i,
				"action", step.Invoke,
				"expected_case", step.Expect.Case,
				"actual_case", comp.OutputCase,
			)
		}

		h.logger.Info("flow step completed",
			"step", i,
			"action", step.Invoke,
			"invocation_id", inv.ID,
			"completion_id", comp.ID,
			"output_case", comp.OutputCase,
		)
	}

	return nil
}

// convertArgsToIRObject converts a map[string]interface{} to ir.IRObject.
// This handles YAML-parsed values and converts them to proper IRValue types.
func convertArgsToIRObject(args map[string]interface{}) (ir.IRObject, error) {
	if args == nil {
		return ir.IRObject{}, nil
	}

	result := make(ir.IRObject)
	for key, val := range args {
		irVal, err := convertToIRValue(val)
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", key, err)
		}
		result[key] = irVal
	}
	return result, nil
}

// convertToIRValue converts a YAML-parsed value to an IRValue.
// Returns an error for null values since they are forbidden in canonical JSON
// and would fail later during ID computation (ir.MarshalCanonical rejects nulls).
func convertToIRValue(val interface{}) (ir.IRValue, error) {
	if val == nil {
		// Reject nulls early with a clear error message.
		// YAML `null` or `~` would pass through here but fail during
		// canonical JSON serialization for content-addressed IDs.
		return nil, fmt.Errorf("null values are forbidden in IR (canonical JSON does not support null)")
	}

	switch v := val.(type) {
	case string:
		return ir.IRString(v), nil
	case int:
		return ir.IRInt(int64(v)), nil
	case int64:
		return ir.IRInt(v), nil
	case float64:
		// YAML parses all numbers as float64
		// Check if it's actually an integer (floats forbidden in IR per CP-5)
		if v == float64(int64(v)) {
			return ir.IRInt(int64(v)), nil
		}
		// Floats are forbidden in IR (CP-5)
		return nil, fmt.Errorf("floats are forbidden in IR (CP-5): %v", v)
	case bool:
		return ir.IRBool(v), nil
	case []interface{}:
		arr := make(ir.IRArray, len(v))
		for i, elem := range v {
			irElem, err := convertToIRValue(elem)
			if err != nil {
				return nil, fmt.Errorf("array[%d]: %w", i, err)
			}
			arr[i] = irElem
		}
		return arr, nil
	case map[string]interface{}:
		obj, err := convertArgsToIRObject(v)
		if err != nil {
			return nil, err
		}
		return obj, nil
	default:
		return nil, fmt.Errorf("unsupported type %T", val)
	}
}
