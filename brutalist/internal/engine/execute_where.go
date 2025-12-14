package engine

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/queryir"
	"github.com/roach88/nysm/internal/querysql"
)

// executeWhere executes a where-clause query and returns binding sets.
//
// Returns zero or more bindings (empty slice is valid, not an error).
// Bindings are ordered deterministically per CP-4.
//
// Parameters:
//   - ctx: Context for query execution
//   - where: The where-clause from the sync rule
//   - whenBindings: Bindings extracted from the when-clause
//   - flowToken: Flow token for scoping (used in flow-scoped queries)
//
// Returns:
//   - []ir.IRObject: Zero or more binding sets, each containing merged when+where bindings
//   - error: Query compilation or execution errors
func (e *Engine) executeWhere(
	ctx context.Context,
	where *ir.WhereClause,
	whenBindings ir.IRObject,
	flowToken string,
) ([]ir.IRObject, error) {
	// If no where-clause, return single binding set (when-bindings only)
	if where == nil {
		return []ir.IRObject{whenBindings}, nil
	}

	// Build QueryIR query from where-clause
	query, err := e.buildQueryFromWhere(where, whenBindings)
	if err != nil {
		return nil, fmt.Errorf("build query: %w", err)
	}

	// Create SQL compiler with bound values
	compiler := querysql.NewSQLCompiler()
	for k, v := range whenBindings {
		param, err := irValueToSQLParam(v)
		if err != nil {
			return nil, fmt.Errorf("convert bound value %s: %w", k, err)
		}
		compiler.BoundValues["bound."+k] = param
	}

	// Compile QueryIR to SQL
	sqlStr, params, err := compiler.Compile(query)
	if err != nil {
		return nil, fmt.Errorf("compile query: %w", err)
	}

	// Execute query against store
	rows, err := e.store.Query(ctx, sqlStr, params...)
	if err != nil {
		return nil, fmt.Errorf("execute query: %w", err)
	}
	defer rows.Close()

	// Scan rows into binding sets
	var bindings []ir.IRObject
	for rows.Next() {
		binding, err := scanBinding(rows, where.Bindings)
		if err != nil {
			return nil, fmt.Errorf("scan binding: %w", err)
		}

		// Merge when-bindings with where-bindings
		mergedBinding := mergeBindings(whenBindings, binding)
		bindings = append(bindings, mergedBinding)
	}

	// Check for row iteration errors
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	// Empty slice is valid (zero matches)
	return bindings, nil
}

// buildQueryFromWhere constructs a QueryIR query from a where-clause and when-bindings.
//
// The WhereClause has:
//   - Source: table name (e.g., "CartItems")
//   - Filter: filter expression (e.g., "cart_id == bound.cart_id AND status == 'active'")
//   - Bindings: field → variable name mapping (e.g., {"item_id": "itemId"})
func (e *Engine) buildQueryFromWhere(
	where *ir.WhereClause,
	whenBindings ir.IRObject,
) (queryir.Query, error) {
	// Parse filter expression into predicate
	var filter queryir.Predicate
	if where.Filter != "" {
		parsed, err := parseFilterExpression(where.Filter)
		if err != nil {
			return nil, fmt.Errorf("parse filter: %w", err)
		}
		filter = parsed
	}

	// Build SELECT query
	query := queryir.Select{
		From:     where.Source,
		Filter:   filter,
		Bindings: where.Bindings,
	}

	return query, nil
}

// parseFilterExpression parses a filter expression string into a QueryIR Predicate.
//
// Supported expression formats:
//   - "field == value" → Equals{Field: "field", Value: value}
//   - "field == bound.var" → BoundEquals{Field: "field", BoundVar: "bound.var"}
//   - "expr1 AND expr2" → And{Predicates: [expr1, expr2]}
//
// This is a simplified parser for MVP. Full expression parsing would use
// a proper AST parser from the CUE compiler.
func parseFilterExpression(filter string) (queryir.Predicate, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, nil
	}

	// Check for AND expressions (split by " AND " or " and ")
	andParts := splitByAnd(filter)
	if len(andParts) > 1 {
		predicates := make([]queryir.Predicate, 0, len(andParts))
		for _, part := range andParts {
			pred, err := parseFilterExpression(part)
			if err != nil {
				return nil, err
			}
			if pred != nil {
				predicates = append(predicates, pred)
			}
		}
		if len(predicates) == 0 {
			return nil, nil
		}
		if len(predicates) == 1 {
			return predicates[0], nil
		}
		return queryir.And{Predicates: predicates}, nil
	}

	// Parse single comparison: "field == value" or "field == bound.var"
	return parseSingleComparison(filter)
}

// splitByAnd splits a filter expression by AND (case insensitive).
func splitByAnd(filter string) []string {
	// Simple split - look for " AND " or " and "
	var parts []string
	remaining := filter

	for {
		// Find " AND " (case insensitive)
		lowerRemaining := strings.ToLower(remaining)
		idx := strings.Index(lowerRemaining, " and ")
		if idx == -1 {
			parts = append(parts, strings.TrimSpace(remaining))
			break
		}

		parts = append(parts, strings.TrimSpace(remaining[:idx]))
		remaining = remaining[idx+5:] // len(" and ") == 5
	}

	return parts
}

// parseSingleComparison parses a single comparison expression.
// Supports: "field == value", "field == bound.var", "field == 'literal'"
func parseSingleComparison(expr string) (queryir.Predicate, error) {
	expr = strings.TrimSpace(expr)

	// Look for "==" operator
	idx := strings.Index(expr, "==")
	if idx == -1 {
		// Try single "=" as well
		idx = strings.Index(expr, "=")
		if idx == -1 {
			return nil, fmt.Errorf("unsupported expression (no == found): %s", expr)
		}
		// Make sure it's not part of !=
		if idx > 0 && expr[idx-1] == '!' {
			return nil, fmt.Errorf("unsupported operator != in: %s", expr)
		}
	}

	// Split into field and value parts
	var field, value string
	if strings.Index(expr, "==") != -1 {
		parts := strings.SplitN(expr, "==", 2)
		field = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	} else {
		parts := strings.SplitN(expr, "=", 2)
		field = strings.TrimSpace(parts[0])
		value = strings.TrimSpace(parts[1])
	}

	// Check if value is a bound variable reference
	if strings.HasPrefix(value, "bound.") {
		return queryir.BoundEquals{
			Field:    field,
			BoundVar: value,
		}, nil
	}

	// Check if value is a string literal (quoted)
	if (strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) ||
		(strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) {
		// Remove quotes
		literal := value[1 : len(value)-1]
		return queryir.Equals{
			Field: field,
			Value: ir.IRString(literal),
		}, nil
	}

	// Check if value is a number
	if isNumeric(value) {
		intVal := parseInt(value)
		return queryir.Equals{
			Field: field,
			Value: ir.IRInt(intVal),
		}, nil
	}

	// Check if value is a boolean
	if value == "true" {
		return queryir.Equals{
			Field: field,
			Value: ir.IRBool(true),
		}, nil
	}
	if value == "false" {
		return queryir.Equals{
			Field: field,
			Value: ir.IRBool(false),
		}, nil
	}

	// Assume unquoted string literal
	return queryir.Equals{
		Field: field,
		Value: ir.IRString(value),
	}, nil
}

// isNumeric checks if a string is a valid integer.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
	}
	if start >= len(s) {
		return false
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseInt parses a string to int64.
func parseInt(s string) int64 {
	var result int64
	negative := false
	start := 0

	if s[0] == '-' {
		negative = true
		start = 1
	} else if s[0] == '+' {
		start = 1
	}

	for i := start; i < len(s); i++ {
		result = result*10 + int64(s[i]-'0')
	}

	if negative {
		return -result
	}
	return result
}

// scanBinding scans a SQL row into an ir.IRObject.
// Maps SQL columns to binding variable names per bindingSpec.
func scanBinding(
	rows *sql.Rows,
	bindingSpec map[string]string,
) (ir.IRObject, error) {
	// Get column names from result set
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	// Create scan targets (interface{} for each column)
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Scan row into values
	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("scan row: %w", err)
	}

	// Build binding object
	binding := make(ir.IRObject)
	for i, colName := range columns {
		// Look up variable name for this column
		varName, exists := bindingSpec[colName]
		if !exists {
			// Column not in binding spec - skip
			continue
		}

		// Convert SQL value to IRValue
		irValue, err := sqlToIRValue(values[i])
		if err != nil {
			return nil, fmt.Errorf("convert column %s: %w", colName, err)
		}

		binding[varName] = irValue
	}

	return binding, nil
}

// mergeBindings is defined in scope.go with better nil handling.
// It combines when-bindings and where-bindings, with where-bindings
// taking precedence if there are conflicts.

// sqlToIRValue converts a SQL value (from database/sql) to an ir.IRValue.
func sqlToIRValue(v interface{}) (ir.IRValue, error) {
	if v == nil {
		return ir.IRNull{}, nil
	}

	switch val := v.(type) {
	case int64:
		return ir.IRInt(val), nil
	case int:
		return ir.IRInt(int64(val)), nil
	case float64:
		// CP-5: Floats are FORBIDDEN in IR - they break determinism.
		// SQL REAL/FLOAT columns must be avoided in schema design.
		// If you hit this, either:
		// 1. Change the column type to INTEGER (store cents not dollars)
		// 2. Store as TEXT with explicit precision
		return nil, fmt.Errorf("float64 values are forbidden in IR (CP-5): %v - use INTEGER or TEXT instead", val)
	case string:
		return ir.IRString(val), nil
	case []byte:
		return ir.IRString(string(val)), nil
	case bool:
		return ir.IRBool(val), nil
	default:
		return nil, fmt.Errorf("unsupported SQL type: %T", v)
	}
}

// irValueToSQLParam converts an ir.IRValue to a Go native type for SQL parameter.
func irValueToSQLParam(v ir.IRValue) (any, error) {
	switch val := v.(type) {
	case ir.IRString:
		return string(val), nil
	case ir.IRInt:
		return int64(val), nil
	case ir.IRBool:
		return bool(val), nil
	case ir.IRNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported IRValue type for SQL parameter: %T", v)
	}
}
