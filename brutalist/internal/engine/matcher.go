package engine

import (
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// matchWhen checks if a completion matches a when-clause.
//
// The match is determined by:
// 1. Action URI: when.ActionRef must match inv.ActionURI
// 2. Event type: when.EventType must be "completed" (for completions)
// 3. Output case: when.OutputCase matches if empty (any) or exact match
//
// Returns true only if ALL conditions are satisfied.
//
// NOTE: This function requires both the invocation (for action URI) and
// the completion (for output case). The invocation must be looked up
// separately before calling this function.
func matchWhen(when ir.WhenClause, inv *ir.Invocation, comp *ir.Completion) bool {
	// Check action URI match
	if when.ActionRef != string(inv.ActionURI) {
		return false
	}

	// Check event type (only "completed" supported for completions)
	if when.EventType != "completed" {
		return false
	}

	// Check output case (empty = match any, non-empty = exact match)
	if when.OutputCase != "" {
		if when.OutputCase != comp.OutputCase {
			return false
		}
	}

	return true
}

// extractBindings extracts bound variables from completion result.
//
// The when.Bindings map defines how to extract values:
//   - Key: variable name to bind (e.g., "order_id")
//   - Value: field name in completion result (e.g., "order_id" or "order.id")
//
// Returns error if any binding references a non-existent field.
// All-or-nothing: partial extraction is not allowed.
//
// NOTE: This implementation only supports top-level field access.
// Nested path syntax (e.g., "user.name") is not yet supported.
func extractBindings(when ir.WhenClause, comp *ir.Completion) (ir.IRObject, error) {
	if when.Bindings == nil {
		return ir.IRObject{}, nil
	}

	bindings := make(ir.IRObject, len(when.Bindings))

	for boundVar, resultField := range when.Bindings {
		// Look up field in completion result
		value, exists := comp.Result[resultField]
		if !exists {
			return nil, fmt.Errorf("binding field %q not found in completion result", resultField)
		}

		// Store in bindings map
		bindings[boundVar] = value
	}

	return bindings, nil
}

// extractInvocationBindings extracts bound variables from invocation args.
//
// Similar to extractBindings but operates on invocation args instead of
// completion result. Used for "invoked" event matching (Story 3.6).
//
// Returns error if any binding references a non-existent field.
// All-or-nothing: partial extraction is not allowed.
func extractInvocationBindings(when ir.WhenClause, inv *ir.Invocation) (ir.IRObject, error) {
	if when.Bindings == nil {
		return ir.IRObject{}, nil
	}

	bindings := make(ir.IRObject, len(when.Bindings))

	for boundVar, argsField := range when.Bindings {
		// Look up field in invocation args
		value, exists := inv.Args[argsField]
		if !exists {
			return nil, fmt.Errorf("binding field %q not found in invocation args", argsField)
		}

		// Store in bindings map
		bindings[boundVar] = value
	}

	return bindings, nil
}
