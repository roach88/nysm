package engine

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/queryir"
)

// TestParseFilterExpression tests filter expression parsing.
func TestParseFilterExpression(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		expected queryir.Predicate
		wantErr  bool
	}{
		{
			name:     "empty filter",
			filter:   "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:   "simple string equals",
			filter: "status == 'active'",
			expected: queryir.Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
			wantErr: false,
		},
		{
			name:   "simple string equals double quotes",
			filter: `status == "active"`,
			expected: queryir.Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
			wantErr: false,
		},
		{
			name:   "integer equals",
			filter: "quantity == 42",
			expected: queryir.Equals{
				Field: "quantity",
				Value: ir.IRInt(42),
			},
			wantErr: false,
		},
		{
			name:   "negative integer equals",
			filter: "balance == -100",
			expected: queryir.Equals{
				Field: "balance",
				Value: ir.IRInt(-100),
			},
			wantErr: false,
		},
		{
			name:   "boolean true equals",
			filter: "active == true",
			expected: queryir.Equals{
				Field: "active",
				Value: ir.IRBool(true),
			},
			wantErr: false,
		},
		{
			name:   "boolean false equals",
			filter: "deleted == false",
			expected: queryir.Equals{
				Field: "deleted",
				Value: ir.IRBool(false),
			},
			wantErr: false,
		},
		{
			name:   "bound variable reference",
			filter: "cart_id == bound.cartId",
			expected: queryir.BoundEquals{
				Field:    "cart_id",
				BoundVar: "bound.cartId",
			},
			wantErr: false,
		},
		{
			name:   "AND expression",
			filter: "status == 'active' AND cart_id == bound.cartId",
			expected: queryir.And{
				Predicates: []queryir.Predicate{
					queryir.Equals{Field: "status", Value: ir.IRString("active")},
					queryir.BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
				},
			},
			wantErr: false,
		},
		{
			name:   "case insensitive AND",
			filter: "status == 'active' and cart_id == bound.cartId",
			expected: queryir.And{
				Predicates: []queryir.Predicate{
					queryir.Equals{Field: "status", Value: ir.IRString("active")},
					queryir.BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
				},
			},
			wantErr: false,
		},
		{
			name:   "multiple AND expressions",
			filter: "a == 1 AND b == 2 AND c == 3",
			expected: queryir.And{
				Predicates: []queryir.Predicate{
					queryir.Equals{Field: "a", Value: ir.IRInt(1)},
					queryir.Equals{Field: "b", Value: ir.IRInt(2)},
					queryir.Equals{Field: "c", Value: ir.IRInt(3)},
				},
			},
			wantErr: false,
		},
		{
			name:   "single equals operator",
			filter: "status = 'active'",
			expected: queryir.Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
			wantErr: false,
		},
		{
			name:    "unsupported != operator",
			filter:  "status != 'deleted'",
			wantErr: true,
		},
		{
			name:    "no operator",
			filter:  "status active",
			wantErr: true,
		},
		{
			name:   "unquoted string literal",
			filter: "status == active",
			expected: queryir.Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFilterExpression(tt.filter)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSplitByAnd tests AND splitting.
func TestSplitByAnd(t *testing.T) {
	tests := []struct {
		name     string
		filter   string
		expected []string
	}{
		{
			name:     "no AND",
			filter:   "status == 'active'",
			expected: []string{"status == 'active'"},
		},
		{
			name:     "single AND",
			filter:   "a == 1 AND b == 2",
			expected: []string{"a == 1", "b == 2"},
		},
		{
			name:     "multiple ANDs",
			filter:   "a == 1 AND b == 2 AND c == 3",
			expected: []string{"a == 1", "b == 2", "c == 3"},
		},
		{
			name:     "lowercase and",
			filter:   "a == 1 and b == 2",
			expected: []string{"a == 1", "b == 2"},
		},
		{
			name:     "mixed case",
			filter:   "a == 1 AND b == 2 and c == 3",
			expected: []string{"a == 1", "b == 2", "c == 3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitByAnd(tt.filter)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsNumeric tests numeric string detection.
func TestIsNumeric(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"42", true},
		{"-42", true},
		{"+42", true},
		{"0", true},
		{"123456789", true},
		{"", false},
		{"abc", false},
		{"12abc", false},
		{"12.34", false}, // Floats not supported
		{"-", false},
		{"+", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumeric(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseInt tests integer parsing.
func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"42", 42},
		{"-42", -42},
		{"+42", 42},
		{"0", 0},
		{"123456789", 123456789},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseInt(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSqlToIRValue tests SQL to IR value conversion.
func TestSqlToIRValue(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected ir.IRValue
		wantErr  bool
	}{
		{
			name:     "nil to IRNull",
			input:    nil,
			expected: ir.IRNull{},
			wantErr:  false,
		},
		{
			name:     "int64 to IRInt",
			input:    int64(42),
			expected: ir.IRInt(42),
			wantErr:  false,
		},
		{
			name:     "int to IRInt",
			input:    int(42),
			expected: ir.IRInt(42),
			wantErr:  false,
		},
		{
			name:     "string to IRString",
			input:    "hello",
			expected: ir.IRString("hello"),
			wantErr:  false,
		},
		{
			name:     "[]byte to IRString",
			input:    []byte("hello"),
			expected: ir.IRString("hello"),
			wantErr:  false,
		},
		{
			name:     "bool true to IRBool",
			input:    true,
			expected: ir.IRBool(true),
			wantErr:  false,
		},
		{
			name:     "bool false to IRBool",
			input:    false,
			expected: ir.IRBool(false),
			wantErr:  false,
		},
		{
			name:    "float64 forbidden (CP-5)",
			input:   float64(3.14),
			wantErr: true, // Floats break determinism
		},
		{
			name:    "unsupported type",
			input:   struct{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sqlToIRValue(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIrValueToSQLParam tests IR to SQL parameter conversion.
func TestIrValueToSQLParam(t *testing.T) {
	tests := []struct {
		name     string
		input    ir.IRValue
		expected any
		wantErr  bool
	}{
		{
			name:     "IRString to string",
			input:    ir.IRString("hello"),
			expected: "hello",
			wantErr:  false,
		},
		{
			name:     "IRInt to int64",
			input:    ir.IRInt(42),
			expected: int64(42),
			wantErr:  false,
		},
		{
			name:     "IRBool true",
			input:    ir.IRBool(true),
			expected: true,
			wantErr:  false,
		},
		{
			name:     "IRBool false",
			input:    ir.IRBool(false),
			expected: false,
			wantErr:  false,
		},
		{
			name:     "IRNull to nil",
			input:    ir.IRNull{},
			expected: nil,
			wantErr:  false,
		},
		{
			name:    "IRArray unsupported",
			input:   ir.IRArray{ir.IRInt(1)},
			wantErr: true,
		},
		{
			name:    "IRObject unsupported",
			input:   ir.IRObject{"key": ir.IRString("value")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := irValueToSQLParam(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseSingleComparison tests single comparison parsing.
func TestParseSingleComparison(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected queryir.Predicate
		wantErr  bool
	}{
		{
			name: "string with single quotes",
			expr: "status == 'active'",
			expected: queryir.Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
		},
		{
			name: "string with double quotes",
			expr: `name == "test"`,
			expected: queryir.Equals{
				Field: "name",
				Value: ir.IRString("test"),
			},
		},
		{
			name: "integer",
			expr: "count == 42",
			expected: queryir.Equals{
				Field: "count",
				Value: ir.IRInt(42),
			},
		},
		{
			name: "bound variable",
			expr: "user_id == bound.userId",
			expected: queryir.BoundEquals{
				Field:    "user_id",
				BoundVar: "bound.userId",
			},
		},
		{
			name: "boolean true",
			expr: "active == true",
			expected: queryir.Equals{
				Field: "active",
				Value: ir.IRBool(true),
			},
		},
		{
			name: "boolean false",
			expr: "deleted == false",
			expected: queryir.Equals{
				Field: "deleted",
				Value: ir.IRBool(false),
			},
		},
		{
			name:    "no equals operator",
			expr:    "status active",
			wantErr: true,
		},
		{
			name:    "!= not supported",
			expr:    "status != deleted",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseSingleComparison(tt.expr)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// mockStore implements the interface needed by Engine for testing.
type mockStore struct {
	rows   *sql.Rows
	err    error
	lastQ  string
	lastA  []any
	closed bool
}

func (m *mockStore) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	m.lastQ = query
	m.lastA = args
	return m.rows, m.err
}

func (m *mockStore) Close() error {
	m.closed = true
	return nil
}

// TestBuildQueryFromWhere tests building QueryIR from WhereClause.
func TestBuildQueryFromWhere(t *testing.T) {
	e := &Engine{}

	tests := []struct {
		name         string
		where        *ir.WhereClause
		whenBindings ir.IRObject
		wantErr      bool
	}{
		{
			name: "simple where clause",
			where: &ir.WhereClause{
				Source: "cart_items",
				Filter: "status == 'active'",
				Bindings: map[string]string{
					"item_id": "itemId",
				},
			},
			whenBindings: ir.IRObject{"cartId": ir.IRString("cart-123")},
			wantErr:      false,
		},
		{
			name: "where with bound variable",
			where: &ir.WhereClause{
				Source: "cart_items",
				Filter: "cart_id == bound.cartId",
				Bindings: map[string]string{
					"item_id": "itemId",
				},
			},
			whenBindings: ir.IRObject{"cartId": ir.IRString("cart-123")},
			wantErr:      false,
		},
		{
			name: "where with no filter",
			where: &ir.WhereClause{
				Source: "cart_items",
				Filter: "",
				Bindings: map[string]string{
					"item_id": "itemId",
				},
			},
			whenBindings: ir.IRObject{},
			wantErr:      false,
		},
		{
			name: "where with AND filter",
			where: &ir.WhereClause{
				Source: "cart_items",
				Filter: "status == 'active' AND cart_id == bound.cartId",
				Bindings: map[string]string{
					"item_id": "itemId",
				},
			},
			whenBindings: ir.IRObject{"cartId": ir.IRString("cart-123")},
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := e.buildQueryFromWhere(tt.where, tt.whenBindings)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, query)

			// Verify it's a Select query
			sel, ok := query.(queryir.Select)
			require.True(t, ok, "expected queryir.Select")
			assert.Equal(t, tt.where.Source, sel.From)
			assert.Equal(t, tt.where.Bindings, sel.Bindings)
		})
	}
}

// TestExecuteWhereNilClause tests executeWhere with nil where-clause.
func TestExecuteWhereNilClause(t *testing.T) {
	e := &Engine{}
	ctx := context.Background()
	whenBindings := ir.IRObject{
		"cartId": ir.IRString("cart-123"),
		"userId": ir.IRInt(42),
	}

	// When where-clause is nil, should return single binding set with when-bindings
	result, err := e.executeWhere(ctx, nil, whenBindings, "")

	require.NoError(t, err)
	require.Len(t, result, 1, "nil where-clause should return single binding set")
	assert.Equal(t, whenBindings, result[0])
}

// TestMergeBindings_FromScope tests the mergeBindings function from scope.go.
func TestMergeBindings_FromScope(t *testing.T) {
	tests := []struct {
		name     string
		when     ir.IRObject
		where    ir.IRObject
		expected ir.IRObject
	}{
		{
			name:     "both nil",
			when:     nil,
			where:    nil,
			expected: ir.IRObject{},
		},
		{
			name:     "when nil",
			when:     nil,
			where:    ir.IRObject{"a": ir.IRInt(1)},
			expected: ir.IRObject{"a": ir.IRInt(1)},
		},
		{
			name:     "where nil",
			when:     ir.IRObject{"a": ir.IRInt(1)},
			where:    nil,
			expected: ir.IRObject{"a": ir.IRInt(1)},
		},
		{
			name:     "no overlap",
			when:     ir.IRObject{"a": ir.IRInt(1)},
			where:    ir.IRObject{"b": ir.IRInt(2)},
			expected: ir.IRObject{"a": ir.IRInt(1), "b": ir.IRInt(2)},
		},
		{
			name:     "where overrides when",
			when:     ir.IRObject{"a": ir.IRInt(1)},
			where:    ir.IRObject{"a": ir.IRInt(999)}, // Override
			expected: ir.IRObject{"a": ir.IRInt(999)},
		},
		{
			name: "complex merge",
			when: ir.IRObject{
				"cartId": ir.IRString("cart-1"),
				"userId": ir.IRInt(42),
			},
			where: ir.IRObject{
				"itemId":    ir.IRString("item-1"),
				"productId": ir.IRString("prod-1"),
			},
			expected: ir.IRObject{
				"cartId":    ir.IRString("cart-1"),
				"userId":    ir.IRInt(42),
				"itemId":    ir.IRString("item-1"),
				"productId": ir.IRString("prod-1"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeBindings(tt.when, tt.where)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestParseFilterExpressionWhitespace tests whitespace handling.
func TestParseFilterExpressionWhitespace(t *testing.T) {
	tests := []struct {
		name   string
		filter string
	}{
		{"leading whitespace", "  status == 'active'"},
		{"trailing whitespace", "status == 'active'  "},
		{"both whitespace", "  status == 'active'  "},
		{"extra spaces around ==", "status  ==  'active'"},
		{"tabs", "\tstatus == 'active'\t"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseFilterExpression(tt.filter)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Should parse to same result regardless of whitespace
			equals, ok := result.(queryir.Equals)
			require.True(t, ok)
			assert.Equal(t, "status", equals.Field)
			assert.Equal(t, ir.IRString("active"), equals.Value)
		})
	}
}

// TestIRValueConversionRoundTrip tests that values survive IR -> SQL -> IR.
func TestIRValueConversionRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value ir.IRValue
	}{
		{"string", ir.IRString("hello world")},
		{"empty string", ir.IRString("")},
		{"int positive", ir.IRInt(42)},
		{"int negative", ir.IRInt(-42)},
		{"int zero", ir.IRInt(0)},
		{"bool true", ir.IRBool(true)},
		{"bool false", ir.IRBool(false)},
		{"null", ir.IRNull{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// IR -> SQL param
			sqlVal, err := irValueToSQLParam(tt.value)
			require.NoError(t, err)

			// SQL -> IR (simulating what scanBinding does)
			irVal, err := sqlToIRValue(sqlVal)
			require.NoError(t, err)

			assert.Equal(t, tt.value, irVal, "round-trip should preserve value")
		})
	}
}
