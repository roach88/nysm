package compiler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
)

// =============================================================================
// ConceptSpec Validation Tests
// =============================================================================

func TestValidateConceptSpecValid(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Cart",
		Purpose: "Manages shopping cart",
		Actions: []ir.ActionSig{
			{
				Name: "addItem",
				Args: []ir.NamedArg{{Name: "item_id", Type: "string"}},
				Outputs: []ir.OutputCase{
					{Case: "Success", Fields: map[string]string{"id": "string"}},
				},
			},
		},
	}

	errs := Validate(spec)
	assert.Empty(t, errs, "valid spec should have no errors")
}

func TestValidateConceptSpecValidWithStates(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Cart",
		Purpose: "Manages shopping cart",
		StateSchema: []ir.StateSchema{
			{
				Name:   "CartItem",
				Fields: map[string]string{"item_id": "string", "quantity": "int"},
			},
		},
		Actions: []ir.ActionSig{
			{
				Name:    "addItem",
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	assert.Empty(t, errs, "valid spec with states should have no errors")
}

func TestValidateConceptSpecMissingPurpose(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "", // Missing
		Actions: []ir.ActionSig{
			{
				Name:    "foo",
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrConceptPurposeEmpty, errs[0].Code)
	assert.Contains(t, errs[0].Message, "purpose")
}

func TestValidateConceptSpecWhitespacePurpose(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "   ", // Whitespace only
		Actions: []ir.ActionSig{
			{
				Name:    "foo",
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrConceptPurposeEmpty, errs[0].Code)
}

func TestValidateConceptSpecNoActions(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Empty",
		Purpose: "Does nothing",
		Actions: []ir.ActionSig{}, // No actions
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrConceptNoActions, errs[0].Code)
}

func TestValidateConceptSpecActionNoOutputs(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "Has action without outputs",
		Actions: []ir.ActionSig{
			{
				Name:    "foo",
				Outputs: []ir.OutputCase{}, // No outputs
			},
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrActionNoOutputs, errs[0].Code)
	assert.Contains(t, errs[0].Message, "foo")
}

func TestValidateConceptSpecFloatArgForbidden(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "Has float",
		Actions: []ir.ActionSig{
			{
				Name: "buy",
				Args: []ir.NamedArg{{Name: "price", Type: "float"}}, // Forbidden!
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	require.GreaterOrEqual(t, len(errs), 1)

	// Should have both invalid type and float forbidden errors
	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.Code] = true
	}
	assert.True(t, codes[ErrFloatTypeForbidden], "should have float forbidden error")
	assert.True(t, codes[ErrInvalidFieldType], "should have invalid type error")
}

func TestValidateConceptSpecFloatOutputForbidden(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "Has float output",
		Actions: []ir.ActionSig{
			{
				Name: "calc",
				Outputs: []ir.OutputCase{
					{Case: "Success", Fields: map[string]string{"result": "float64"}},
				},
			},
		},
	}

	errs := Validate(spec)
	require.GreaterOrEqual(t, len(errs), 1)

	hasFloatErr := false
	for _, e := range errs {
		if e.Code == ErrFloatTypeForbidden {
			hasFloatErr = true
		}
	}
	assert.True(t, hasFloatErr, "should have float forbidden error")
}

func TestValidateConceptSpecFloatStateForbidden(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "Has float state",
		StateSchema: []ir.StateSchema{
			{
				Name:   "Item",
				Fields: map[string]string{"price": "number"}, // "number" is float-like
			},
		},
		Actions: []ir.ActionSig{
			{Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}},
		},
	}

	errs := Validate(spec)
	require.GreaterOrEqual(t, len(errs), 1)

	hasFloatErr := false
	for _, e := range errs {
		if e.Code == ErrFloatTypeForbidden {
			hasFloatErr = true
		}
	}
	assert.True(t, hasFloatErr, "should have float forbidden error for 'number' type")
}

func TestValidateConceptSpecInvalidFieldType(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Bad",
		Purpose: "Has invalid type",
		Actions: []ir.ActionSig{
			{
				Name: "foo",
				Args: []ir.NamedArg{{Name: "x", Type: "unknown_type"}},
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidFieldType, errs[0].Code)
	assert.Contains(t, errs[0].Message, "unknown_type")
}

func TestValidateConceptSpecDuplicateAction(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Dup",
		Purpose: "Has duplicates",
		Actions: []ir.ActionSig{
			{Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}},
			{Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}}, // Duplicate
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrDuplicateName, errs[0].Code)
	assert.Contains(t, errs[0].Message, "foo")
}

func TestValidateConceptSpecDuplicateState(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Dup",
		Purpose: "Has duplicate states",
		StateSchema: []ir.StateSchema{
			{Name: "Item", Fields: map[string]string{"id": "string"}},
			{Name: "Item", Fields: map[string]string{"id": "string"}}, // Duplicate
		},
		Actions: []ir.ActionSig{
			{Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}},
		},
	}

	errs := Validate(spec)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrDuplicateName, errs[0].Code)
	assert.Contains(t, errs[0].Message, "Item")
}

func TestValidateConceptSpecAllValidTypes(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "Valid",
		Purpose: "Tests all valid types",
		Actions: []ir.ActionSig{
			{
				Name: "test",
				Args: []ir.NamedArg{
					{Name: "s", Type: "string"},
					{Name: "i", Type: "int"},
					{Name: "b", Type: "bool"},
					{Name: "a", Type: "array"},
					{Name: "o", Type: "object"},
				},
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	assert.Empty(t, errs, "all valid types should pass")
}

// =============================================================================
// SyncRule Validation Tests
// =============================================================================

func TestValidateSyncRuleValid(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When: ir.WhenClause{
			ActionRef: "Cart.checkout",
			EventType: "completed",
			Bindings:  map[string]string{"cart_id": "result.cart_id"},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"id": "bound.cart_id"},
		},
	}

	errs := Validate(rule)
	assert.Empty(t, errs, "valid rule should have no errors")
}

func TestValidateSyncRuleValidWithWhere(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When: ir.WhenClause{
			ActionRef: "Cart.checkout",
			EventType: "completed",
			Bindings:  map[string]string{"cart_id": "result.cart_id"},
		},
		Where: &ir.WhereClause{
			Source:   "CartItem",
			Filter:   "cart_id == bound.cart_id",
			Bindings: map[string]string{"item_id": "item_id"},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"id": "bound.item_id"},
		},
	}

	errs := Validate(rule)
	assert.Empty(t, errs, "valid rule with where clause should have no errors")
}

func TestValidateSyncRuleScopeFlow(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	assert.Empty(t, errs)
}

func TestValidateSyncRuleScopeGlobal(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "global"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	assert.Empty(t, errs)
}

func TestValidateSyncRuleScopeKeyedValid(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "keyed", Key: "user_id"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	assert.Empty(t, errs)
}

func TestValidateSyncRuleInvalidScope(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "invalid"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidScopeMode, errs[0].Code)
	assert.Contains(t, errs[0].Message, "invalid")
}

func TestValidateSyncRuleKeyedScopeMissingKey(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "keyed", Key: ""}, // Missing key!
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidScopeMode, errs[0].Code)
	assert.Contains(t, errs[0].Message, "key")
}

func TestValidateSyncRuleKeyedScopeWhitespaceKey(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "keyed", Key: "   "}, // Whitespace only
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidScopeMode, errs[0].Code)
}

func TestValidateSyncRuleEventTypeCompleted(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	assert.Empty(t, errs)
}

func TestValidateSyncRuleEventTypeInvoked(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "invoked"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	assert.Empty(t, errs)
}

func TestValidateSyncRuleInvalidEventType(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "started"}, // Invalid!
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidEventType, errs[0].Code)
	assert.Contains(t, errs[0].Message, "started")
}

func TestValidateSyncRuleInvalidWhenActionRef(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "invalid-format", EventType: "completed"}, // Wrong format!
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidActionRef, errs[0].Code)
	assert.Contains(t, errs[0].Field, "when")
}

func TestValidateSyncRuleInvalidThenActionRef(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "lowercase.action"}, // Concept must be uppercase
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidActionRef, errs[0].Code)
	assert.Contains(t, errs[0].Field, "then")
}

func TestValidateSyncRuleActionRefFormats(t *testing.T) {
	validRefs := []string{
		"Cart.addItem",
		"Inventory.reserve",
		"Order123.process",
		"A.b",
	}

	invalidRefs := []string{
		"cart.addItem",      // lowercase concept
		"Cart.AddItem",      // uppercase action
		"CartaddItem",       // missing dot
		"Cart.",             // missing action
		".addItem",          // missing concept
		"Cart..addItem",     // double dot
		"123Cart.addItem",   // concept starts with number
		"Cart.123action",    // action starts with number
	}

	for _, ref := range validRefs {
		assert.True(t, isValidActionRef(ref), "should be valid: %s", ref)
	}

	for _, ref := range invalidRefs {
		assert.False(t, isValidActionRef(ref), "should be invalid: %s", ref)
	}
}

func TestValidateSyncRuleUndefinedBoundVar(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When: ir.WhenClause{
			ActionRef: "Cart.checkout",
			EventType: "completed",
			Bindings:  map[string]string{"cart_id": "result.cart_id"},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"id": "bound.undefined_var"}, // Not defined!
		},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrUndefinedBoundVariable, errs[0].Code)
	assert.Contains(t, errs[0].Message, "undefined_var")
}

func TestValidateSyncRuleMultipleUndefinedVars(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When: ir.WhenClause{
			ActionRef: "Cart.checkout",
			EventType: "completed",
			Bindings:  map[string]string{"cart_id": "result.cart_id"},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args: map[string]string{
				"id":   "bound.undefined1",
				"qty":  "bound.undefined2",
			},
		},
	}

	errs := Validate(rule)
	assert.GreaterOrEqual(t, len(errs), 2, "should have errors for both undefined vars")

	for _, e := range errs {
		assert.Equal(t, ErrUndefinedBoundVariable, e.Code)
	}
}

func TestValidateSyncRuleBoundVarFromWhere(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When: ir.WhenClause{
			ActionRef: "Cart.checkout",
			EventType: "completed",
			Bindings:  map[string]string{"cart_id": "result.cart_id"},
		},
		Where: &ir.WhereClause{
			Source:   "CartItem",
			Bindings: map[string]string{"item_id": "item_id"},
		},
		Then: ir.ThenClause{
			ActionRef: "Inventory.reserve",
			Args:      map[string]string{"id": "bound.item_id"}, // From where clause
		},
	}

	errs := Validate(rule)
	assert.Empty(t, errs, "bound var from where clause should be valid")
}

func TestValidateSyncRuleWhereEmptySource(t *testing.T) {
	rule := &ir.SyncRule{
		ID:    "bad",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Where: &ir.WhereClause{
			Source: "", // Empty source!
		},
		Then: ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule)
	require.Len(t, errs, 1)
	assert.Equal(t, ErrInvalidWhereClause, errs[0].Code)
}

// =============================================================================
// General Validation Tests
// =============================================================================

func TestValidateUnsupportedType(t *testing.T) {
	errs := Validate("not an IR type")
	require.Len(t, errs, 1)
	assert.Equal(t, ErrUnsupportedIRType, errs[0].Code)
}

func TestValidateConceptSpecByValue(t *testing.T) {
	spec := ir.ConceptSpec{
		Name:    "Test",
		Purpose: "Testing",
		Actions: []ir.ActionSig{
			{Name: "foo", Outputs: []ir.OutputCase{{Case: "Success"}}},
		},
	}

	errs := Validate(spec) // Pass by value, not pointer
	assert.Empty(t, errs)
}

func TestValidateSyncRuleByValue(t *testing.T) {
	rule := ir.SyncRule{
		ID:    "test",
		Scope: ir.ScopeSpec{Mode: "flow"},
		When:  ir.WhenClause{ActionRef: "Cart.checkout", EventType: "completed"},
		Then:  ir.ThenClause{ActionRef: "Inventory.reserve"},
	}

	errs := Validate(rule) // Pass by value, not pointer
	assert.Empty(t, errs)
}

func TestValidateCollectsAllErrors(t *testing.T) {
	spec := &ir.ConceptSpec{
		Name:    "",        // Missing name is OK, but purpose...
		Purpose: "",        // E101
		Actions: []ir.ActionSig{
			{
				Name:    "foo",
				Args:    []ir.NamedArg{{Name: "x", Type: "float"}}, // E104 + E106
				Outputs: []ir.OutputCase{},                          // E103
			},
			{
				Name:    "foo", // E105 duplicate
				Outputs: []ir.OutputCase{{Case: "Success"}},
			},
		},
	}

	errs := Validate(spec)
	assert.GreaterOrEqual(t, len(errs), 4, "should collect multiple errors")

	// Verify we got different error codes
	codes := make(map[string]bool)
	for _, e := range errs {
		codes[e.Code] = true
	}
	assert.True(t, codes[ErrConceptPurposeEmpty], "should have purpose error")
	assert.True(t, codes[ErrActionNoOutputs], "should have no outputs error")
	assert.True(t, codes[ErrDuplicateName], "should have duplicate name error")
}

func TestValidationErrorFormat(t *testing.T) {
	err := ValidationError{
		Field:   "purpose",
		Message: "purpose is required",
		Code:    ErrConceptPurposeEmpty,
	}

	assert.Equal(t, "[E101] purpose: purpose is required", err.Error())
}

func TestValidationErrorFormatWithLine(t *testing.T) {
	err := ValidationError{
		Field:   "actions[0].type",
		Message: "invalid type",
		Code:    ErrInvalidFieldType,
		Line:    42,
	}

	assert.Equal(t, "[E104] line 42: actions[0].type: invalid type", err.Error())
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestExtractBoundVariableRefs(t *testing.T) {
	tests := []struct {
		expr     string
		expected []string
	}{
		{"bound.cart_id", []string{"cart_id"}},
		{"bound.x + bound.y", []string{"x", "y"}},
		{"no refs here", []string{}},
		{"bound.item_id + 1", []string{"item_id"}},
		{"bound.a_b_c", []string{"a_b_c"}},
		{"bound.x123", []string{"x123"}},
	}

	for _, tt := range tests {
		result := extractBoundVariableRefs(tt.expr)
		assert.Equal(t, tt.expected, result, "for expr: %s", tt.expr)
	}
}

func TestCollectBoundVariables(t *testing.T) {
	rule := &ir.SyncRule{
		When: ir.WhenClause{
			Bindings: map[string]string{"a": "result.a", "b": "result.b"},
		},
		Where: &ir.WhereClause{
			Bindings: map[string]string{"c": "c", "d": "d"},
		},
	}

	vars := collectBoundVariables(rule)
	assert.True(t, vars["a"])
	assert.True(t, vars["b"])
	assert.True(t, vars["c"])
	assert.True(t, vars["d"])
	assert.False(t, vars["e"])
}

func TestCollectBoundVariablesNoWhere(t *testing.T) {
	rule := &ir.SyncRule{
		When: ir.WhenClause{
			Bindings: map[string]string{"a": "result.a"},
		},
		Where: nil, // No where clause
	}

	vars := collectBoundVariables(rule)
	assert.True(t, vars["a"])
	assert.Len(t, vars, 1)
}

func TestIsValidType(t *testing.T) {
	validTypes := []string{"string", "int", "bool", "array", "object"}
	invalidTypes := []string{"float", "float64", "number", "double", "unknown", ""}

	for _, typ := range validTypes {
		assert.True(t, isValidType(typ), "should be valid: %s", typ)
	}

	for _, typ := range invalidTypes {
		assert.False(t, isValidType(typ), "should be invalid: %s", typ)
	}
}

func TestIsFloatType(t *testing.T) {
	floatTypes := []string{"float", "float32", "float64", "number", "double"}
	nonFloatTypes := []string{"string", "int", "bool", "array", "object", "unknown"}

	for _, typ := range floatTypes {
		assert.True(t, isFloatType(typ), "should be float: %s", typ)
	}

	for _, typ := range nonFloatTypes {
		assert.False(t, isFloatType(typ), "should not be float: %s", typ)
	}
}

func TestIsValidScopeMode(t *testing.T) {
	validModes := []string{"flow", "global", "keyed"}
	invalidModes := []string{"local", "session", ""}

	for _, mode := range validModes {
		assert.True(t, isValidScopeMode(mode), "should be valid: %s", mode)
	}

	for _, mode := range invalidModes {
		assert.False(t, isValidScopeMode(mode), "should be invalid: %s", mode)
	}
}
