package querysql

import (
	"fmt"
	"sort"
	"strings"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/queryir"
)

// SQLCompiler compiles QueryIR to parameterized SQL for SQLite.
//
// CRITICAL: ALL queries include ORDER BY per CP-4 for deterministic results.
// CRITICAL: All values are parameterized (never interpolated) per HIGH-3.
type SQLCompiler struct {
	// BoundValues holds the values for BoundEquals predicates.
	// Must be set by the engine before compilation.
	BoundValues map[string]any
}

// NewSQLCompiler creates a new SQLCompiler.
func NewSQLCompiler() *SQLCompiler {
	return &SQLCompiler{
		BoundValues: make(map[string]any),
	}
}

// Compile converts a QueryIR query to parameterized SQL.
// Returns (sql, params, error) tuple.
//
// MANDATORY: Every query includes ORDER BY with deterministic tiebreaker per CP-4.
// MANDATORY: All values are parameterized (never interpolated) per HIGH-3.
func (c *SQLCompiler) Compile(q queryir.Query) (string, []any, error) {
	if q == nil {
		return "", nil, fmt.Errorf("cannot compile nil query")
	}

	switch query := q.(type) {
	case queryir.Select:
		return c.compileSelect(query)
	case *queryir.Select:
		return c.compileSelect(*query)
	case queryir.Join:
		return c.compileJoin(query)
	case *queryir.Join:
		return c.compileJoin(*query)
	default:
		return "", nil, fmt.Errorf("unsupported query type: %T", q)
	}
}

// compileSelect compiles a queryir.Select to SQL.
// MANDATORY: Includes ORDER BY per CP-4.
func (c *SQLCompiler) compileSelect(q queryir.Select) (string, []any, error) {
	// Build SELECT clause from bindings
	selectClause := c.compileBindings(q.Bindings)

	// Build FROM clause
	fromClause := q.From

	// Build WHERE clause and collect parameters
	var whereClause string
	var params []any
	if q.Filter != nil {
		filterSQL, filterParams, err := c.compilePredicate(q.Filter)
		if err != nil {
			return "", nil, fmt.Errorf("compile filter: %w", err)
		}
		whereClause = " WHERE " + filterSQL
		params = filterParams
	}

	// MANDATORY: Always add ORDER BY per CP-4
	orderByClause := " ORDER BY " + c.stableOrderKey(q)

	// Assemble SQL
	sql := fmt.Sprintf("SELECT %s FROM %s%s%s",
		selectClause,
		fromClause,
		whereClause,
		orderByClause)

	return sql, params, nil
}

// compileBindings converts bindings map to SELECT column list.
// Example: {"item_id": "itemId"} → "item_id AS itemId"
// Keys are sorted for deterministic output.
func (c *SQLCompiler) compileBindings(bindings map[string]string) string {
	if len(bindings) == 0 {
		return "*"
	}

	// Sort keys for deterministic output (testing)
	keys := make([]string, 0, len(bindings))
	for k := range bindings {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, sourceField := range keys {
		boundVar := bindings[sourceField]
		if sourceField == boundVar {
			// No alias needed
			parts = append(parts, sourceField)
		} else {
			// Alias: source_field AS bound_var
			parts = append(parts, fmt.Sprintf("%s AS %s", sourceField, boundVar))
		}
	}

	return strings.Join(parts, ", ")
}

// stableOrderKey returns the ORDER BY clause for a query.
// MANDATORY: Every query MUST call this function per CP-4.
// Uses COLLATE BINARY for deterministic text ordering.
func (c *SQLCompiler) stableOrderKey(q queryir.Select) string {
	// Default to id as primary key
	// COLLATE BINARY ensures deterministic text ordering across SQLite versions
	return "id ASC COLLATE BINARY"
}

// compilePredicate compiles a queryir.Predicate to SQL WHERE clause fragment.
// Returns (sql, params, error).
// CRITICAL: Values NEVER interpolated - always use ? placeholders.
func (c *SQLCompiler) compilePredicate(p queryir.Predicate) (string, []any, error) {
	if p == nil {
		return "1 = 1", nil, nil // Always true
	}

	switch pred := p.(type) {
	case queryir.Equals:
		return c.compileEquals(pred)
	case *queryir.Equals:
		return c.compileEquals(*pred)
	case queryir.And:
		return c.compileAnd(pred)
	case *queryir.And:
		return c.compileAnd(*pred)
	case queryir.BoundEquals:
		return c.compileBoundEquals(pred)
	case *queryir.BoundEquals:
		return c.compileBoundEquals(*pred)
	default:
		return "", nil, fmt.Errorf("unsupported predicate type: %T", p)
	}
}

// compileEquals compiles an Equals predicate to "field = ?".
// CRITICAL: Value is NEVER interpolated - always parameterized.
func (c *SQLCompiler) compileEquals(eq queryir.Equals) (string, []any, error) {
	// Convert IRValue to Go native type for SQL parameter
	param, err := irValueToParam(eq.Value)
	if err != nil {
		return "", nil, fmt.Errorf("convert value: %w", err)
	}

	sql := fmt.Sprintf("%s = ?", eq.Field)
	params := []any{param}

	return sql, params, nil
}

// compileAnd compiles an And predicate to conjunction with AND.
func (c *SQLCompiler) compileAnd(and queryir.And) (string, []any, error) {
	if len(and.Predicates) == 0 {
		return "1 = 1", nil, nil // Always true (vacuous truth)
	}

	var sqlParts []string
	var allParams []any

	for _, pred := range and.Predicates {
		sql, params, err := c.compilePredicate(pred)
		if err != nil {
			return "", nil, err
		}
		sqlParts = append(sqlParts, sql)
		allParams = append(allParams, params...)
	}

	// Join with AND
	sql := strings.Join(sqlParts, " AND ")

	return sql, allParams, nil
}

// compileBoundEquals compiles a BoundEquals predicate.
// BoundEquals references a variable from when-clause bindings.
// The bound value is looked up from BoundValues map.
// CRITICAL: Value is NEVER interpolated - always parameterized.
func (c *SQLCompiler) compileBoundEquals(beq queryir.BoundEquals) (string, []any, error) {
	sql := fmt.Sprintf("%s = ?", beq.Field)

	// Look up bound value from BoundValues map
	var params []any
	if c.BoundValues != nil {
		if val, ok := c.BoundValues[beq.BoundVar]; ok {
			params = []any{val}
		}
	}
	// If no bound value found, params remains empty
	// The engine will need to provide the value at execution time

	return sql, params, nil
}

// compileJoin compiles a queryir.Join to SQL INNER JOIN.
// MANDATORY: Includes ORDER BY per CP-4.
func (c *SQLCompiler) compileJoin(j queryir.Join) (string, []any, error) {
	// Get left table (must be Select for MVP)
	leftTable, leftOK := getSelectFrom(j.Left)
	if !leftOK {
		return "", nil, fmt.Errorf("join left must be Select for MVP")
	}

	// Get right table (must be Select for MVP)
	rightTable, rightOK := getSelectFrom(j.Right)
	if !rightOK {
		return "", nil, fmt.Errorf("join right must be Select for MVP")
	}

	// Collect all parameters from left and right filters
	var allParams []any

	// Compile left filter if present
	leftSelect := getSelect(j.Left)
	if leftSelect != nil && leftSelect.Filter != nil {
		_, leftParams, err := c.compilePredicate(leftSelect.Filter)
		if err != nil {
			return "", nil, fmt.Errorf("compile left filter: %w", err)
		}
		allParams = append(allParams, leftParams...)
	}

	// Compile right filter if present
	rightSelect := getSelect(j.Right)
	if rightSelect != nil && rightSelect.Filter != nil {
		_, rightParams, err := c.compilePredicate(rightSelect.Filter)
		if err != nil {
			return "", nil, fmt.Errorf("compile right filter: %w", err)
		}
		allParams = append(allParams, rightParams...)
	}

	// Compile ON predicate
	var onSQL string
	if j.On != nil {
		sql, onParams, err := c.compilePredicate(j.On)
		if err != nil {
			return "", nil, fmt.Errorf("compile join ON: %w", err)
		}
		onSQL = sql
		allParams = append(allParams, onParams...)
	} else {
		onSQL = "1 = 1" // Cross join (no condition)
	}

	// Build JOIN SQL
	sql := fmt.Sprintf("%s INNER JOIN %s ON %s",
		leftTable,
		rightTable,
		onSQL)

	// MANDATORY: Add ORDER BY per CP-4
	// For joins, order by first table's primary key
	sql += " ORDER BY " + leftTable + ".id ASC COLLATE BINARY"

	return sql, allParams, nil
}

// getSelectFrom extracts the table name from a Query if it's a Select.
func getSelectFrom(q queryir.Query) (string, bool) {
	switch query := q.(type) {
	case queryir.Select:
		return query.From, true
	case *queryir.Select:
		return query.From, true
	default:
		return "", false
	}
}

// getSelect extracts the Select from a Query if it's a Select.
func getSelect(q queryir.Query) *queryir.Select {
	switch query := q.(type) {
	case queryir.Select:
		return &query
	case *queryir.Select:
		return query
	default:
		return nil
	}
}

// irValueToParam converts an ir.IRValue to a Go native type for SQL parameter.
// Supports string, int, bool. Arrays and objects are not directly supported
// as SQL parameters.
func irValueToParam(v ir.IRValue) (any, error) {
	switch val := v.(type) {
	case ir.IRString:
		return string(val), nil
	case ir.IRInt:
		return int64(val), nil
	case ir.IRBool:
		return bool(val), nil
	case ir.IRNull:
		return nil, nil
	case ir.IRArray:
		return nil, fmt.Errorf("IRArray cannot be used as SQL parameter directly")
	case ir.IRObject:
		return nil, fmt.Errorf("IRObject cannot be used as SQL parameter directly")
	default:
		return nil, fmt.Errorf("unsupported IRValue type for SQL parameter: %T", v)
	}
}
