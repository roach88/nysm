package harness

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// validIdentifier matches valid SQL identifiers (table/column names).
// Only allows alphanumeric and underscore, must start with letter or underscore.
// This prevents SQL injection via identifier interpolation.
var validIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// AssertionError is returned when an assertion fails.
// It includes detailed context to help debug the failure.
type AssertionError struct {
	Type     string       // Assertion type for categorization
	Expected string       // Human-readable expected outcome
	Actual   string       // Human-readable actual outcome
	Trace    []TraceEvent // Full trace for debugging context
}

// Error implements the error interface.
func (e *AssertionError) Error() string {
	var buf strings.Builder

	// Header with assertion type
	fmt.Fprintf(&buf, "Assertion failed: %s\n", e.Type)

	// Expected vs Actual (most important info)
	fmt.Fprintf(&buf, "  Expected: %s\n", e.Expected)
	fmt.Fprintf(&buf, "  Actual: %s\n", e.Actual)

	// Full trace for context
	fmt.Fprintf(&buf, "\nFull trace:\n")
	for i, event := range e.Trace {
		if event.Type == "invocation" {
			fmt.Fprintf(&buf, "  [%d] %s %v\n", i+1, event.ActionURI, event.Args)
		}
	}

	return buf.String()
}

// assertTraceContains checks if the trace contains an invocation matching
// the specified action and args (subset match).
func assertTraceContains(trace []TraceEvent, assertion Assertion) error {
	for _, event := range trace {
		if event.Type == "invocation" && event.ActionURI == assertion.Action {
			// Check args match (subset semantics)
			if matchArgs(event.Args, assertion.Args) {
				return nil // Found matching invocation
			}
		}
	}

	return &AssertionError{
		Type:     "trace_contains",
		Expected: fmt.Sprintf("action %s with args %v", assertion.Action, assertion.Args),
		Actual:   "not found in trace",
		Trace:    trace,
	}
}

// assertTraceOrder checks if actions appear in the specified order.
// Actions don't need to be consecutive (intervening actions are allowed).
func assertTraceOrder(trace []TraceEvent, assertion Assertion) error {
	// Step 1: Find first position of each expected action
	positions := make(map[string]int)

	for i, event := range trace {
		if event.Type == "invocation" {
			for _, expectedAction := range assertion.Actions {
				if event.ActionURI == expectedAction && positions[expectedAction] == 0 {
					positions[expectedAction] = i + 1 // 1-indexed for readability
				}
			}
		}
	}

	// Step 2: Verify all actions found
	for _, action := range assertion.Actions {
		if positions[action] == 0 {
			return &AssertionError{
				Type:     "trace_order",
				Expected: fmt.Sprintf("all actions present: %v", assertion.Actions),
				Actual:   fmt.Sprintf("missing action: %s", action),
				Trace:    trace,
			}
		}
	}

	// Step 3: Verify order
	for i := 1; i < len(assertion.Actions); i++ {
		prev := assertion.Actions[i-1]
		curr := assertion.Actions[i]

		if positions[prev] >= positions[curr] {
			return &AssertionError{
				Type:     "trace_order",
				Expected: fmt.Sprintf("actions in order: %v", assertion.Actions),
				Actual: fmt.Sprintf("%s (pos %d) should be before %s (pos %d)",
					prev, positions[prev], curr, positions[curr]),
				Trace: trace,
			}
		}
	}

	return nil
}

// assertTraceCount checks if the action appears exactly the specified number of times.
func assertTraceCount(trace []TraceEvent, assertion Assertion) error {
	count := 0

	// Count invocations matching action URI
	for _, event := range trace {
		if event.Type == "invocation" && event.ActionURI == assertion.Action {
			count++
		}
	}

	// Check exact count match
	if count != assertion.Count {
		return &AssertionError{
			Type:     "trace_count",
			Expected: fmt.Sprintf("%d occurrences of %s", assertion.Count, assertion.Action),
			Actual:   fmt.Sprintf("%d occurrences", count),
			Trace:    trace,
		}
	}

	return nil
}

// assertFinalState checks if the final state table contains expected values.
// Queries the state table with parameterized SQL (HIGH-3) and validates
// expected values using subset semantics.
//
// Security: Table and column names are validated against a whitelist pattern
// to prevent SQL injection via identifier interpolation.
func assertFinalState(ctx context.Context, st *store.Store, assertion Assertion) error {
	if assertion.Table == "" {
		return fmt.Errorf("final_state assertion requires table name")
	}

	// Validate table name to prevent SQL injection (identifiers can't be parameterized)
	if !validIdentifier.MatchString(assertion.Table) {
		return fmt.Errorf("invalid table name %q: must match pattern %s", assertion.Table, validIdentifier.String())
	}

	// Build WHERE clause with parameterized SQL (HIGH-3: never interpolate values)
	whereSQL, whereArgs, err := buildWhereClause(assertion.Where)
	if err != nil {
		return err // Identifier validation failed
	}

	// Build SELECT query (table name validated above)
	query := fmt.Sprintf("SELECT * FROM %s", assertion.Table)
	if whereSQL != "" {
		query += " WHERE " + whereSQL
	}

	// Execute query
	rows, err := st.Query(ctx, query, whereArgs...)
	if err != nil {
		return &AssertionError{
			Type:     "final_state",
			Expected: fmt.Sprintf("query table %s", assertion.Table),
			Actual:   fmt.Sprintf("query error: %v", err),
			Trace:    nil,
		}
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("get columns: %w", err)
	}

	// Scan the first row
	if !rows.Next() {
		// Row not found
		whereDesc := formatWhereClause(assertion.Where)
		return &AssertionError{
			Type:     "final_state",
			Expected: fmt.Sprintf("row in %s where %s", assertion.Table, whereDesc),
			Actual:   "row not found",
			Trace:    nil,
		}
	}

	// Prepare scan destinations
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return fmt.Errorf("scan row: %w", err)
	}

	// Check for multiple matching rows (would indicate ambiguous assertion)
	if rows.Next() {
		whereDesc := formatWhereClause(assertion.Where)
		return &AssertionError{
			Type:     "final_state",
			Expected: fmt.Sprintf("exactly one row in %s where %s", assertion.Table, whereDesc),
			Actual:   "multiple rows matched (assertion is ambiguous)",
			Trace:    nil,
		}
	}

	// Build map of column -> value
	actualRow := make(map[string]interface{})
	for i, col := range columns {
		actualRow[col] = values[i]
	}

	// Check each expected field (subset semantics - only check fields in Expect)
	for key, expectedValue := range assertion.Expect {
		actualValue, exists := actualRow[key]
		if !exists {
			return &AssertionError{
				Type:     "final_state",
				Expected: fmt.Sprintf("field %q to exist", key),
				Actual:   fmt.Sprintf("field %q not present in result columns: %v", key, columns),
				Trace:    nil,
			}
		}

		if !stateValuesEqual(expectedValue, actualValue) {
			return &AssertionError{
				Type:     "final_state",
				Expected: fmt.Sprintf("field %q = %v (type %T)", key, expectedValue, expectedValue),
				Actual:   fmt.Sprintf("field %q = %v (type %T)", key, actualValue, actualValue),
				Trace:    nil,
			}
		}
	}

	return nil
}

// buildWhereClause constructs parameterized WHERE clause from assertion.Where.
// Returns SQL fragment, arguments slice, and error. Keys are sorted for determinism.
//
// Security: Column names are validated against a whitelist pattern to prevent
// SQL injection via identifier interpolation.
func buildWhereClause(where map[string]interface{}) (string, []interface{}, error) {
	if len(where) == 0 {
		return "", nil, nil
	}

	// Sort keys for deterministic query generation
	keys := make([]string, 0, len(where))
	for k := range where {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	clauses := make([]string, 0, len(keys))
	args := make([]interface{}, 0, len(keys))

	for _, key := range keys {
		// Validate column name to prevent SQL injection
		if !validIdentifier.MatchString(key) {
			return "", nil, fmt.Errorf("invalid column name %q in where clause: must match pattern %s", key, validIdentifier.String())
		}
		clauses = append(clauses, fmt.Sprintf("%s = ?", key))
		args = append(args, toSQLValue(where[key]))
	}

	return strings.Join(clauses, " AND "), args, nil
}

// toSQLValue converts an interface{} value to a SQL-compatible value.
func toSQLValue(v interface{}) interface{} {
	switch val := v.(type) {
	case ir.IRString:
		return string(val)
	case ir.IRInt:
		return int64(val)
	case ir.IRBool:
		return bool(val)
	case string, int, int64, bool:
		return val
	default:
		// For other types, convert to string
		return fmt.Sprintf("%v", val)
	}
}

// formatWhereClause creates a human-readable description of WHERE conditions.
func formatWhereClause(where map[string]interface{}) string {
	if len(where) == 0 {
		return "(no conditions)"
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(where))
	for k := range where {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, where[k]))
	}
	return strings.Join(parts, " AND ")
}

// stateValuesEqual compares expected and actual values from state tables.
// Handles type coercion for SQLite values which may be returned as different types.
func stateValuesEqual(expected, actual interface{}) bool {
	// Handle nil cases
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}

	// Handle IR types
	switch exp := expected.(type) {
	case ir.IRString:
		if actualStr, ok := actual.(string); ok {
			return string(exp) == actualStr
		}
		return false
	case ir.IRInt:
		// SQLite returns int64 for integers
		if actualInt, ok := actual.(int64); ok {
			return int64(exp) == actualInt
		}
		// Also handle int
		if actualInt, ok := actual.(int); ok {
			return int64(exp) == int64(actualInt)
		}
		return false
	case ir.IRBool:
		// SQLite stores booleans as integers (0/1)
		if actualBool, ok := actual.(bool); ok {
			return bool(exp) == actualBool
		}
		if actualInt, ok := actual.(int64); ok {
			return bool(exp) == (actualInt != 0)
		}
		return false
	}

	// Handle primitive types in expected
	switch exp := expected.(type) {
	case string:
		if actualStr, ok := actual.(string); ok {
			return exp == actualStr
		}
		return false
	case int:
		if actualInt, ok := actual.(int64); ok {
			return int64(exp) == actualInt
		}
		if actualInt, ok := actual.(int); ok {
			return exp == actualInt
		}
		return false
	case int64:
		if actualInt, ok := actual.(int64); ok {
			return exp == actualInt
		}
		return false
	case bool:
		if actualBool, ok := actual.(bool); ok {
			return exp == actualBool
		}
		// SQLite stores booleans as integers
		if actualInt, ok := actual.(int64); ok {
			return exp == (actualInt != 0)
		}
		return false
	}

	// Fallback to DeepEqual for complex types
	return reflect.DeepEqual(expected, actual)
}

// matchArgs checks if actual args contain all expected args (subset match).
// Extra keys in actual are ignored.
func matchArgs(actual interface{}, expected map[string]interface{}) bool {
	if expected == nil || len(expected) == 0 {
		return true // No args to match
	}

	actualMap, ok := actual.(map[string]interface{})
	if !ok {
		return false
	}

	for key, expectedVal := range expected {
		actualVal, exists := actualMap[key]
		if !exists {
			return false // Required key missing
		}
		if !valuesEqual(actualVal, expectedVal) {
			return false // Value mismatch
		}
	}

	// Extra keys in actual are OK (subset match)
	return true
}

// valuesEqual compares two values for equality.
// Handles nested maps and slices.
func valuesEqual(actual, expected interface{}) bool {
	// Handle nil cases
	if actual == nil && expected == nil {
		return true
	}
	if actual == nil || expected == nil {
		return false
	}

	// Use reflect.DeepEqual for complex comparisons
	// This handles nested maps, slices, and IRValue types
	return reflect.DeepEqual(actual, expected)
}

// AssertionContext provides context for evaluating assertions.
type AssertionContext struct {
	Store *store.Store
	Ctx   context.Context
}

// EvaluateAssertions evaluates all assertions against the result.
// Returns a slice of error messages for failed assertions.
// The actx parameter provides database access for final_state assertions.
func EvaluateAssertions(result *Result, assertions []Assertion, actx *AssertionContext) []string {
	var errors []string

	for i, assertion := range assertions {
		var err error

		switch assertion.Type {
		case AssertTraceContains:
			err = assertTraceContains(result.Trace, assertion)
		case AssertTraceOrder:
			err = assertTraceOrder(result.Trace, assertion)
		case AssertTraceCount:
			err = assertTraceCount(result.Trace, assertion)
		case AssertFinalState:
			if actx == nil || actx.Store == nil {
				err = fmt.Errorf("assertion[%d]: final_state requires database context", i)
			} else {
				err = assertFinalState(actx.Ctx, actx.Store, assertion)
			}
		default:
			err = fmt.Errorf("assertion[%d]: unknown assertion type %q", i, assertion.Type)
		}

		if err != nil {
			errors = append(errors, err.Error())
		}
	}

	return errors
}
