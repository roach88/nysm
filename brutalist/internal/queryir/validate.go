package queryir

import (
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// ValidationResult contains portability analysis of a query.
//
// The portable fragment is the subset of QueryIR that can be implemented
// by both SQL and SPARQL backends. Queries outside this fragment will
// work with SQL but may require rewriting for SPARQL migration.
type ValidationResult struct {
	// IsPortable indicates if the query uses only portable fragment features.
	// True means the query will work with both SQL and future SPARQL backends.
	IsPortable bool

	// Warnings lists non-portable features used in the query.
	// Empty when IsPortable is true.
	Warnings []string
}

// Validate checks if a query conforms to the portable fragment rules.
//
// The portable fragment is the subset of QueryIR that can be implemented
// by both SQL and SPARQL backends. Queries outside this fragment will
// work with SQL but may require rewriting for SPARQL migration.
//
// Portable fragment rules:
//  1. No NULLs - all field comparisons must use explicit values
//  2. No outer joins - only inner joins allowed
//  3. Set semantics - no aggregations or duplicate handling
//  4. Explicit bindings - no SELECT * wildcards
//
// Non-portable queries are allowed and will execute correctly with the
// SQL backend. Warnings are returned to inform developers of migration
// constraints.
//
// Validate is a pure function with no side effects.
func Validate(query Query) ValidationResult {
	v := &validator{
		warnings: []string{},
	}
	v.validateQuery(query)

	return ValidationResult{
		IsPortable: len(v.warnings) == 0,
		Warnings:   v.warnings,
	}
}

// validator accumulates warnings during traversal.
type validator struct {
	warnings []string
}

// addWarning appends a warning message.
func (v *validator) addWarning(format string, args ...any) {
	v.warnings = append(v.warnings, fmt.Sprintf(format, args...))
}

// validateQuery recursively validates a query node.
func (v *validator) validateQuery(q Query) {
	if q == nil {
		v.addWarning("nil query - portable fragment requires valid query nodes")
		return
	}

	switch query := q.(type) {
	case Select:
		v.validateSelect(query)
	case *Select:
		v.validateSelect(*query)
	case Join:
		v.validateJoin(query)
	case *Join:
		v.validateJoin(*query)
	default:
		// Unknown query type - add warning
		v.addWarning("Unknown query type: %T - portability cannot be verified", q)
	}
}

// validateSelect validates a Select query node.
func (v *validator) validateSelect(sel Select) {
	// Rule 4: Explicit bindings - no SELECT *
	if len(sel.Bindings) == 0 {
		v.addWarning("Empty bindings (SELECT *) - portable fragment requires explicit field selection")
	}

	// Validate filter predicates
	if sel.Filter != nil {
		v.validatePredicate(sel.Filter)
	}
}

// validateJoin validates a Join query node.
func (v *validator) validateJoin(join Join) {
	// Rule 2: No outer joins
	// The current Join type only supports inner joins (no JoinType field).
	// If we add LEFT/RIGHT/FULL join support later, check it here:
	// if join.JoinType != JoinTypeInner {
	//     v.addWarning("Outer join detected - portable fragment requires inner joins only")
	// }

	// Recursively validate left and right sides
	v.validateQuery(join.Left)
	v.validateQuery(join.Right)

	// Validate join condition
	if join.On != nil {
		v.validatePredicate(join.On)
	}
}

// validatePredicate recursively validates a predicate node.
func (v *validator) validatePredicate(p Predicate) {
	if p == nil {
		return // nil predicates are valid (no filter)
	}

	switch pred := p.(type) {
	case Equals:
		v.validateEquals(pred)
	case *Equals:
		v.validateEquals(*pred)
	case BoundEquals:
		// BoundEquals is portable - references when-clause bindings
		// Binding existence is checked at runtime, not during validation
	case *BoundEquals:
		// Same as above
	case And:
		v.validateAnd(pred)
	case *And:
		v.validateAnd(*pred)
	default:
		// Unknown predicate type
		v.addWarning("Unknown predicate type: %T - portability cannot be verified", p)
	}
}

// validateEquals validates an Equals predicate.
func (v *validator) validateEquals(eq Equals) {
	// Rule 1: No NULLs
	// Check if the value is IRNull - not portable
	if _, isNull := eq.Value.(ir.IRNull); isNull {
		v.addWarning("Field '%s' compared to NULL - portable fragment requires explicit values", eq.Field)
	}
}

// validateAnd validates an And predicate.
func (v *validator) validateAnd(and And) {
	// Recursively validate all sub-predicates
	for _, subPred := range and.Predicates {
		v.validatePredicate(subPred)
	}
}
