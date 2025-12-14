package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/roach88/nysm/internal/ir"
)

// executeThen generates invocations from then-clause bindings.
//
// For each binding:
// 1. Resolve arg templates with binding values
// 2. Check for cycles (same sync/binding in same flow)
// 3. Generate invocation with content-addressed ID
// 4. Atomically write sync_firing, invocation, provenance_edge (CP-1 crash safety)
// 5. If already fired (idempotency), skip enqueue
// 6. Otherwise enqueue invocation for execution
//
// Empty bindings slice is valid (generates zero invocations).
// Multiple bindings generate multiple invocations (critical for multi-binding syncs).
//
// CRASH SAFETY: Uses WriteSyncFiringAtomic to ensure all 3 writes happen atomically.
// If we crash mid-write, either all 3 are persisted or none are. This prevents the
// scenario where HasFiring returns true but the invocation was never written.
//
// CYCLE SAFETY: Uses CycleDetector to prevent infinite loops within a flow.
// If the same (sync_id, binding_hash) would fire twice in the same flow, returns
// RuntimeError with ErrCodeCycleDetected. This is distinct from idempotency:
//   - Idempotency (persistent): "Have we fired this (completion, sync, binding)?"
//   - Cycle detection (in-memory): "Have we fired this (sync, binding) in this flow?"
//
// Implements FR-2.4 (execute then-clause to generate invocations from bindings).
// Implements CP-1 (binding-level idempotency via binding hash).
// Implements Story 5.3 (cycle detection per flow).
func (e *Engine) executeThen(
	ctx context.Context,
	then ir.ThenClause,
	bindings []ir.IRObject,
	flowToken string,
	completion ir.Completion,
	sync ir.SyncRule,
) error {
	// Handle empty bindings (valid - no invocations generated)
	if len(bindings) == 0 {
		return nil
	}

	// Process each binding independently
	for _, binding := range bindings {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context cancelled: %w", err)
		}

		// Compute binding hash for idempotency (CP-1) and cycle detection
		bindingHash, err := ir.BindingHash(binding)
		if err != nil {
			return fmt.Errorf("compute binding hash: %w", err)
		}

		// CYCLE DETECTION (Story 5.3): Check if this (sync, binding) would cycle
		// This is checked BEFORE firing to prevent infinite loops.
		// Distinct from idempotency: cycles are per-flow, idempotency is per-completion.
		if e.cycleDetector.WouldCycle(flowToken, sync.ID, bindingHash) {
			return NewCycleError(flowToken, sync.ID, bindingHash)
		}

		// NOTE: Record() happens AFTER WriteSyncFiringAtomic, not here.
		// This ensures replay scenarios work: if inserted=false (already fired),
		// we don't record, so subsequent replay attempts won't trigger cycle errors.

		// Resolve arg templates with binding values
		resolvedArgs, err := resolveArgs(then.Args, binding)
		if err != nil {
			return fmt.Errorf("resolve args for binding: %w", err)
		}

		// Get sequence number for invocation
		seq := e.clock.Next()

		// Compute content-addressed ID
		invID, err := ir.InvocationID(flowToken, then.ActionRef, resolvedArgs, seq)
		if err != nil {
			return fmt.Errorf("compute invocation ID: %w", err)
		}

		// Generate invocation
		inv := ir.Invocation{
			ID:              invID,
			FlowToken:       flowToken,
			ActionURI:       ir.ActionRef(then.ActionRef),
			Args:            resolvedArgs,
			Seq:             seq,
			SecurityContext: completion.SecurityContext, // Propagate from completion
			SpecHash:        e.specHash,
			EngineVersion:   ir.EngineVersion,
			IRVersion:       ir.IRVersion,
		}

		// Create sync firing record
		firing := ir.SyncFiring{
			CompletionID: completion.ID,
			SyncID:       sync.ID,
			BindingHash:  bindingHash,
			Seq:          e.clock.Next(),
		}

		// CRASH ATOMICITY (CP-1): Atomically write firing, invocation, provenance edge.
		// If inserted=false, this binding already fired (replay scenario) - skip enqueue.
		_, inserted, err := e.store.WriteSyncFiringAtomic(ctx, firing, inv)
		if err != nil {
			return fmt.Errorf("atomic write firing: %w", err)
		}

		// Only record and enqueue if this is a new firing (not a replay).
		// IMPORTANT: Record() must happen AFTER successful write, not before.
		// This ensures replay scenarios work correctly:
		// - Fresh engine + replay: WouldCycle=false, Write inserted=false, no Record
		// - Same engine + cycle: WouldCycle=true (already recorded), error returned
		if inserted {
			e.cycleDetector.Record(flowToken, sync.ID, bindingHash)
			e.queue.Enqueue(Event{
				Type:       EventTypeInvocation,
				Invocation: &inv,
			})
		}
	}

	return nil
}

// resolveArgs substitutes binding variables into then-clause arg templates.
//
// Arg templates support "bound.varName" syntax for binding substitution:
//   - "bound.item_id" -> looks up "item_id" in bindings
//   - Literal strings (not starting with "bound.") passed through unchanged
//
// Returns error if binding variable not found.
// All-or-nothing: partial substitution not allowed.
//
// Example:
//
//	argTemplates = {"product": "bound.item_id", "priority": "high"}
//	bindings = {"item_id": IRString("widget-x")}
//	result = {"product": IRString("widget-x"), "priority": IRString("high")}
func resolveArgs(argTemplates map[string]string, bindings ir.IRObject) (ir.IRObject, error) {
	resolvedArgs := make(ir.IRObject, len(argTemplates))

	for key, template := range argTemplates {
		// Check if template is a bound variable reference
		if strings.HasPrefix(template, "bound.") {
			// Extract variable name (strip "bound." prefix)
			varName := template[6:] // len("bound.") == 6

			// Look up variable in bindings
			value, exists := bindings[varName]
			if !exists {
				return nil, fmt.Errorf("binding variable %q not found (referenced in arg %q)", varName, key)
			}

			// Substitute binding value
			resolvedArgs[key] = value
		} else {
			// Literal value - pass through as IRString
			resolvedArgs[key] = ir.IRString(template)
		}
	}

	return resolvedArgs, nil
}
