package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

// Test helper to create a test invocation
func makeTestInvocation(actionURI string) *ir.Invocation {
	return &ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: ir.ActionRef(actionURI),
		Args:      ir.IRObject{},
		Seq:       1,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
}

// Test helper to create a test completion
func makeTestCompletion(outputCase string, result ir.IRObject) *ir.Completion {
	return &ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   outputCase,
		Result:       result,
		Seq:          2,
		SecurityContext: ir.SecurityContext{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
	}
}

func TestMatchWhen_ExactMatch(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "completed",
		OutputCase: "Success",
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Cart.checkout")
	comp := makeTestCompletion("Success", ir.IRObject{})

	result := matchWhen(when, inv, comp)
	assert.True(t, result, "should match when all conditions satisfied")
}

func TestMatchWhen_EmptyOutputCaseMatchesAny(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "completed",
		OutputCase: "", // Empty = match any case
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Cart.checkout")

	testCases := []struct {
		name       string
		outputCase string
	}{
		{"success case", "Success"},
		{"error case 1", "InsufficientStock"},
		{"error case 2", "PaymentFailed"},
		{"custom case", "CustomCase"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			comp := makeTestCompletion(tc.outputCase, ir.IRObject{})

			result := matchWhen(when, inv, comp)
			assert.True(t, result, "empty output case should match any case")
		})
	}
}

func TestMatchWhen_ActionMismatch(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "completed",
		OutputCase: "",
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Inventory.reserve") // Different action
	comp := makeTestCompletion("Success", ir.IRObject{})

	result := matchWhen(when, inv, comp)
	assert.False(t, result, "should not match when action differs")
}

func TestMatchWhen_OutputCaseMismatch(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "completed",
		OutputCase: "Success", // Match only Success
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Cart.checkout")
	comp := makeTestCompletion("PaymentFailed", ir.IRObject{}) // Different case

	result := matchWhen(when, inv, comp)
	assert.False(t, result, "should not match when output case differs")
}

func TestMatchWhen_EventTypeMismatch(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Cart.checkout",
		EventType:  "invoked", // Not "completed"
		OutputCase: "",
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Cart.checkout")
	comp := makeTestCompletion("Success", ir.IRObject{})

	result := matchWhen(when, inv, comp)
	assert.False(t, result, "should not match when event type is not 'completed'")
}

func TestMatchWhen_ErrorVariantMatch(t *testing.T) {
	when := ir.WhenClause{
		ActionRef:  "Inventory.reserve",
		EventType:  "completed",
		OutputCase: "InsufficientStock", // Match specific error variant
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Inventory.reserve")
	comp := makeTestCompletion("InsufficientStock", ir.IRObject{
		"item":      ir.IRString("widget-x"),
		"requested": ir.IRInt(10),
		"available": ir.IRInt(5),
	})

	result := matchWhen(when, inv, comp)
	assert.True(t, result, "should match specific error variant")
}

// Binding extraction tests

func TestExtractBindings_Success(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Inventory.reserve",
		EventType: "completed",
		Bindings: map[string]string{
			"res_id": "reservation_id",
			"item":   "item_name",
			"qty":    "quantity",
		},
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"reservation_id": ir.IRString("res-123"),
		"item_name":      ir.IRString("widget-x"),
		"quantity":       ir.IRInt(5),
		"extra_field":    ir.IRString("ignored"), // Extra fields OK
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	expected := ir.IRObject{
		"res_id": ir.IRString("res-123"),
		"item":   ir.IRString("widget-x"),
		"qty":    ir.IRInt(5),
	}

	assert.Equal(t, expected, bindings)
}

func TestExtractBindings_MissingField(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Inventory.reserve",
		EventType: "completed",
		Bindings: map[string]string{
			"res_id":  "reservation_id",
			"missing": "nonexistent_field", // This field doesn't exist
		},
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"reservation_id": ir.IRString("res-123"),
	})

	bindings, err := extractBindings(when, comp)
	require.Error(t, err)
	assert.Nil(t, bindings, "should return nil on error")
	assert.Contains(t, err.Error(), "nonexistent_field")
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractBindings_EmptyBindings(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Cart.checkout",
		EventType: "completed",
		Bindings:  map[string]string{}, // Empty bindings
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"order_id": ir.IRString("order-123"),
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	expected := ir.IRObject{}
	assert.Equal(t, expected, bindings, "empty bindings should return empty IRObject")
}

func TestExtractBindings_NilBindings(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Cart.checkout",
		EventType: "completed",
		Bindings:  nil, // Nil bindings
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"order_id": ir.IRString("order-123"),
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	expected := ir.IRObject{}
	assert.Equal(t, expected, bindings, "nil bindings should return empty IRObject")
}

func TestExtractBindings_NestedObjects(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Order.process",
		EventType: "completed",
		Bindings: map[string]string{
			"customer": "customer_info", // Binds entire nested object
		},
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"customer_info": ir.IRObject{
			"id":   ir.IRString("cust-123"),
			"name": ir.IRString("Alice"),
		},
		"order_id": ir.IRString("order-456"),
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	// Verify nested object extracted correctly
	customerObj, ok := bindings["customer"].(ir.IRObject)
	require.True(t, ok, "customer should be IRObject")
	assert.Equal(t, ir.IRString("cust-123"), customerObj["id"])
	assert.Equal(t, ir.IRString("Alice"), customerObj["name"])
}

func TestExtractBindings_ArrayValues(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Order.getItems",
		EventType: "completed",
		Bindings: map[string]string{
			"items": "item_list",
		},
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"item_list": ir.IRArray{
			ir.IRString("item-1"),
			ir.IRString("item-2"),
			ir.IRString("item-3"),
		},
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	// Verify array extracted correctly
	itemsArr, ok := bindings["items"].(ir.IRArray)
	require.True(t, ok, "items should be IRArray")
	assert.Len(t, itemsArr, 3)
	assert.Equal(t, ir.IRString("item-1"), itemsArr[0])
}

func TestExtractBindings_AllIRValueTypes(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Test.action",
		EventType: "completed",
		Bindings: map[string]string{
			"str":  "string_val",
			"num":  "int_val",
			"bool": "bool_val",
			"arr":  "array_val",
			"obj":  "object_val",
		},
	}

	comp := makeTestCompletion("Success", ir.IRObject{
		"string_val": ir.IRString("hello"),
		"int_val":    ir.IRInt(42),
		"bool_val":   ir.IRBool(true),
		"array_val":  ir.IRArray{ir.IRInt(1), ir.IRInt(2)},
		"object_val": ir.IRObject{"key": ir.IRString("value")},
	})

	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	assert.Equal(t, ir.IRString("hello"), bindings["str"])
	assert.Equal(t, ir.IRInt(42), bindings["num"])
	assert.Equal(t, ir.IRBool(true), bindings["bool"])
	assert.Equal(t, ir.IRArray{ir.IRInt(1), ir.IRInt(2)}, bindings["arr"])
	assert.Equal(t, ir.IRObject{"key": ir.IRString("value")}, bindings["obj"])
}

// Invocation binding extraction tests

func TestExtractInvocationBindings_Success(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Cart.addItem",
		EventType: "invoked",
		Bindings: map[string]string{
			"item_id": "product_id",
			"qty":     "quantity",
		},
	}

	inv := &ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
		Args: ir.IRObject{
			"product_id": ir.IRString("prod-123"),
			"quantity":   ir.IRInt(3),
		},
	}

	bindings, err := extractInvocationBindings(when, inv)
	require.NoError(t, err)

	expected := ir.IRObject{
		"item_id": ir.IRString("prod-123"),
		"qty":     ir.IRInt(3),
	}

	assert.Equal(t, expected, bindings)
}

func TestExtractInvocationBindings_MissingField(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Cart.addItem",
		EventType: "invoked",
		Bindings: map[string]string{
			"item_id": "product_id",
			"missing": "nonexistent", // Doesn't exist
		},
	}

	inv := &ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
		Args: ir.IRObject{
			"product_id": ir.IRString("prod-123"),
		},
	}

	bindings, err := extractInvocationBindings(when, inv)
	require.Error(t, err)
	assert.Nil(t, bindings)
	assert.Contains(t, err.Error(), "nonexistent")
	assert.Contains(t, err.Error(), "not found")
}

func TestExtractInvocationBindings_EmptyBindings(t *testing.T) {
	when := ir.WhenClause{
		ActionRef: "Cart.addItem",
		EventType: "invoked",
		Bindings:  map[string]string{},
	}

	inv := &ir.Invocation{
		ID:        "inv-1",
		FlowToken: "flow-1",
		ActionURI: "Cart.addItem",
		Args: ir.IRObject{
			"product_id": ir.IRString("prod-123"),
		},
	}

	bindings, err := extractInvocationBindings(when, inv)
	require.NoError(t, err)
	assert.Equal(t, ir.IRObject{}, bindings)
}

// Story 3.4: Error handling tests

func TestMatchWhen_SpecificErrorVariant_InsufficientStock(t *testing.T) {
	// This test verifies that error variants can be matched specifically
	// for implementing error-handling sync rules

	when := ir.WhenClause{
		ActionRef:  "Inventory.reserve",
		EventType:  "completed",
		OutputCase: "InsufficientStock",
		Bindings: map[string]string{
			"item":      "item",
			"available": "available",
			"requested": "requested",
		},
	}

	inv := makeTestInvocation("Inventory.reserve")

	// InsufficientStock should match
	errorComp := makeTestCompletion("InsufficientStock", ir.IRObject{
		"item":      ir.IRString("product-123"),
		"available": ir.IRInt(3),
		"requested": ir.IRInt(5),
	})

	assert.True(t, matchWhen(when, inv, errorComp), "should match specific error variant")

	// Success should NOT match
	successComp := makeTestCompletion("Success", ir.IRObject{
		"reservation_id": ir.IRString("res-456"),
	})

	assert.False(t, matchWhen(when, inv, successComp), "should not match Success when expecting InsufficientStock")

	// Different error should NOT match
	otherErrorComp := makeTestCompletion("InvalidQuantity", ir.IRObject{
		"message": ir.IRString("quantity must be positive"),
	})

	assert.False(t, matchWhen(when, inv, otherErrorComp), "should not match different error variant")
}

func TestMatchWhen_ErrorFieldExtraction(t *testing.T) {
	// Test that error fields can be extracted for use in compensating actions

	when := ir.WhenClause{
		ActionRef:  "Inventory.reserve",
		EventType:  "completed",
		OutputCase: "InsufficientStock",
		Bindings: map[string]string{
			"item":      "item",
			"available": "available",
			"requested": "requested",
		},
	}

	inv := makeTestInvocation("Inventory.reserve")
	comp := makeTestCompletion("InsufficientStock", ir.IRObject{
		"item":      ir.IRString("product-123"),
		"available": ir.IRInt(3),
		"requested": ir.IRInt(5),
	})

	// Verify match
	assert.True(t, matchWhen(when, inv, comp))

	// Extract bindings
	bindings, err := extractBindings(when, comp)
	require.NoError(t, err)

	// Verify error fields extracted correctly
	assert.Equal(t, ir.IRString("product-123"), bindings["item"])
	assert.Equal(t, ir.IRInt(3), bindings["available"])
	assert.Equal(t, ir.IRInt(5), bindings["requested"])
}

func TestMatchWhen_UniversalHandler(t *testing.T) {
	// Test that empty OutputCase matches ALL outcomes (for logging/metrics)

	when := ir.WhenClause{
		ActionRef:  "Cart.addItem",
		EventType:  "completed",
		OutputCase: "", // Empty = match any outcome
		Bindings:   map[string]string{},
	}

	inv := makeTestInvocation("Cart.addItem")

	testCases := []struct {
		name       string
		outputCase string
	}{
		{"Success", "Success"},
		{"ItemUnavailable", "ItemUnavailable"},
		{"DuplicateItem", "DuplicateItem"},
		{"InvalidQuantity", "InvalidQuantity"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			comp := makeTestCompletion(tc.outputCase, ir.IRObject{
				"outcome": ir.IRString(tc.outputCase),
			})

			assert.True(t, matchWhen(when, inv, comp),
				"empty output case should match %s outcome", tc.outputCase)
		})
	}
}
