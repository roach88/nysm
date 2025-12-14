package queryir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
)

func TestValidate_PortableQuery(t *testing.T) {
	// Simple portable query: SELECT item_id FROM cart_items WHERE cart_id = ?
	query := Select{
		From: "cart_items",
		Filter: Equals{
			Field: "cart_id",
			Value: ir.IRString("cart-123"),
		},
		Bindings: map[string]string{
			"item_id": "item_id",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "simple select should be portable")
	assert.Empty(t, result.Warnings, "no warnings for portable query")
}

func TestValidate_PortableQueryWithPointer(t *testing.T) {
	// Test with pointer types
	query := &Select{
		From: "cart_items",
		Filter: &Equals{
			Field: "cart_id",
			Value: ir.IRString("cart-123"),
		},
		Bindings: map[string]string{
			"item_id": "item_id",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "pointer types should be portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_EmptyBindings(t *testing.T) {
	// SELECT * is not portable
	query := Select{
		From:     "cart_items",
		Filter:   nil,
		Bindings: map[string]string{}, // Empty = SELECT *
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "SELECT * is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "Empty bindings")
	assert.Contains(t, result.Warnings[0], "SELECT *")
}

func TestValidate_NilBindings(t *testing.T) {
	// nil bindings is also SELECT *
	query := Select{
		From:     "cart_items",
		Filter:   nil,
		Bindings: nil,
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "nil bindings is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "Empty bindings")
}

func TestValidate_NestedAndPredicate(t *testing.T) {
	// Portable query with AND predicate
	query := Select{
		From: "inventory",
		Filter: And{
			Predicates: []Predicate{
				Equals{
					Field: "product_id",
					Value: ir.IRString("product-1"),
				},
				Equals{
					Field: "warehouse_id",
					Value: ir.IRString("warehouse-west"),
				},
			},
		},
		Bindings: map[string]string{
			"available": "quantity_available",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "AND predicates are portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_BoundEquals(t *testing.T) {
	// BoundEquals references when-clause bindings - portable
	query := Select{
		From: "inventory",
		Filter: BoundEquals{
			Field:    "product_id",
			BoundVar: "bound.itemId", // From when-clause
		},
		Bindings: map[string]string{
			"available": "quantity_available",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "BoundEquals is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_BoundEqualsPointer(t *testing.T) {
	// BoundEquals with pointer type
	query := Select{
		From: "inventory",
		Filter: &BoundEquals{
			Field:    "product_id",
			BoundVar: "bound.itemId",
		},
		Bindings: map[string]string{
			"available": "quantity_available",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "BoundEquals pointer is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_Join(t *testing.T) {
	// Inner join is portable
	query := Join{
		Left: Select{
			From: "cart_items",
			Bindings: map[string]string{
				"product_id": "product_id",
			},
		},
		Right: Select{
			From: "inventory",
			Bindings: map[string]string{
				"available": "quantity_available",
			},
		},
		On: Equals{
			Field: "product_id",
			Value: ir.IRString("product-1"),
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "inner join is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_JoinPointer(t *testing.T) {
	// Inner join with pointer type
	query := &Join{
		Left: &Select{
			From: "cart_items",
			Bindings: map[string]string{
				"product_id": "product_id",
			},
		},
		Right: &Select{
			From: "inventory",
			Bindings: map[string]string{
				"available": "quantity_available",
			},
		},
		On: &Equals{
			Field: "product_id",
			Value: ir.IRString("product-1"),
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "inner join pointer is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_NullValue(t *testing.T) {
	// NULL comparisons are not portable
	query := Select{
		From: "cart_items",
		Filter: Equals{
			Field: "deleted_at",
			Value: ir.IRNull{}, // NULL comparison
		},
		Bindings: map[string]string{
			"item_id": "item_id",
		},
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "NULL comparison is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "NULL")
	assert.Contains(t, result.Warnings[0], "deleted_at")
}

func TestValidate_NullInAnd(t *testing.T) {
	// NULL in nested And predicate
	query := Select{
		From: "cart_items",
		Filter: And{
			Predicates: []Predicate{
				Equals{Field: "status", Value: ir.IRString("active")},
				Equals{Field: "deleted_at", Value: ir.IRNull{}},
			},
		},
		Bindings: map[string]string{
			"item_id": "item_id",
		},
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "NULL in And is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "NULL")
}

func TestValidate_MultipleViolations(t *testing.T) {
	// Query with multiple non-portable features
	query := Join{
		Left: Select{
			From:     "cart_items",
			Filter:   Equals{Field: "deleted_at", Value: ir.IRNull{}}, // Violation 1: NULL
			Bindings: map[string]string{},                             // Violation 2: SELECT *
		},
		Right: Select{
			From:     "inventory",
			Bindings: map[string]string{}, // Violation 3: SELECT *
		},
		On: Equals{Field: "product_id", Value: ir.IRString("p1")},
	}

	result := Validate(query)

	assert.False(t, result.IsPortable)
	assert.Len(t, result.Warnings, 3, "should accumulate multiple warnings")
}

func TestValidate_NestedJoins(t *testing.T) {
	// Nested joins - all should be validated recursively
	query := Join{
		Left: Join{
			Left: Select{
				From:     "table_a",
				Bindings: map[string]string{"a": "a"},
			},
			Right: Select{
				From:     "table_b",
				Bindings: map[string]string{"b": "b"},
			},
			On: Equals{Field: "id", Value: ir.IRInt(1)},
		},
		Right: Select{
			From:     "table_c",
			Bindings: map[string]string{"c": "c"},
		},
		On: Equals{Field: "id", Value: ir.IRInt(2)},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "nested inner joins are portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_NestedJoinsWithViolation(t *testing.T) {
	// Nested join with violation deep in the tree
	query := Join{
		Left: Join{
			Left: Select{
				From:     "table_a",
				Bindings: map[string]string{}, // Violation: SELECT *
			},
			Right: Select{
				From:     "table_b",
				Bindings: map[string]string{"b": "b"},
			},
			On: Equals{Field: "id", Value: ir.IRInt(1)},
		},
		Right: Select{
			From:     "table_c",
			Bindings: map[string]string{"c": "c"},
		},
		On: Equals{Field: "id", Value: ir.IRInt(2)},
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "nested join with violation is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "Empty bindings")
}

func TestValidate_DeepNestedAnd(t *testing.T) {
	// Deeply nested And predicates
	query := Select{
		From: "items",
		Filter: And{
			Predicates: []Predicate{
				And{
					Predicates: []Predicate{
						Equals{Field: "a", Value: ir.IRInt(1)},
						And{
							Predicates: []Predicate{
								Equals{Field: "b", Value: ir.IRInt(2)},
								Equals{Field: "c", Value: ir.IRInt(3)},
							},
						},
					},
				},
				Equals{Field: "d", Value: ir.IRInt(4)},
			},
		},
		Bindings: map[string]string{"id": "id"},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "deep nested And is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_DeepNestedAndWithViolation(t *testing.T) {
	// Deeply nested And with NULL violation
	query := Select{
		From: "items",
		Filter: And{
			Predicates: []Predicate{
				And{
					Predicates: []Predicate{
						Equals{Field: "a", Value: ir.IRInt(1)},
						And{
							Predicates: []Predicate{
								Equals{Field: "b", Value: ir.IRNull{}}, // Violation deep in tree
							},
						},
					},
				},
			},
		},
		Bindings: map[string]string{"id": "id"},
	}

	result := Validate(query)

	assert.False(t, result.IsPortable, "deep nested NULL is not portable")
	require.Len(t, result.Warnings, 1)
	assert.Contains(t, result.Warnings[0], "NULL")
}

func TestValidate_EmptyAndPredicate(t *testing.T) {
	// Empty And predicate (vacuous truth) is portable
	query := Select{
		From: "items",
		Filter: And{
			Predicates: []Predicate{},
		},
		Bindings: map[string]string{"id": "id"},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "empty And is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_NilFilter(t *testing.T) {
	// nil filter is portable (no filter = all rows)
	query := Select{
		From:     "items",
		Filter:   nil,
		Bindings: map[string]string{"id": "id"},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "nil filter is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_AllIRValueTypes(t *testing.T) {
	// All non-NULL IRValue types should be portable
	tests := []struct {
		name  string
		value ir.IRValue
	}{
		{"string", ir.IRString("test")},
		{"int", ir.IRInt(42)},
		{"bool", ir.IRBool(true)},
		{"array", ir.IRArray{ir.IRInt(1), ir.IRInt(2)}},
		{"object", ir.IRObject{"key": ir.IRString("value")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := Select{
				From: "items",
				Filter: Equals{
					Field: "field",
					Value: tt.value,
				},
				Bindings: map[string]string{"id": "id"},
			}

			result := Validate(query)

			assert.True(t, result.IsPortable, "%s should be portable", tt.name)
			assert.Empty(t, result.Warnings)
		})
	}
}

func TestValidate_JoinWithNilOn(t *testing.T) {
	// Join with nil On predicate (cross join) is portable
	// but unusual - validation doesn't warn about this
	query := Join{
		Left: Select{
			From:     "table_a",
			Bindings: map[string]string{"a": "a"},
		},
		Right: Select{
			From:     "table_b",
			Bindings: map[string]string{"b": "b"},
		},
		On: nil, // Cross join
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "nil On predicate is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_JoinWithBoundEqualsOn(t *testing.T) {
	// Join with BoundEquals On predicate
	query := Join{
		Left: Select{
			From:     "cart_items",
			Bindings: map[string]string{"product_id": "productId"},
		},
		Right: Select{
			From:     "inventory",
			Bindings: map[string]string{"available": "available"},
		},
		On: BoundEquals{
			Field:    "product_id",
			BoundVar: "bound.productId",
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "BoundEquals join condition is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_JoinWithAndOn(t *testing.T) {
	// Join with And On predicate
	query := Join{
		Left: Select{
			From:     "cart_items",
			Bindings: map[string]string{"product_id": "productId"},
		},
		Right: Select{
			From:     "inventory",
			Bindings: map[string]string{"available": "available"},
		},
		On: And{
			Predicates: []Predicate{
				Equals{Field: "product_id", Value: ir.IRString("p1")},
				Equals{Field: "warehouse_id", Value: ir.IRString("w1")},
			},
		},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "And join condition is portable")
	assert.Empty(t, result.Warnings)
}

func TestValidate_ComplexPortableQuery(t *testing.T) {
	// Complex but fully portable query
	query := Join{
		Left: Select{
			From: "carts",
			Filter: And{
				Predicates: []Predicate{
					Equals{Field: "status", Value: ir.IRString("active")},
					BoundEquals{Field: "user_id", BoundVar: "bound.userId"},
				},
			},
			Bindings: map[string]string{"cart_id": "cartId"},
		},
		Right: Join{
			Left: Select{
				From: "cart_items",
				Filter: BoundEquals{
					Field:    "cart_id",
					BoundVar: "bound.cartId",
				},
				Bindings: map[string]string{"item_id": "itemId", "product_id": "productId"},
			},
			Right: Select{
				From:     "products",
				Bindings: map[string]string{"name": "productName", "price": "price"},
			},
			On: Equals{Field: "product_id", Value: ir.IRString("p1")},
		},
		On: Equals{Field: "cart_id", Value: ir.IRString("c1")},
	}

	result := Validate(query)

	assert.True(t, result.IsPortable, "complex query should be portable")
	assert.Empty(t, result.Warnings)
}

func TestValidationResult_String(t *testing.T) {
	// Verify ValidationResult fields are accessible
	result := ValidationResult{
		IsPortable: false,
		Warnings:   []string{"warning 1", "warning 2"},
	}

	assert.False(t, result.IsPortable)
	assert.Len(t, result.Warnings, 2)
	assert.Equal(t, "warning 1", result.Warnings[0])
	assert.Equal(t, "warning 2", result.Warnings[1])
}

func TestValidate_Idempotent(t *testing.T) {
	// Validate should be pure - calling twice gives same result
	query := Select{
		From:     "items",
		Bindings: map[string]string{}, // Violation
	}

	result1 := Validate(query)
	result2 := Validate(query)

	assert.Equal(t, result1.IsPortable, result2.IsPortable)
	assert.Equal(t, result1.Warnings, result2.Warnings)
}
