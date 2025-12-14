package ir

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionSigValidation(t *testing.T) {
	tests := []struct {
		name     string
		action   ActionSig
		wantErrs int
		errField string // Expected field in first error (for specific checks)
	}{
		{
			name: "valid action with success case",
			action: ActionSig{
				Name: "checkout",
				Args: []NamedArg{{Name: "cart_id", Type: "string"}},
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{"order_id": "string"}},
				},
			},
			wantErrs: 0,
		},
		{
			name: "valid action with multiple outputs",
			action: ActionSig{
				Name: "addItem",
				Args: []NamedArg{
					{Name: "item_id", Type: "string"},
					{Name: "quantity", Type: "int"},
				},
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{"item_id": "string", "new_quantity": "int"}},
					{Case: "InsufficientStock", Fields: map[string]string{"available": "int", "requested": "int"}},
					{Case: "InvalidQuantity", Fields: map[string]string{"message": "string", "max_allowed": "int"}},
				},
				Requires: []string{"cart:write"},
			},
			wantErrs: 0,
		},
		{
			name: "empty outputs",
			action: ActionSig{
				Name:    "invalid",
				Args:    []NamedArg{},
				Outputs: []OutputCase{},
			},
			wantErrs: 1,
			errField: "outputs",
		},
		{
			name: "duplicate case names",
			action: ActionSig{
				Name: "bad",
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{}},
					{Case: "Success", Fields: map[string]string{}}, // duplicate
				},
			},
			wantErrs: 1,
			errField: "outputs[1].case",
		},
		{
			name: "invalid type in args - float forbidden",
			action: ActionSig{
				Name: "bad",
				Args: []NamedArg{{Name: "price", Type: "float"}}, // float forbidden!
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{}},
				},
			},
			wantErrs: 1,
			errField: "args[0].type",
		},
		{
			name: "invalid type in output fields",
			action: ActionSig{
				Name: "bad",
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{"amount": "float"}}, // float forbidden!
				},
			},
			wantErrs: 1,
			errField: "outputs[0].fields.amount",
		},
		{
			name: "multiple errors",
			action: ActionSig{
				Name: "veryBad",
				Args: []NamedArg{
					{Name: "price", Type: "float"},   // error 1
					{Name: "value", Type: "decimal"}, // error 2
				},
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{"total": "number"}}, // error 3
				},
			},
			wantErrs: 3,
		},
		{
			name: "all valid types",
			action: ActionSig{
				Name: "allTypes",
				Args: []NamedArg{
					{Name: "a", Type: "string"},
					{Name: "b", Type: "int"},
					{Name: "c", Type: "bool"},
					{Name: "d", Type: "array"},
					{Name: "e", Type: "object"},
				},
				Outputs: []OutputCase{
					{Case: "Success", Fields: map[string]string{
						"s": "string",
						"i": "int",
						"b": "bool",
						"a": "array",
						"o": "object",
					}},
				},
			},
			wantErrs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := tt.action.Validate()
			assert.Len(t, errs, tt.wantErrs)
			if tt.errField != "" && len(errs) > 0 {
				assert.Equal(t, tt.errField, errs[0].Field)
			}
		})
	}
}

func TestValidTypesNoFloat(t *testing.T) {
	// Explicitly verify float is NOT in ValidTypes (CP-5)
	assert.False(t, ValidTypes["float"], "float must NOT be a valid type (CP-5)")
	assert.False(t, ValidTypes["double"], "double must NOT be a valid type (CP-5)")
	assert.False(t, ValidTypes["number"], "number must NOT be a valid type (CP-5)")

	// Verify all expected types are present
	assert.True(t, ValidTypes["string"])
	assert.True(t, ValidTypes["int"])
	assert.True(t, ValidTypes["bool"])
	assert.True(t, ValidTypes["array"])
	assert.True(t, ValidTypes["object"])
}

func TestOutputCaseJSONSortedKeys(t *testing.T) {
	out := OutputCase{
		Case: "InsufficientStock",
		Fields: map[string]string{
			"requested": "int",
			"available": "int",
		},
	}

	data, err := json.Marshal(out)
	require.NoError(t, err)

	// Fields should be sorted: available before requested
	expected := `{"case":"InsufficientStock","fields":{"available":"int","requested":"int"}}`
	assert.Equal(t, expected, string(data))
}

func TestActionSigJSONSortedKeys(t *testing.T) {
	action := ActionSig{
		Name: "checkout",
		Args: []NamedArg{
			{Name: "cart_id", Type: "string"},
		},
		Outputs: []OutputCase{
			{Case: "Success", Fields: map[string]string{"order_id": "string"}},
		},
	}

	data, err := json.Marshal(action)
	require.NoError(t, err)

	// Keys should be in alphabetical order: args, name, outputs
	// (requires is omitted because it's empty)
	expected := `{"args":[{"name":"cart_id","type":"string"}],"name":"checkout","outputs":[{"case":"Success","fields":{"order_id":"string"}}]}`
	assert.Equal(t, expected, string(data))
}

func TestActionSigJSONWithRequires(t *testing.T) {
	action := ActionSig{
		Name: "checkout",
		Args: []NamedArg{
			{Name: "cart_id", Type: "string"},
		},
		Outputs: []OutputCase{
			{Case: "Success", Fields: map[string]string{"order_id": "string"}},
		},
		Requires: []string{"cart:write", "order:create"},
	}

	data, err := json.Marshal(action)
	require.NoError(t, err)

	// Requires should appear after outputs (alphabetical)
	assert.Contains(t, string(data), `"requires":["cart:write","order:create"]`)
}

func TestErrorVariantWithTypedFields(t *testing.T) {
	// Verify error variants can have rich typed fields
	action := ActionSig{
		Name: "reserve",
		Args: []NamedArg{
			{Name: "item_id", Type: "string"},
			{Name: "quantity", Type: "int"},
		},
		Outputs: []OutputCase{
			{
				Case: "Success",
				Fields: map[string]string{
					"reservation_id": "string",
					"expires_at":     "int", // Unix timestamp as int, not float
				},
			},
			{
				Case: "InsufficientStock",
				Fields: map[string]string{
					"available": "int",
					"requested": "int",
					"item_name": "string",
				},
			},
		},
	}

	errs := action.Validate()
	assert.Empty(t, errs, "error variant with typed fields should validate")
}

func TestActionSigJSONRoundTrip(t *testing.T) {
	original := ActionSig{
		Name: "addItem",
		Args: []NamedArg{
			{Name: "item_id", Type: "string"},
			{Name: "quantity", Type: "int"},
		},
		Outputs: []OutputCase{
			{Case: "Success", Fields: map[string]string{"item_id": "string", "new_quantity": "int"}},
			{Case: "InsufficientStock", Fields: map[string]string{"available": "int", "requested": "int"}},
		},
		Requires: []string{"cart:write"},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded ActionSig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.Name, decoded.Name)
	assert.Equal(t, original.Args, decoded.Args)
	assert.Equal(t, original.Requires, decoded.Requires)
	assert.Len(t, decoded.Outputs, 2)
	assert.Equal(t, original.Outputs[0].Case, decoded.Outputs[0].Case)
	assert.Equal(t, original.Outputs[1].Case, decoded.Outputs[1].Case)
}

func TestOutputCaseEmptyFields(t *testing.T) {
	out := OutputCase{
		Case:   "Success",
		Fields: map[string]string{},
	}

	data, err := json.Marshal(out)
	require.NoError(t, err)

	expected := `{"case":"Success","fields":{}}`
	assert.Equal(t, expected, string(data))
}

func TestNamedArgJSONSortedKeys(t *testing.T) {
	arg := NamedArg{
		Name: "item_id",
		Type: "string",
	}

	data, err := json.Marshal(arg)
	require.NoError(t, err)

	// Keys should be in alphabetical order: name, type
	expected := `{"name":"item_id","type":"string"}`
	assert.Equal(t, expected, string(data))
}

func TestValidationErrorMessage(t *testing.T) {
	err := ValidationError{
		Field:   "args[0].type",
		Message: "invalid type",
	}

	assert.Equal(t, "args[0].type: invalid type", err.Error())
}

func TestActionSigWithNoArgs(t *testing.T) {
	// Actions can have no args (e.g., getStatus)
	action := ActionSig{
		Name: "getStatus",
		Args: []NamedArg{},
		Outputs: []OutputCase{
			{Case: "Success", Fields: map[string]string{"status": "string"}},
		},
	}

	errs := action.Validate()
	assert.Empty(t, errs, "action with no args should be valid")

	data, err := json.Marshal(action)
	require.NoError(t, err)

	expected := `{"args":[],"name":"getStatus","outputs":[{"case":"Success","fields":{"status":"string"}}]}`
	assert.Equal(t, expected, string(data))
}
