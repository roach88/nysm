package queryir

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/roach88/nysm/internal/ir"
)

func TestSelect_Construction(t *testing.T) {
	sel := Select{
		From: "CartItems",
		Filter: &Equals{
			Field: "status",
			Value: ir.IRString("active"),
		},
		Bindings: map[string]string{
			"item_id":  "itemId",
			"quantity": "qty",
		},
	}

	assert.Equal(t, "CartItems", sel.From)
	assert.NotNil(t, sel.Filter)
	assert.Len(t, sel.Bindings, 2)
}

func TestSelect_ImplementsQuery(t *testing.T) {
	var q Query = Select{From: "Test"}
	assert.NotNil(t, q)

	// Sealed interface - can type switch exhaustively
	switch q.(type) {
	case Select:
		// Expected
	case Join:
		t.Fatal("unexpected type")
	}
}

func TestSelect_NilFilter(t *testing.T) {
	// Filter is optional - nil means no filtering
	sel := Select{
		From:     "CartItems",
		Filter:   nil,
		Bindings: map[string]string{"item_id": "itemId"},
	}

	assert.Nil(t, sel.Filter)
}

func TestSelect_EmptyBindings(t *testing.T) {
	// Empty bindings map is valid (though unusual)
	sel := Select{
		From:     "CartItems",
		Bindings: map[string]string{},
	}

	assert.Empty(t, sel.Bindings)
}

func TestJoin_Construction(t *testing.T) {
	left := Select{From: "Carts", Bindings: map[string]string{"cart_id": "cartId"}}
	right := Select{From: "CartItems", Bindings: map[string]string{"item_id": "itemId"}}
	on := &Equals{Field: "cart_id", Value: ir.IRString("cart123")}

	join := Join{Left: left, Right: right, On: on}

	assert.NotNil(t, join.Left)
	assert.NotNil(t, join.Right)
	assert.NotNil(t, join.On)
}

func TestJoin_ImplementsQuery(t *testing.T) {
	var q Query = Join{
		Left:  Select{From: "Left"},
		Right: Select{From: "Right"},
		On:    &Equals{Field: "id", Value: ir.IRInt(1)},
	}
	assert.NotNil(t, q)
}

func TestJoin_NestedJoins(t *testing.T) {
	// Joins can be nested (recursive structure)
	inner := Join{
		Left:  Select{From: "A"},
		Right: Select{From: "B"},
		On:    &Equals{Field: "id", Value: ir.IRInt(1)},
	}

	outer := Join{
		Left:  inner,
		Right: Select{From: "C"},
		On:    &Equals{Field: "id", Value: ir.IRInt(2)},
	}

	assert.NotNil(t, outer.Left)
	assert.IsType(t, Join{}, outer.Left)
}

func TestEquals_Construction(t *testing.T) {
	eq := Equals{
		Field: "status",
		Value: ir.IRString("active"),
	}

	assert.Equal(t, "status", eq.Field)
	assert.Equal(t, ir.IRString("active"), eq.Value)
}

func TestEquals_ImplementsPredicate(t *testing.T) {
	var p Predicate = Equals{Field: "test", Value: ir.IRInt(1)}
	assert.NotNil(t, p)

	// Sealed interface - can type switch exhaustively
	switch p.(type) {
	case Equals:
		// Expected
	case BoundEquals:
		t.Fatal("unexpected type")
	case And:
		t.Fatal("unexpected type")
	}
}

func TestEquals_AllIRValueTypes(t *testing.T) {
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
			eq := Equals{Field: "field", Value: tt.value}
			assert.Equal(t, tt.value, eq.Value)
		})
	}
}

func TestBoundEquals_Construction(t *testing.T) {
	beq := BoundEquals{
		Field:    "cart_id",
		BoundVar: "bound.cartId",
	}

	assert.Equal(t, "cart_id", beq.Field)
	assert.Equal(t, "bound.cartId", beq.BoundVar)
}

func TestBoundEquals_ImplementsPredicate(t *testing.T) {
	var p Predicate = BoundEquals{Field: "test", BoundVar: "bound.var"}
	assert.NotNil(t, p)
}

func TestBoundEquals_BoundVarConvention(t *testing.T) {
	// BoundVar should follow "bound.varName" convention
	// (not enforced by type, but documented)
	beq := BoundEquals{
		Field:    "user_id",
		BoundVar: "bound.userId",
	}

	assert.Contains(t, beq.BoundVar, "bound.")
}

func TestAnd_Construction(t *testing.T) {
	and := And{
		Predicates: []Predicate{
			Equals{Field: "status", Value: ir.IRString("active")},
			BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
		},
	}

	assert.Len(t, and.Predicates, 2)
}

func TestAnd_ImplementsPredicate(t *testing.T) {
	var p Predicate = And{Predicates: []Predicate{}}
	assert.NotNil(t, p)
}

func TestAnd_EmptyPredicates(t *testing.T) {
	// Empty predicates means "always true" (vacuous truth)
	and := And{Predicates: []Predicate{}}
	assert.Empty(t, and.Predicates)
}

func TestAnd_NestedAnd(t *testing.T) {
	// And can contain nested And (though usually flattened)
	nested := And{
		Predicates: []Predicate{
			Equals{Field: "a", Value: ir.IRInt(1)},
			And{
				Predicates: []Predicate{
					Equals{Field: "b", Value: ir.IRInt(2)},
					Equals{Field: "c", Value: ir.IRInt(3)},
				},
			},
		},
	}

	assert.Len(t, nested.Predicates, 2)
	assert.IsType(t, And{}, nested.Predicates[1])
}

func TestQuery_SealedInterface(t *testing.T) {
	// Only Select and Join implement Query (sealed interface)
	queries := []Query{
		Select{From: "Test"},
		Join{
			Left:  Select{From: "Left"},
			Right: Select{From: "Right"},
			On:    Equals{Field: "id", Value: ir.IRInt(1)},
		},
	}

	for _, q := range queries {
		// Type switch is exhaustive - compiler knows all types
		switch q.(type) {
		case Select:
			// OK
		case Join:
			// OK
		default:
			t.Fatal("unexpected query type")
		}
	}
}

func TestPredicate_SealedInterface(t *testing.T) {
	// Only Equals, BoundEquals, And implement Predicate (sealed interface)
	predicates := []Predicate{
		Equals{Field: "test", Value: ir.IRInt(1)},
		BoundEquals{Field: "test", BoundVar: "bound.var"},
		And{Predicates: []Predicate{}},
	}

	for _, p := range predicates {
		// Type switch is exhaustive - compiler knows all types
		switch p.(type) {
		case Equals:
			// OK
		case BoundEquals:
			// OK
		case And:
			// OK
		default:
			t.Fatal("unexpected predicate type")
		}
	}
}

func TestSelect_JSONMarshaling(t *testing.T) {
	sel := Select{
		From: "CartItems",
		Filter: Equals{
			Field: "status",
			Value: ir.IRString("active"),
		},
		Bindings: map[string]string{
			"item_id": "itemId",
		},
	}

	// Marshal to JSON (for IR serialization)
	data, err := json.Marshal(sel)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"From":"CartItems"`)
	assert.Contains(t, string(data), `"item_id":"itemId"`)
}

func TestComplexQuery_Construction(t *testing.T) {
	// Complex query: Select with And filter containing multiple conditions
	query := Select{
		From: "CartItems",
		Filter: And{
			Predicates: []Predicate{
				BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
				Equals{Field: "status", Value: ir.IRString("active")},
				Equals{Field: "quantity", Value: ir.IRInt(1)},
			},
		},
		Bindings: map[string]string{
			"item_id":  "itemId",
			"quantity": "qty",
			"price":    "price",
		},
	}

	assert.Equal(t, "CartItems", query.From)
	assert.IsType(t, And{}, query.Filter)
	assert.Len(t, query.Bindings, 3)

	and := query.Filter.(And)
	assert.Len(t, and.Predicates, 3)
}

func TestComplexQuery_JoinWithFilters(t *testing.T) {
	// Join two selects, each with filters
	query := Join{
		Left: Select{
			From: "Carts",
			Filter: Equals{
				Field: "status",
				Value: ir.IRString("active"),
			},
			Bindings: map[string]string{"cart_id": "cartId"},
		},
		Right: Select{
			From: "CartItems",
			Filter: And{
				Predicates: []Predicate{
					BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
					Equals{Field: "quantity", Value: ir.IRInt(1)},
				},
			},
			Bindings: map[string]string{"item_id": "itemId"},
		},
		On: Equals{Field: "cart_id", Value: ir.IRString("cart123")},
	}

	assert.IsType(t, Select{}, query.Left)
	assert.IsType(t, Select{}, query.Right)
	assert.IsType(t, Equals{}, query.On)
}

func TestSelect_PointerVariants(t *testing.T) {
	// Test that both value and pointer types work for Select
	sel := &Select{
		From:     "Test",
		Bindings: map[string]string{"id": "id"},
	}

	var q Query = sel
	assert.NotNil(t, q)

	// Type switch with pointers
	switch q.(type) {
	case *Select:
		// Expected - pointer type
	case Select:
		t.Fatal("expected pointer type, got value type")
	}
}

func TestJoin_PointerVariants(t *testing.T) {
	// Test that both value and pointer types work for Join
	join := &Join{
		Left:  Select{From: "Left"},
		Right: Select{From: "Right"},
		On:    Equals{Field: "id", Value: ir.IRInt(1)},
	}

	var q Query = join
	assert.NotNil(t, q)

	// Type switch with pointers
	switch q.(type) {
	case *Join:
		// Expected - pointer type
	case Join:
		t.Fatal("expected pointer type, got value type")
	}
}

func TestEquals_PointerVariants(t *testing.T) {
	// Test that both value and pointer types work for Equals
	eq := &Equals{
		Field: "test",
		Value: ir.IRInt(1),
	}

	var p Predicate = eq
	assert.NotNil(t, p)

	// Type switch with pointers
	switch p.(type) {
	case *Equals:
		// Expected - pointer type
	case Equals:
		t.Fatal("expected pointer type, got value type")
	}
}

func TestBoundEquals_PointerVariants(t *testing.T) {
	// Test that both value and pointer types work for BoundEquals
	beq := &BoundEquals{
		Field:    "test",
		BoundVar: "bound.var",
	}

	var p Predicate = beq
	assert.NotNil(t, p)

	// Type switch with pointers
	switch p.(type) {
	case *BoundEquals:
		// Expected - pointer type
	case BoundEquals:
		t.Fatal("expected pointer type, got value type")
	}
}

func TestAnd_PointerVariants(t *testing.T) {
	// Test that both value and pointer types work for And
	and := &And{
		Predicates: []Predicate{
			Equals{Field: "a", Value: ir.IRInt(1)},
		},
	}

	var p Predicate = and
	assert.NotNil(t, p)

	// Type switch with pointers
	switch p.(type) {
	case *And:
		// Expected - pointer type
	case And:
		t.Fatal("expected pointer type, got value type")
	}
}

func TestQuery_MarkerMethodExists(t *testing.T) {
	// Verify the marker method exists and is callable
	sel := Select{From: "Test"}
	sel.queryNode()

	join := Join{Left: sel, Right: sel, On: Equals{Field: "id", Value: ir.IRInt(1)}}
	join.queryNode()
}

func TestPredicate_MarkerMethodExists(t *testing.T) {
	// Verify the marker method exists and is callable
	eq := Equals{Field: "test", Value: ir.IRInt(1)}
	eq.predicateNode()

	beq := BoundEquals{Field: "test", BoundVar: "bound.var"}
	beq.predicateNode()

	and := And{Predicates: []Predicate{}}
	and.predicateNode()
}

func TestSelect_WithNullValue(t *testing.T) {
	// IRNull is valid in IRValue - though NULLs are discouraged in portable fragment
	sel := Select{
		From: "CartItems",
		Filter: Equals{
			Field: "deleted_at",
			Value: ir.IRNull{},
		},
		Bindings: map[string]string{"item_id": "itemId"},
	}

	assert.NotNil(t, sel.Filter)
	eq := sel.Filter.(Equals)
	assert.IsType(t, ir.IRNull{}, eq.Value)
}

func TestAnd_SinglePredicate(t *testing.T) {
	// And with single predicate - valid but unusual
	and := And{
		Predicates: []Predicate{
			Equals{Field: "status", Value: ir.IRString("active")},
		},
	}

	assert.Len(t, and.Predicates, 1)
}

func TestJoin_WithNilOn(t *testing.T) {
	// On can be nil (though not recommended in portable fragment)
	join := Join{
		Left:  Select{From: "Left"},
		Right: Select{From: "Right"},
		On:    nil,
	}

	assert.Nil(t, join.On)
}

func TestDeepNesting(t *testing.T) {
	// Test deeply nested structure
	innerAnd := And{
		Predicates: []Predicate{
			Equals{Field: "a", Value: ir.IRInt(1)},
			Equals{Field: "b", Value: ir.IRInt(2)},
		},
	}

	outerAnd := And{
		Predicates: []Predicate{
			innerAnd,
			Equals{Field: "c", Value: ir.IRInt(3)},
		},
	}

	innerJoin := Join{
		Left:  Select{From: "A", Filter: innerAnd},
		Right: Select{From: "B", Filter: outerAnd},
		On:    Equals{Field: "id", Value: ir.IRInt(1)},
	}

	outerJoin := Join{
		Left:  innerJoin,
		Right: Select{From: "C"},
		On:    BoundEquals{Field: "id", BoundVar: "bound.id"},
	}

	// Verify structure is valid
	assert.IsType(t, Join{}, outerJoin.Left)
	assert.IsType(t, Select{}, outerJoin.Right)
	assert.IsType(t, BoundEquals{}, outerJoin.On)
}
