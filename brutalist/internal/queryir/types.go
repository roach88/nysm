package queryir

import "github.com/roach88/nysm/internal/ir"

// Query represents an abstract query in the QueryIR.
//
// This is a sealed interface - only types in this package implement it.
// The marker method pattern prevents external implementations and enables
// exhaustive type switches in backend compilers.
//
// Query types:
//   - Select: Basic table access with filtering and field bindings
//   - Join: Combine two queries with inner join
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
//   - Equals: field = literal_value
//   - BoundEquals: field = bound_variable (from when-clause)
//   - And: all predicates must be true
//
// The portable fragment excludes OR predicates and subqueries.
// Use separate sync rules or future UNION support for OR semantics.
type Predicate interface {
	predicateNode() // Marker method - seals interface to this package
}

// Select represents a basic table access query with filtering.
//
// Semantics:
//
//	SELECT <bindings> FROM <from> WHERE <filter>
//
// The Select query:
//  1. Accesses rows from a state table (From)
//  2. Filters rows using a predicate (Filter, optional)
//  3. Binds specific fields to variables (Bindings)
//
// Example (conceptual SQL translation):
//
//	Select{
//	  From: "CartItems",
//	  Filter: &And{Predicates: []Predicate{
//	    &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//	    &Equals{Field: "status", Value: ir.IRString("active")},
//	  }},
//	  Bindings: map[string]string{
//	    "item_id": "itemId",
//	    "quantity": "qty",
//	  },
//	}
//
// Translates to SQL:
//
//	SELECT item_id, quantity FROM CartItems
//	WHERE cart_id = ? AND status = 'active'
//
// Produces bindings: {"itemId": <value>, "qty": <value>}
//
// PORTABLE FRAGMENT RULES:
//   - From must reference a state table from concept spec
//   - Filter must use portable predicates only (no SQL functions)
//   - Bindings must be explicit (no SELECT *)
//   - No NULLs in results (fields with NULL are excluded from bindings)
type Select struct {
	From     string            // Table/source name (e.g., "CartItems")
	Filter   Predicate         // WHERE conditions (nil = no filter)
	Bindings map[string]string // source_field → bound_variable
}

func (Select) queryNode() {}

// Join represents an inner join of two queries.
//
// Semantics:
//
//	SELECT * FROM (<left>) JOIN (<right>) ON <on>
//
// The Join query:
//  1. Executes left query to produce left bindings
//  2. Executes right query to produce right bindings
//  3. Combines binding sets where On predicate is true
//  4. Returns combined bindings (left ∪ right)
//
// Example (conceptual):
//
//	Join{
//	  Left: &Select{From: "Carts", Bindings: map[string]string{"cart_id": "cartId"}},
//	  Right: &Select{From: "CartItems", Bindings: map[string]string{"item_id": "itemId"}},
//	  On: &Equals{Field: "cart_id", Value: /* reference to left.cartId */},
//	}
//
// PORTABLE FRAGMENT RULES:
//   - Only INNER joins supported (no LEFT/RIGHT/FULL)
//   - On predicate typically Equals or And of Equals (equi-join)
//   - Left and Right can be Select or Join (recursive)
//   - No cross joins (On predicate required)
//
// SPARQL MAPPING:
// Inner joins map to multiple SPARQL triple patterns:
//
//	Join(Select("Carts"), Select("CartItems"), cart_id = cart_id)
//
// becomes:
//
//	?cart :cart_id ?cartId .
//	?item :cart_id ?cartId .
type Join struct {
	Left  Query     // Left query (any Query type)
	Right Query     // Right query (any Query type)
	On    Predicate // Join condition (required for portable fragment)
}

func (Join) queryNode() {}

// Equals represents a field-equals-literal predicate.
//
// Semantics:
//
//	<field> = <value>
//
// The Equals predicate:
//  1. References a field in the current query source
//  2. Compares it to a literal value
//  3. Returns true if field value equals literal
//
// Example:
//
//	Equals{Field: "status", Value: ir.IRString("active")}
//
// Translates to SQL:
//
//	status = 'active'
//
// PORTABLE FRAGMENT RULES:
//   - Value must be ir.IRValue (no floats per CP-5)
//   - Comparison uses deterministic equality (no fuzzy matching)
//   - NULLs never equal anything (use explicit IS NOT NULL filter)
//
// SPARQL MAPPING:
//
//	Equals{Field: "status", Value: "active"}
//
// becomes:
//
//	FILTER(?status = "active")
type Equals struct {
	Field string     // Field name in current query source
	Value ir.IRValue // Literal value (constrained to IRValue types)
}

func (Equals) predicateNode() {}

// BoundEquals represents a field-equals-bound-variable predicate.
//
// Semantics:
//
//	<field> = <bound_variable>
//
// The BoundEquals predicate:
//  1. References a field in the current query source
//  2. References a variable from when-clause bindings (outer scope)
//  3. Returns true if field value equals bound variable value
//
// Example (in sync rule context):
//
//	when: Cart.checkout.completed { cart_id: cartId }
//	where: Select{
//	  From: "CartItems",
//	  Filter: &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//	}
//
// The "bound.cartId" refers to the cartId variable bound in the when-clause.
//
// PORTABLE FRAGMENT RULES:
//   - BoundVar must follow "bound.varName" convention
//   - Variable must be defined in when-clause bindings
//   - No nested bound variables (flat scope only)
//
// SPARQL MAPPING:
//
//	BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"}
//
// becomes:
//
//	?item :cart_id ?cartId .  // Variable from outer scope
type BoundEquals struct {
	Field    string // Field name in current query source
	BoundVar string // Variable from when-clause (e.g., "bound.cartId")
}

func (BoundEquals) predicateNode() {}

// And represents a conjunction of predicates (all must be true).
//
// Semantics:
//
//	<predicate1> AND <predicate2> AND ... AND <predicateN>
//
// The And predicate:
//  1. Evaluates all predicates in Predicates slice
//  2. Returns true if ALL predicates are true
//  3. Returns false if ANY predicate is false
//  4. Returns true if Predicates is empty (vacuous truth)
//
// Example:
//
//	And{Predicates: []Predicate{
//	  &Equals{Field: "status", Value: ir.IRString("active")},
//	  &BoundEquals{Field: "cart_id", BoundVar: "bound.cartId"},
//	  &Equals{Field: "quantity", Value: ir.IRInt(1)},
//	}}
//
// Translates to SQL:
//
//	status = 'active' AND cart_id = ? AND quantity = 1
//
// PORTABLE FRAGMENT RULES:
//   - Predicates can contain any Predicate type (including nested And)
//   - Empty Predicates slice means "always true" (no conditions)
//   - All predicates must be in portable fragment
//   - No short-circuit evaluation guaranteed (backends may optimize)
//
// SPARQL MAPPING:
//
//	And{Predicates: [pred1, pred2, pred3]}
//
// becomes:
//
//	FILTER(pred1 && pred2 && pred3)
type And struct {
	Predicates []Predicate // All must be true (empty = always true)
}

func (And) predicateNode() {}
