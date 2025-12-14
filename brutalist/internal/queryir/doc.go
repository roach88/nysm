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
//	[where DSL] → [Query IR] → [SQL Backend]
//	                         → [SPARQL Backend] (future)
//
// The Query IR defines a portable fragment of relational algebra that can
// be implemented by both SQL and SPARQL backends. Features outside the
// portable fragment are backend-specific and require explicit migration.
//
// PORTABLE FRAGMENT:
//
// The portable fragment includes:
//   - Select(from, filter, bindings) - Table/source access with filtering
//   - Join(left, right, on) - Inner joins only
//   - Predicates: Equals, BoundEquals, And
//   - Explicit field bindings (no SELECT *)
//
// The portable fragment EXCLUDES:
//   - NULLs (use explicit Option types or IS NOT NULL filters)
//   - Outer joins (LEFT/RIGHT/FULL not portable to SPARQL)
//   - Aggregations (SUM/COUNT/GROUP BY not in MVP)
//   - SELECT * (explicit bindings required)
//   - Subqueries (not in MVP)
//   - OR predicates (use separate rules or UNION)
//
// SEALED INTERFACES:
//
// Query and Predicate are sealed interfaces using the marker method pattern.
// Only types in this package can implement Query or Predicate interfaces.
//
// This enables:
//   - Exhaustive type switches in backends
//   - Compile-time safety against external extensions
//   - Clear contract for backend implementers
//
// Example:
//
//	switch q := query.(type) {
//	case *Select:
//	    // Handle select
//	case *Join:
//	    // Handle join
//	default:
//	    // Impossible - compiler knows all Query types
//	}
//
// SPARQL MIGRATION:
//
// The QueryIR portable fragment maps cleanly to SPARQL:
//
//	QueryIR              SPARQL
//	-------              ------
//	Select               SELECT with triple patterns
//	Join                 Multiple triple patterns (implicit join)
//	Equals               ?var = "value"
//	BoundEquals          Variable binding from outer scope
//	And                  Multiple filters (implicit AND)
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
