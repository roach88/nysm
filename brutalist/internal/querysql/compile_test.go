package querysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/queryir"
)

func TestCompile_SimpleSelect(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From: "inventory",
		Bindings: map[string]string{
			"name":     "item",
			"quantity": "qty",
		},
		Filter: queryir.Equals{
			Field: "category",
			Value: ir.IRString("widgets"),
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify SQL structure
	assert.Contains(t, sql, "SELECT")
	assert.Contains(t, sql, "FROM inventory")
	assert.Contains(t, sql, "WHERE category = ?")
	assert.Contains(t, sql, "ORDER BY") // MANDATORY per CP-4

	// Verify parameterized query (no interpolation)
	assert.NotContains(t, sql, "widgets") // Value NOT in SQL
	assert.Equal(t, []any{"widgets"}, params)

	// Verify COLLATE BINARY for deterministic ordering
	assert.Contains(t, sql, "COLLATE BINARY")
}

func TestCompile_SimpleSelectPointer(t *testing.T) {
	compiler := NewSQLCompiler()

	query := &queryir.Select{
		From: "inventory",
		Bindings: map[string]string{
			"name": "item",
		},
		Filter: &queryir.Equals{
			Field: "category",
			Value: ir.IRString("widgets"),
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "FROM inventory")
	assert.Contains(t, sql, "WHERE category = ?")
	assert.Equal(t, []any{"widgets"}, params)
}

func TestCompile_OrderByMandatory(t *testing.T) {
	compiler := NewSQLCompiler()

	testCases := []struct {
		name  string
		query queryir.Query
	}{
		{
			name: "select with filter",
			query: queryir.Select{
				From:     "inventory",
				Bindings: map[string]string{"id": "id"},
				Filter:   queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
			},
		},
		{
			name: "select without filter",
			query: queryir.Select{
				From:     "inventory",
				Bindings: map[string]string{"id": "id"},
			},
		},
		{
			name: "select with And predicate",
			query: queryir.Select{
				From:     "inventory",
				Bindings: map[string]string{"id": "id"},
				Filter: queryir.And{
					Predicates: []queryir.Predicate{
						queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
						queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
					},
				},
			},
		},
		{
			name: "select with empty bindings",
			query: queryir.Select{
				From: "inventory",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sql, _, err := compiler.Compile(tc.query)
			require.NoError(t, err)

			// CRITICAL: Every query MUST have ORDER BY per CP-4
			assert.Contains(t, sql, "ORDER BY",
				"Query MUST include ORDER BY per CP-4: %s", sql)
			assert.Contains(t, sql, "COLLATE BINARY",
				"ORDER BY MUST use COLLATE BINARY: %s", sql)
		})
	}
}

func TestCompile_NoStringInterpolation(t *testing.T) {
	compiler := NewSQLCompiler()

	// Use a value that would be dangerous if interpolated
	dangerousValue := "'; DROP TABLE inventory; --"

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{"id": "id"},
		Filter: queryir.Equals{
			Field: "category",
			Value: ir.IRString(dangerousValue),
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify SQL does NOT contain the dangerous value
	assert.NotContains(t, sql, dangerousValue,
		"Value MUST NOT be interpolated into SQL (SQL injection risk)")

	// Verify value is in parameters
	assert.Contains(t, params, dangerousValue,
		"Value MUST be in parameters array")

	// Verify SQL uses ? placeholder
	assert.Contains(t, sql, "category = ?",
		"SQL MUST use ? placeholder, not interpolated value")
}

func TestCompile_AndPredicate(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{"id": "id"},
		Filter: queryir.And{
			Predicates: []queryir.Predicate{
				queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
				queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
				queryir.Equals{Field: "quantity", Value: ir.IRInt(10)},
			},
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify WHERE clause with AND
	assert.Contains(t, sql, "WHERE")
	assert.Contains(t, sql, "category = ?")
	assert.Contains(t, sql, " AND ")
	assert.Contains(t, sql, "in_stock = ?")
	assert.Contains(t, sql, "quantity = ?")

	// Verify parameters in order
	assert.Equal(t, []any{"widgets", true, int64(10)}, params)

	// Verify ORDER BY present
	assert.Contains(t, sql, "ORDER BY")
}

func TestCompile_AndPredicatePointer(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{"id": "id"},
		Filter: &queryir.And{
			Predicates: []queryir.Predicate{
				&queryir.Equals{Field: "a", Value: ir.IRString("x")},
				&queryir.Equals{Field: "b", Value: ir.IRInt(1)},
			},
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "a = ?")
	assert.Contains(t, sql, "AND")
	assert.Contains(t, sql, "b = ?")
	assert.Equal(t, []any{"x", int64(1)}, params)
}

func TestCompile_EmptyAndPredicate(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{"id": "id"},
		Filter: queryir.And{
			Predicates: []queryir.Predicate{}, // Empty = always true
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Empty And should produce "1 = 1" (always true)
	assert.Contains(t, sql, "WHERE 1 = 1")
	assert.Empty(t, params)
	assert.Contains(t, sql, "ORDER BY") // Still has ORDER BY
}

func TestCompile_BoundEquals(t *testing.T) {
	compiler := NewSQLCompiler()
	compiler.BoundValues = map[string]any{
		"bound.cartId": "cart-123",
	}

	query := queryir.Select{
		From:     "cart_items",
		Bindings: map[string]string{"item_id": "itemId"},
		Filter: queryir.BoundEquals{
			Field:    "cart_id",
			BoundVar: "bound.cartId",
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "cart_id = ?")
	assert.Equal(t, []any{"cart-123"}, params)
}

func TestCompile_BoundEqualsPointer(t *testing.T) {
	compiler := NewSQLCompiler()
	compiler.BoundValues = map[string]any{
		"bound.userId": int64(42),
	}

	query := queryir.Select{
		From:     "items",
		Bindings: map[string]string{"id": "id"},
		Filter: &queryir.BoundEquals{
			Field:    "user_id",
			BoundVar: "bound.userId",
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "user_id = ?")
	assert.Equal(t, []any{int64(42)}, params)
}

func TestCompile_BoundEqualsNotFound(t *testing.T) {
	compiler := NewSQLCompiler()
	// No bound values set

	query := queryir.Select{
		From:     "items",
		Bindings: map[string]string{"id": "id"},
		Filter: queryir.BoundEquals{
			Field:    "user_id",
			BoundVar: "bound.unknownVar",
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// SQL is still generated, but params will be empty
	assert.Contains(t, sql, "user_id = ?")
	assert.Empty(t, params)
}

func TestCompile_InnerJoin(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Join{
		Left:  queryir.Select{From: "orders"},
		Right: queryir.Select{From: "customers"},
		On: queryir.Equals{
			Field: "customer_id",
			Value: ir.IRString("cust-123"),
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify INNER JOIN syntax
	assert.Contains(t, sql, "INNER JOIN")
	assert.Contains(t, sql, "orders")
	assert.Contains(t, sql, "customers")
	assert.Contains(t, sql, "ON")
	assert.Contains(t, sql, "customer_id = ?")

	// Verify parameters
	assert.Equal(t, []any{"cust-123"}, params)

	// Verify ORDER BY present (even on joins)
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "COLLATE BINARY")
}

func TestCompile_InnerJoinPointer(t *testing.T) {
	compiler := NewSQLCompiler()

	query := &queryir.Join{
		Left:  &queryir.Select{From: "orders"},
		Right: &queryir.Select{From: "customers"},
		On: &queryir.Equals{
			Field: "customer_id",
			Value: ir.IRInt(123),
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "INNER JOIN")
	assert.Contains(t, sql, "customer_id = ?")
	assert.Equal(t, []any{int64(123)}, params)
}

func TestCompile_JoinWithNilOn(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Join{
		Left:  queryir.Select{From: "orders"},
		Right: queryir.Select{From: "customers"},
		On:    nil, // Cross join
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	// Cross join uses "1 = 1" as ON condition
	assert.Contains(t, sql, "INNER JOIN")
	assert.Contains(t, sql, "ON 1 = 1")
	assert.Empty(t, params)
}

func TestCompile_EmptyFilter(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{"id": "id"},
		Filter:   nil, // No filter
	}

	sql, _, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify no WHERE clause
	assert.NotContains(t, sql, "WHERE")

	// Verify ORDER BY STILL present (mandatory per CP-4)
	assert.Contains(t, sql, "ORDER BY",
		"ORDER BY MUST be present even without WHERE clause")
}

func TestCompile_BindingsAlias(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From: "inventory",
		Bindings: map[string]string{
			"item_name": "itemName", // Different: needs AS
			"quantity":  "quantity", // Same: no AS needed
		},
	}

	sql, _, err := compiler.Compile(query)
	require.NoError(t, err)

	// Verify aliases
	assert.Contains(t, sql, "item_name AS itemName")
	assert.Contains(t, sql, "quantity") // No AS since same name
}

func TestCompile_EmptyBindings(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: map[string]string{}, // Empty = SELECT *
	}

	sql, _, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "SELECT * FROM inventory")
}

func TestCompile_NilBindings(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "inventory",
		Bindings: nil, // nil = SELECT *
	}

	sql, _, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "SELECT * FROM inventory")
}

func TestCompile_AllIRValueTypes(t *testing.T) {
	tests := []struct {
		name     string
		value    ir.IRValue
		expected any
	}{
		{"string", ir.IRString("test"), "test"},
		{"int", ir.IRInt(42), int64(42)},
		{"bool_true", ir.IRBool(true), true},
		{"bool_false", ir.IRBool(false), false},
		{"null", ir.IRNull{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := NewSQLCompiler()

			query := queryir.Select{
				From:     "items",
				Bindings: map[string]string{"id": "id"},
				Filter: queryir.Equals{
					Field: "field",
					Value: tt.value,
				},
			}

			sql, params, err := compiler.Compile(query)
			require.NoError(t, err)

			assert.Contains(t, sql, "field = ?")
			require.Len(t, params, 1)
			assert.Equal(t, tt.expected, params[0])
		})
	}
}

func TestCompile_UnsupportedIRValueTypes(t *testing.T) {
	tests := []struct {
		name  string
		value ir.IRValue
	}{
		{"array", ir.IRArray{ir.IRInt(1), ir.IRInt(2)}},
		{"object", ir.IRObject{"key": ir.IRString("value")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compiler := NewSQLCompiler()

			query := queryir.Select{
				From:     "items",
				Bindings: map[string]string{"id": "id"},
				Filter: queryir.Equals{
					Field: "field",
					Value: tt.value,
				},
			}

			_, _, err := compiler.Compile(query)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "cannot be used as SQL parameter")
		})
	}
}

func TestCompile_NilQuery(t *testing.T) {
	compiler := NewSQLCompiler()

	_, _, err := compiler.Compile(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "nil query")
}

func TestCompile_NestedAndPredicate(t *testing.T) {
	compiler := NewSQLCompiler()

	query := queryir.Select{
		From:     "items",
		Bindings: map[string]string{"id": "id"},
		Filter: queryir.And{
			Predicates: []queryir.Predicate{
				queryir.Equals{Field: "a", Value: ir.IRInt(1)},
				queryir.And{
					Predicates: []queryir.Predicate{
						queryir.Equals{Field: "b", Value: ir.IRInt(2)},
						queryir.Equals{Field: "c", Value: ir.IRInt(3)},
					},
				},
			},
		},
	}

	sql, params, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Contains(t, sql, "a = ?")
	assert.Contains(t, sql, "AND")
	assert.Contains(t, sql, "b = ?")
	assert.Contains(t, sql, "c = ?")
	assert.Equal(t, []any{int64(1), int64(2), int64(3)}, params)
}

func TestCompile_GoldenSQL(t *testing.T) {
	compiler := NewSQLCompiler()

	testCases := []struct {
		name       string
		query      queryir.Query
		wantSQL    string
		wantParams []any
	}{
		{
			name: "simple select with alias",
			query: queryir.Select{
				From:     "inventory",
				Bindings: map[string]string{"name": "item"},
				Filter:   queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
			},
			wantSQL:    "SELECT name AS item FROM inventory WHERE category = ? ORDER BY id ASC COLLATE BINARY",
			wantParams: []any{"widgets"},
		},
		{
			name: "select with And",
			query: queryir.Select{
				From: "inventory",
				Filter: queryir.And{
					Predicates: []queryir.Predicate{
						queryir.Equals{Field: "category", Value: ir.IRString("widgets")},
						queryir.Equals{Field: "in_stock", Value: ir.IRBool(true)},
					},
				},
			},
			wantSQL:    "SELECT * FROM inventory WHERE category = ? AND in_stock = ? ORDER BY id ASC COLLATE BINARY",
			wantParams: []any{"widgets", true},
		},
		{
			name: "select without filter",
			query: queryir.Select{
				From:     "inventory",
				Bindings: map[string]string{"id": "id"},
			},
			wantSQL:    "SELECT id FROM inventory ORDER BY id ASC COLLATE BINARY",
			wantParams: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sql, params, err := compiler.Compile(tc.query)
			require.NoError(t, err)

			assert.Equal(t, tc.wantSQL, sql, "SQL mismatch")
			assert.Equal(t, tc.wantParams, params, "Parameters mismatch")
		})
	}
}

func TestCompile_BindingsDeterministicOrder(t *testing.T) {
	compiler := NewSQLCompiler()

	// Multiple bindings - should be sorted alphabetically
	query := queryir.Select{
		From: "items",
		Bindings: map[string]string{
			"zebra":    "z",
			"apple":    "a",
			"mango":    "m",
			"banana":   "b",
			"cherry":   "c",
			"same":     "same",
			"diffname": "alias",
		},
	}

	sql1, _, err := compiler.Compile(query)
	require.NoError(t, err)

	// Compile again - should produce identical SQL
	sql2, _, err := compiler.Compile(query)
	require.NoError(t, err)

	assert.Equal(t, sql1, sql2, "SQL should be deterministic")

	// Verify alphabetical order in SELECT clause
	appleIdx := indexOf(sql1, "apple")
	bananaIdx := indexOf(sql1, "banana")
	cherryIdx := indexOf(sql1, "cherry")
	mangoIdx := indexOf(sql1, "mango")
	zebraIdx := indexOf(sql1, "zebra")

	assert.Less(t, appleIdx, bananaIdx)
	assert.Less(t, bananaIdx, cherryIdx)
	assert.Less(t, cherryIdx, mangoIdx)
	assert.Less(t, mangoIdx, zebraIdx)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestIRValueToParam(t *testing.T) {
	tests := []struct {
		name     string
		value    ir.IRValue
		expected any
		wantErr  bool
	}{
		{"string", ir.IRString("test"), "test", false},
		{"int", ir.IRInt(42), int64(42), false},
		{"bool_true", ir.IRBool(true), true, false},
		{"bool_false", ir.IRBool(false), false, false},
		{"null", ir.IRNull{}, nil, false},
		{"array", ir.IRArray{ir.IRInt(1)}, nil, true},
		{"object", ir.IRObject{"k": ir.IRString("v")}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := irValueToParam(tt.value)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
