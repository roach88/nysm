package engine

import (
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// ScopeMode defines how sync rules match records across flows.
// This implements the flow token scoping modes from FR-3.3 and HIGH-1.
type ScopeMode string

const (
	// ScopeModeFlow matches only records with same flow_token (default).
	// This is the safest mode - prevents accidental cross-request joins.
	// Use for normal transactional workflows (cart checkout, order processing).
	ScopeModeFlow ScopeMode = "flow"

	// ScopeModeGlobal matches all records regardless of flow_token.
	// Requires explicit opt-in via `scope: "global"` in sync rule.
	// Use for global inventory limits, system-wide rate limiting, deduplication.
	ScopeModeGlobal ScopeMode = "global"

	// ScopeModeKeyed matches records sharing same value for specified field.
	// Allows cross-flow coordination within entity boundaries.
	// Use for per-user rate limits, session tracking, tenant isolation.
	ScopeModeKeyed ScopeMode = "keyed"
)

// ValidateScopeMode checks if mode is a valid scope mode.
// Returns error if mode is not one of: flow, global, keyed.
func ValidateScopeMode(mode string) error {
	switch ScopeMode(mode) {
	case ScopeModeFlow, ScopeModeGlobal, ScopeModeKeyed:
		return nil
	case "":
		// Empty is valid - will default to flow
		return nil
	default:
		return fmt.Errorf("invalid scope mode %q: must be flow, global, or keyed", mode)
	}
}

// DefaultScope returns the default scope spec (flow mode).
// Per FR-3.3, the default is "flow" to prevent accidental cross-flow joins.
// This is the safe-by-default approach - developers must explicitly opt-in
// to global or keyed modes.
func DefaultScope() ir.ScopeSpec {
	return ir.ScopeSpec{
		Mode: string(ScopeModeFlow),
	}
}

// NormalizeScope ensures a scope spec has a valid mode.
// Returns the scope with defaulted mode if empty.
func NormalizeScope(scope ir.ScopeSpec) ir.ScopeSpec {
	if scope.Mode == "" {
		scope.Mode = string(ScopeModeFlow)
	}
	return scope
}

// extractKeyValue extracts the key field value from bindings.
// Returns error if key not found (required for keyed scope).
//
// This function is used by executeWhereClause to get the key value
// when processing a sync rule with keyed scope.
func extractKeyValue(bindings ir.IRObject, key string) (ir.IRValue, error) {
	if key == "" {
		return nil, fmt.Errorf("keyed scope requires non-empty key field")
	}

	value, exists := bindings[key]
	if !exists {
		return nil, fmt.Errorf("key field %q not found in when-bindings (required for keyed scope)", key)
	}

	return value, nil
}

// mergeBindings combines when-bindings and where-bindings.
// Where-bindings take precedence in case of conflicts.
//
// This function is used to combine bindings from the when-clause
// (extracted from completion result) with bindings from the where-clause
// (extracted from state query results).
func mergeBindings(whenBindings, whereBindings ir.IRObject) ir.IRObject {
	// Handle nil cases
	if whenBindings == nil && whereBindings == nil {
		return ir.IRObject{}
	}
	if whenBindings == nil {
		result := make(ir.IRObject, len(whereBindings))
		for k, v := range whereBindings {
			result[k] = v
		}
		return result
	}
	if whereBindings == nil {
		result := make(ir.IRObject, len(whenBindings))
		for k, v := range whenBindings {
			result[k] = v
		}
		return result
	}

	merged := make(ir.IRObject, len(whenBindings)+len(whereBindings))

	// Copy when-bindings first
	for k, v := range whenBindings {
		merged[k] = v
	}

	// Where-bindings override (if any conflicts)
	for k, v := range whereBindings {
		merged[k] = v
	}

	return merged
}
