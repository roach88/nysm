# Story 4.1: QueryIR Type System

Status: ready-for-dev

## Story

As a **developer building the query layer**,
I want **an abstract QueryIR between DSL and SQL**,
So that **I can migrate to SPARQL later without rewriting syncs**.

## Acceptance Criteria

1. **QueryIR types defined in `internal/queryir/types.go`**
   ```go
   // Query interface - sealed interface for query nodes
   type Query interface {
       queryNode() // Marker method - only types in this package implement
   }

   // Predicate interface - sealed interface for filter predicates
   type Predicate interface {
       predicateNode() // Marker method
   }
   ```
   - Sealed interfaces prevent external implementations
   - All query types implement Query interface
   - All predicate types implement Predicate interface

2. **Select query type for basic table access**
   ```go
   type Select struct {
       From     string            // Table/source name (e.g., "CartItems")
       Filter   Predicate         // WHERE conditions (optional, nil = all rows)
       Bindings map[string]string // field → bound variable (e.g., "item_id" → "itemId")
   }
   func (Select) queryNode() {}
   ```
   - From references state table name from concept spec
   - Filter uses Predicate types for WHERE conditions
   - Bindings map source fields to variable names in sync rule

3. **Join query type for combining queries**
   ```go
   type Join struct {
       Left  Query     // Left query (must be Query interface)
       Right Query     // Right query (must be Query interface)
       On    Predicate // Join condition (equi-join only in portable fragment)
   }
   func (Join) queryNode() {}
   ```
   - Supports inner joins only (outer joins not in portable fragment)
   - On predicate typically Equals or And of Equals predicates
   - Recursive structure allows nested joins

4. **Equals predicate for field comparisons**
   ```go
   type Equals struct {
       Field string      // Field name (e.g., "cart_id")
       Value ir.IRValue  // Literal value (constrained to IRValue types)
   }
   func (Equals) predicateNode() {}
   ```
   - Field references column in query source
   - Value uses IRValue types (no floats, deterministic)
   - Represents `field = value` condition

5. **BoundEquals predicate for referencing bound variables**
   ```go
   type BoundEquals struct {
       Field    string // Field name in current query
       BoundVar string // Variable from when-clause bindings (e.g., "bound.cartId")
   }
   func (BoundEquals) predicateNode() {}
   ```
   - Enables where-clause to reference when-clause bindings
   - BoundVar follows "bound.varName" convention
   - Critical for correlating sync trigger with query

6. **And predicate for combining conditions**
   ```go
   type And struct {
       Predicates []Predicate // All must be true
   }
   func (And) predicateNode() {}
   ```
   - Combines multiple predicates with AND logic
   - Predicates slice can contain any Predicate type
   - Empty slice means always true (no conditions)

7. **Package documentation in `internal/queryir/doc.go`**
   - Explains QueryIR abstraction boundary (HIGH-2)
   - Documents portable fragment constraints
   - Lists excluded features (NULLs, outer joins, aggregations, SELECT *)

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **HIGH-2** | Query IR abstraction boundary between DSL and backends |
| **FR-2.5** | Maintain abstraction for future SPARQL migration |
| **Portable Fragment** | No NULLs, outer joins, aggregations, or SELECT * |

## Tasks / Subtasks

- [ ] Task 1: Create queryir package structure (AC: #1, #7)
  - [ ] 1.1 Create `internal/queryir/` directory
  - [ ] 1.2 Create `internal/queryir/doc.go` with package documentation
  - [ ] 1.3 Create `internal/queryir/types.go` with Query and Predicate interfaces

- [ ] Task 2: Implement Query interface and marker method (AC: #1)
  - [ ] 2.1 Define Query interface with queryNode() marker
  - [ ] 2.2 Add detailed interface documentation
  - [ ] 2.3 Document sealed interface pattern

- [ ] Task 3: Implement Predicate interface and marker method (AC: #1)
  - [ ] 3.1 Define Predicate interface with predicateNode() marker
  - [ ] 3.2 Add detailed interface documentation
  - [ ] 3.3 Document sealed interface pattern

- [ ] Task 4: Implement Select query type (AC: #2)
  - [ ] 4.1 Define Select struct with From, Filter, Bindings
  - [ ] 4.2 Implement queryNode() marker method
  - [ ] 4.3 Add detailed struct documentation with examples
  - [ ] 4.4 Document field semantics (table name, filter, bindings)

- [ ] Task 5: Implement Join query type (AC: #3)
  - [ ] 5.1 Define Join struct with Left, Right, On
  - [ ] 5.2 Implement queryNode() marker method
  - [ ] 5.3 Add detailed struct documentation
  - [ ] 5.4 Document inner join restriction (portable fragment)

- [ ] Task 6: Implement Equals predicate type (AC: #4)
  - [ ] 6.1 Define Equals struct with Field, Value
  - [ ] 6.2 Implement predicateNode() marker method
  - [ ] 6.3 Add detailed struct documentation
  - [ ] 6.4 Document IRValue constraint (no floats)

- [ ] Task 7: Implement BoundEquals predicate type (AC: #5)
  - [ ] 7.1 Define BoundEquals struct with Field, BoundVar
  - [ ] 7.2 Implement predicateNode() marker method
  - [ ] 7.3 Add detailed struct documentation
  - [ ] 7.4 Document "bound.varName" convention

- [ ] Task 8: Implement And predicate type (AC: #6)
  - [ ] 8.1 Define And struct with Predicates slice
  - [ ] 8.2 Implement predicateNode() marker method
  - [ ] 8.3 Add detailed struct documentation
  - [ ] 8.4 Document empty slice semantics (always true)

- [ ] Task 9: Write comprehensive tests (all AC)
  - [ ] 9.1 Test Select struct construction and marshaling
  - [ ] 9.2 Test Join struct with nested queries
  - [ ] 9.3 Test Equals predicate with all IRValue types
  - [ ] 9.4 Test BoundEquals predicate construction
  - [ ] 9.5 Test And predicate with multiple conditions
  - [ ] 9.6 Test sealed interfaces (only package types implement)
  - [ ] 9.7 Test JSON marshaling (for IR serialization)

## Dev Notes

### Query IR Abstraction Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    where DSL Parser                      │
│              (Future Story - CUE Compiler)               │
└───────────────────────┬─────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────┐
│                    Query IR Layer                        │
│        (This Story - Abstract Representation)            │
│  ┌─────────┐  ┌──────┐  ┌───────────┐  ┌──────────┐   │
│  │ Select  │  │ Join │  │ Predicate │  │ Bindings │   │
│  └─────────┘  └──────┘  └───────────┘  └──────────┘   │
└───────────────────────┬───────────────┬─────────────────┘
                        │               │
                        ▼               ▼
        ┌──────────────────┐    ┌──────────────────┐
        │   SQL Backend    │    │ SPARQL Backend   │
        │  (Story 4.3)     │    │   (Future)       │
        └──────────────────┘    └──────────────────┘
```

**Critical Design Principles:**

1. **Abstraction Boundary (HIGH-2)**
   - QueryIR is the contract between DSL and backends
   - DSL compiles to QueryIR (portable fragment)
   - Backends translate QueryIR to their query language
   - No backend-specific leakage in QueryIR types

2. **Portable Fragment Constraints**
   - **No NULLs** - Use explicit Option types or WHERE field IS NOT NULL
   - **No outer joins** - Inner joins only (LEFT/RIGHT/FULL not portable)
   - **No aggregations** - SUM/COUNT/GROUP BY not in portable fragment
   - **No SELECT *** - Explicit field bindings required

3. **Sealed Interface Pattern**
   - Query and Predicate are sealed (marker method prevents external impls)
   - Only types in internal/queryir can implement interfaces
   - Enables exhaustive type switches in backends

4. **SPARQL Migration Path**
   - QueryIR maps cleanly to SPARQL graph patterns:
     - Select → SPARQL SELECT with triple patterns
     - Join → SPARQL join (multiple triple patterns)
     - Equals → SPARQL `?var = "value"`
     - BoundEquals → SPARQL variable binding
   - Portable fragment ensures no SQL-specific assumptions

### Package Documentation

```go
// internal/queryir/doc.go
// Package queryir provides an abstract query intermediate representation (IR)
// for NYSM's where-clause query system.
//
// QueryIR is the abstraction boundary (HIGH-2) between the where-clause DSL
// and backend query engines (SQL, SPARQL). This enables SPARQL migration
// without rewriting sync rules.
//
// ARCHITECTURE:
//
// Query IR Layer:
// The QueryIR sits between the DSL compiler and backend query engines:
//
//   [where DSL] → [Query IR] → [SQL Backend]
//                            → [SPARQL Backend] (future)
//
// The Query IR defines a portable fragment of relational algebra that can
// be implemented by both SQL and SPARQL backends. Features outside the
// portable fragment are backend-specific and require explicit migration.
//
// PORTABLE FRAGMENT:
//
// The portable fragment includes:
// - Select(from, filter, bindings) - Table/source access with filtering
// - Join(left, right, on) - Inner joins only
// - Predicates: Equals, BoundEquals, And
// - Explicit field bindings (no SELECT *)
//
// The portable fragment EXCLUDES:
// - NULLs (use explicit Option types or IS NOT NULL filters)
// - Outer joins (LEFT/RIGHT/FULL not portable to SPARQL)
// - Aggregations (SUM/COUNT/GROUP BY not in MVP)
// - SELECT * (explicit bindings required)
// - Subqueries (not in MVP)
// - OR predicates (use separate rules or UNION)
//
// SEALED INTERFACES:
//
// Query and Predicate are sealed interfaces using the marker method pattern.
// Only types in this package can implement Query or Predicate interfaces.
//
// This enables:
// - Exhaustive type switches in backends
// - Compile-time safety against external extensions
// - Clear contract for backend implementers
//
// Example:
//   switch q := query.(type) {
//   case *Select:
//       // Handle select
//   case *Join:
//       // Handle join
//   default:
//       // Impossible - compiler knows all Query types
//   }
//
// SPARQL MIGRATION:
//
// The QueryIR portable fragment maps cleanly to SPARQL:
//
// QueryIR              SPARQL
// -------              ------
// Select               SELECT with triple patterns
// Join                 Multiple triple patterns (implicit join)
// Equals               ?var = "value"
// BoundEquals          Variable binding from outer scope
// And                  Multiple filters (implicit AND)
//
// Queries using portable fragment only are SPARQL-ready. Queries using
// SQL-specific features require explicit migration.
//
// CRITICAL PATTERNS:
//
// CP-5: IRValue Types Only
// All literal values in predicates use ir.IRValue types (no floats).
// This ensures deterministic query execution and canonical encoding.
//
// HIGH-2: Query Abstraction Boundary
// QueryIR is the contract. Backends implement this contract. DSL features
// outside the portable fragment are documented as backend-specific.
//
// FR-2.5: SPARQL Migration
// The portable fragment is designed for SPARQL compatibility. Future
// migration requires only backend implementation, not sync rule rewrites.
package queryir
```

### Type Definitions

```go
// internal/queryir/types.go
package queryir

import "github.com/tyler/nysm/internal/ir"

// Query represents an abstract query in the QueryIR.
//
// This is a sealed interface - only types in this package implement it.
// The marker method pattern prevents external implementations and enables
// exhaustive type switches in backend compilers.
//
// Query types:
// - Select: Basic table access with filtering and field bindings
// - Join: Combine two queries with inner join
//
// All queries produce a set of bindings (variable name → value mappings)
// that can be used in sync rule then-clauses.
type Query interface {
	queryNode() // Marker method - seals interface to this package
}

// Predicate represents a filter condition in the QueryIR.
//
// This is a sealed interface - only types in this package implement it.
// Predicates are used in Select.Filter and Join.On to filter rows.
//
// Predicate types:
// - Equals: field = literal_value
// - BoundEquals: field = bound_variable (from when-clause)
// - And: all predicates must be true
//
// The portable fragment excludes OR predicates and subqueries.
// Use separate sync rules or future UNION support for OR semantics.
type Predicate interface {
	predicateNode() // Marker method - seals interface to this package
}

// Select represents a basic table access query with filtering.
//
// Semantics:
//   SELECT <bindings> FROM <from> WHERE <filter>
//
// The Select query:
// 1. Accesses rows from a state table (From)
// 2. Filters rows using a predicate (Filter, optional)
// 3. Binds specific fields to variables (Bindings)
//
// Example (conceptual SQL translation):
//   Select{
//     From: "CartItems",
//     Filter: &And{Predicates: []Predicate{
//       &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//       &Equals{Field: "status", Value: ir.IRString("active")},
//     }},
//     Bindings: map[string]string{
//       "item_id": "itemId",
//       "quantity": "qty",
//     },
//   }
//
// Translates to SQL:
//   SELECT item_id, quantity FROM CartItems
//   WHERE cart_id = ? AND status = 'active'
//
// Produces bindings: {"itemId": <value>, "qty": <value>}
//
// PORTABLE FRAGMENT RULES:
// - From must reference a state table from concept spec
// - Filter must use portable predicates only (no SQL functions)
// - Bindings must be explicit (no SELECT *)
// - No NULLs in results (fields with NULL are excluded from bindings)
type Select struct {
	From     string            // Table/source name (e.g., "CartItems")
	Filter   Predicate         // WHERE conditions (nil = no filter)
	Bindings map[string]string // source_field → bound_variable
}

func (Select) queryNode() {}

// Join represents an inner join of two queries.
//
// Semantics:
//   SELECT * FROM (<left>) JOIN (<right>) ON <on>
//
// The Join query:
// 1. Executes left query to produce left bindings
// 2. Executes right query to produce right bindings
// 3. Combines binding sets where On predicate is true
// 4. Returns combined bindings (left ∪ right)
//
// Example (conceptual):
//   Join{
//     Left: &Select{From: "Carts", Bindings: map[string]string{"cart_id": "cartId"}},
//     Right: &Select{From: "CartItems", Bindings: map[string]string{"item_id": "itemId"}},
//     On: &Equals{Field: "cart_id", Value: /* reference to left.cartId */},
//   }
//
// PORTABLE FRAGMENT RULES:
// - Only INNER joins supported (no LEFT/RIGHT/FULL)
// - On predicate typically Equals or And of Equals (equi-join)
// - Left and Right can be Select or Join (recursive)
// - No cross joins (On predicate required)
//
// SPARQL MAPPING:
// Inner joins map to multiple SPARQL triple patterns:
//   Join(Select("Carts"), Select("CartItems"), cart_id = cart_id)
// becomes:
//   ?cart :cart_id ?cartId .
//   ?item :cart_id ?cartId .
type Join struct {
	Left  Query     // Left query (any Query type)
	Right Query     // Right query (any Query type)
	On    Predicate // Join condition (required for portable fragment)
}

func (Join) queryNode() {}

// Equals represents a field-equals-literal predicate.
//
// Semantics:
//   <field> = <value>
//
// The Equals predicate:
// 1. References a field in the current query source
// 2. Compares it to a literal value
// 3. Returns true if field value equals literal
//
// Example:
//   Equals{Field: "status", Value: ir.IRString("active")}
//
// Translates to SQL:
//   status = 'active'
//
// PORTABLE FRAGMENT RULES:
// - Value must be ir.IRValue (no floats per CP-5)
// - Comparison uses deterministic equality (no fuzzy matching)
// - NULLs never equal anything (use explicit IS NOT NULL filter)
//
// SPARQL MAPPING:
//   Equals{Field: "status", Value: "active"}
// becomes:
//   FILTER(?status = "active")
type Equals struct {
	Field string     // Field name in current query source
	Value ir.IRValue // Literal value (constrained to IRValue types)
}

func (Equals) predicateNode() {}

// BoundEquals represents a field-equals-bound-variable predicate.
//
// Semantics:
//   <field> = <bound_variable>
//
// The BoundEquals predicate:
// 1. References a field in the current query source
// 2. References a variable from when-clause bindings (outer scope)
// 3. Returns true if field value equals bound variable value
//
// Example (in sync rule context):
//   when: Cart.checkout.completed { cart_id: cartId }
//   where: Select{
//     From: "CartItems",
//     Filter: &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//   }
//
// The "bound.cartId" refers to the cartId variable bound in the when-clause.
//
// PORTABLE FRAGMENT RULES:
// - BoundVar must follow "bound.varName" convention
// - Variable must be defined in when-clause bindings
// - No nested bound variables (flat scope only)
//
// SPARQL MAPPING:
//   BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"}
// becomes:
//   ?item :cart_id ?cartId .  // Variable from outer scope
type BoundEquals struct {
	Field    string // Field name in current query source
	BoundVar string // Variable from when-clause (e.g., "bound.cartId")
}

func (BoundEquals) predicateNode() {}

// And represents a conjunction of predicates (all must be true).
//
// Semantics:
//   <predicate1> AND <predicate2> AND ... AND <predicateN>
//
// The And predicate:
// 1. Evaluates all predicates in Predicates slice
// 2. Returns true if ALL predicates are true
// 3. Returns false if ANY predicate is false
// 4. Returns true if Predicates is empty (vacuous truth)
//
// Example:
//   And{Predicates: []Predicate{
//     &Equals{Field: "status", Value: ir.IRString("active")},
//     &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//     &Equals{Field: "quantity", Value: ir.IRInt(1)},
//   }}
//
// Translates to SQL:
//   status = 'active' AND cart_id = ? AND quantity = 1
//
// PORTABLE FRAGMENT RULES:
// - Predicates can contain any Predicate type (including nested And)
// - Empty Predicates slice means "always true" (no conditions)
// - All predicates must be in portable fragment
// - No short-circuit evaluation guaranteed (backends may optimize)
//
// SPARQL MAPPING:
//   And{Predicates: [pred1, pred2, pred3]}
// becomes:
//   FILTER(pred1 && pred2 && pred3)
type And struct {
	Predicates []Predicate // All must be true (empty = always true)
}

func (And) predicateNode() {}
```

### Test Examples

```go
// internal/queryir/types_test.go
package queryir

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tyler/nysm/internal/ir"
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
	var queries []Query = []Query{
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
	var predicates []Predicate = []Predicate{
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
```

### File List

Files to create:

1. `internal/queryir/doc.go` - Package documentation
2. `internal/queryir/types.go` - Query and Predicate interfaces, all query types
3. `internal/queryir/types_test.go` - Comprehensive tests for all types

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for ir.IRValue types
- Epic 3 complete - Engine needs query layer for where-clause execution

**Enables:**
- Story 4.2 (QueryIR Validation) - Validates queries against portable fragment rules
- Story 4.3 (SQL Compilation) - Compiles QueryIR to parameterized SQL
- Story 4.4 (Where-Clause to QueryIR) - DSL parser compiles to QueryIR
- Story 5.1 (SPARQL Backend) - Future SPARQL implementation uses QueryIR contract

**Note:** This story defines ONLY the type system. No compilation, validation, or execution logic. Pure type definitions with sealed interfaces.

### Story Completion Checklist

- [ ] `internal/queryir/` directory created
- [ ] `internal/queryir/doc.go` written with comprehensive package documentation
- [ ] `internal/queryir/types.go` defines Query and Predicate sealed interfaces
- [ ] Select type implemented with From, Filter, Bindings
- [ ] Join type implemented with Left, Right, On
- [ ] Equals predicate implemented with Field, Value
- [ ] BoundEquals predicate implemented with Field, BoundVar
- [ ] And predicate implemented with Predicates slice
- [ ] All types implement appropriate marker methods (queryNode/predicateNode)
- [ ] All tests pass (`go test ./internal/queryir/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/queryir` passes
- [ ] Sealed interface tests verify only package types implement interfaces
- [ ] Complex query construction tests pass
- [ ] JSON marshaling tests pass (for IR serialization)

### References

- [Source: docs/architecture.md#HIGH-2] - Query IR abstraction boundary
- [Source: docs/architecture.md#Portable Fragment] - QueryIR constraints
- [Source: docs/epics.md#Story 4.1] - Story definition and acceptance criteria
- [Source: docs/prd.md#FR-2.5] - SPARQL migration requirement
- [Source: docs/architecture.md#Technology Stack] - Go 1.25
- [Source: docs/architecture.md#CP-5] - IRValue types only (no floats)

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: Following Epic 1, 2, 3 story format

### Completion Notes

- Foundation story for Epic 4 - all query layer stories depend on QueryIR types
- Sealed interface pattern ensures only package types implement Query/Predicate
- Portable fragment constraints documented in package docs
- No NULLs, outer joins, aggregations, or SELECT * in portable fragment
- BoundEquals enables correlation between when-clause and where-clause bindings
- And predicate supports multiple conditions (empty slice = always true)
- SPARQL migration path documented in package and type comments
- Query IR is pure type definitions - no compilation or execution logic
- Next stories (4.2-4.4) will add validation, compilation, and DSL parsing
- All types use ir.IRValue for literal values (deterministic, no floats per CP-5)
