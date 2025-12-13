# Story 6.4: Final State Assertions

**Epic:** 6 - Conformance Harness
**Status:** ready-for-dev
**Created:** 2025-12-12
**Story Points:** 5

---

## Story Statement

As a **developer verifying outcomes**,
I want **to assert on final state table contents**,
So that **I can verify projections are correct**.

---

## Acceptance Criteria

### AC1: FinalState Assertion Type

**Given** the harness assertion system
**When** defining final state assertions
**Then** the `FinalState` type supports:

```go
// internal/harness/assertions.go
type FinalState struct {
    Table  string                    `json:"table"`   // State table name
    Where  map[string]ir.IRValue     `json:"where"`   // Row identification
    Expect map[string]ir.IRValue     `json:"expect"`  // Expected values (subset)
}

func (FinalState) assertionType() {} // Implements Assertion interface
```

**And** the assertion is loaded from scenario YAML:
```yaml
assertions:
  - type: final_state
    table: cart_items
    where:
      item_id: "widget"
    expect:
      quantity: 3
```

### AC2: Query Row with Parameterized SQL

**Given** a `FinalState` assertion
**When** evaluating the assertion
**Then** the specified row is queried using parameterized SQL per HIGH-3:

```go
func (h *Harness) assertFinalState(ctx context.Context, assertion FinalState) error {
    // Build WHERE clause with parameterization
    whereClauses := []string{}
    whereArgs := []any{}
    for key, value := range assertion.Where {
        whereClauses = append(whereClauses, fmt.Sprintf("%s = ?", key))
        whereArgs = append(whereArgs, h.toSQLValue(value))
    }
    whereSQL := strings.Join(whereClauses, " AND ")

    // Query with parameterized WHERE
    query := fmt.Sprintf("SELECT * FROM %s WHERE %s", assertion.Table, whereSQL)
    row := h.store.QueryRow(ctx, query, whereArgs...)

    actual, err := h.scanRow(row, assertion.Table)
    if err != nil {
        return fmt.Errorf("failed to query row: %w", err)
    }

    // ... compare expected values
}
```

**And** NO string interpolation of values
**And** all WHERE parameters use `?` placeholders

### AC3: Subset Value Comparison

**Given** actual row data from the database
**When** comparing against expected values
**Then** only the fields in `Expect` are checked (subset match):

```go
func (h *Harness) assertFinalState(ctx context.Context, assertion FinalState) error {
    // ... query row ...

    // Compare expected values (subset check)
    for key, expected := range assertion.Expect {
        actual, ok := actualRow[key]
        if !ok {
            return fmt.Errorf("expected field %q not present in result", key)
        }

        if !h.valuesEqual(expected, actual) {
            return fmt.Errorf("field %q: expected %v, got %v", key, expected, actual)
        }
    }

    return nil
}

func (h *Harness) valuesEqual(expected ir.IRValue, actual any) bool {
    switch exp := expected.(type) {
    case ir.IRString:
        actualStr, ok := actual.(string)
        return ok && string(exp) == actualStr
    case ir.IRInt:
        actualInt, ok := actual.(int64)
        return ok && int64(exp) == actualInt
    case ir.IRBool:
        actualBool, ok := actual.(bool)
        return ok && bool(exp) == actualBool
    default:
        return reflect.DeepEqual(expected, actual)
    }
}
```

**And** extra columns in actual row are ignored
**And** only columns in `Expect` affect pass/fail

### AC4: Missing Row Error Handling

**Given** a `FinalState` assertion with WHERE clause
**When** the row is not found in the table
**Then** a clear error is returned:

```go
func (h *Harness) scanRow(row *sql.Row, tableName string) (map[string]any, error) {
    // Get column names for this table
    columns, err := h.getTableColumns(tableName)
    if err != nil {
        return nil, fmt.Errorf("get table columns: %w", err)
    }

    // Prepare scan destinations
    values := make([]any, len(columns))
    valuePtrs := make([]any, len(columns))
    for i := range columns {
        valuePtrs[i] = &values[i]
    }

    // Scan row
    err = row.Scan(valuePtrs...)
    if err == sql.ErrNoRows {
        return nil, fmt.Errorf("row not found in table %s", tableName)
    }
    if err != nil {
        return nil, fmt.Errorf("scan row: %w", err)
    }

    // Build map
    result := make(map[string]any)
    for i, col := range columns {
        result[col] = values[i]
    }
    return result, nil
}
```

**And** the error message includes the table name
**And** distinguishes "row not found" from other scan errors

### AC5: Type Mismatch Detection

**Given** an expected value with a specific type
**When** the actual value has a different type
**Then** a clear type mismatch error is returned:

```go
func (h *Harness) valuesEqual(expected ir.IRValue, actual any) bool {
    switch exp := expected.(type) {
    case ir.IRString:
        actualStr, ok := actual.(string)
        if !ok {
            return false // Type mismatch detected
        }
        return string(exp) == actualStr
    case ir.IRInt:
        actualInt, ok := actual.(int64)
        if !ok {
            return false // Type mismatch detected
        }
        return int64(exp) == actualInt
    // ... other cases
    }
}

// Error message when valuesEqual returns false
if !h.valuesEqual(expected, actual) {
    return fmt.Errorf("field %q: expected %v (type %T), got %v (type %T)",
        key, expected, expected, actual, actual)
}
```

**And** error shows both expected and actual types
**And** makes type mismatches easy to diagnose

---

## Quick Reference

### Relevant Patterns

| Pattern | Location | Usage |
|---------|----------|-------|
| **HIGH-3: Parameterized Queries** | Architecture | All SQL queries use `?` placeholders, never string interpolation |
| **Constrained IRValue Types (CP-5)** | Architecture | Expected values use ir.IRString, ir.IRInt, ir.IRBool - no floats |
| **Subset Checking** | Story AC3 | Only fields in `Expect` are validated; extra actual columns ignored |

---

## Tasks/Subtasks

### Task 1: Define FinalState Assertion Type
- [ ] Create `FinalState` struct in `internal/harness/assertions.go`
- [ ] Add `Table`, `Where`, `Expect` fields with correct types
- [ ] Implement `assertionType()` marker method
- [ ] Add JSON struct tags for YAML unmarshaling
- [ ] Update `Assertion` interface to include `FinalState`

### Task 2: Implement Row Querying with Parameterization
- [ ] Implement `buildWhereClause()` to generate parameterized SQL
- [ ] Implement `whereArgs()` to extract argument values
- [ ] Implement `toSQLValue()` to convert ir.IRValue to SQL types
- [ ] Add `getTableColumns()` to retrieve column metadata
- [ ] Implement `scanRow()` to map SQL result to Go map
- [ ] Handle `sql.ErrNoRows` with clear error message

### Task 3: Implement Subset Value Comparison
- [ ] Implement `assertFinalState()` main assertion logic
- [ ] Implement `valuesEqual()` type-aware comparison
- [ ] Support ir.IRString, ir.IRInt, ir.IRBool comparisons
- [ ] Implement fallback to `reflect.DeepEqual` for complex types
- [ ] Verify extra columns are ignored (subset check)

### Task 4: Add Error Handling and Reporting
- [ ] Clear error for missing row
- [ ] Clear error for type mismatch with both types shown
- [ ] Clear error for missing expected field in result
- [ ] Format error messages with field name, expected, actual
- [ ] Test error messages are actionable

### Task 5: Integration with Harness
- [ ] Register `FinalState` in assertion factory/loader
- [ ] Add YAML unmarshaling support for `final_state` type
- [ ] Update scenario loading to handle `final_state` assertions
- [ ] Integrate with `harness.Run()` assertion evaluation loop
- [ ] Test with full scenario execution

### Task 6: Unit Tests
- [ ] Test row found with matching values → pass
- [ ] Test row found with mismatched values → fail with clear message
- [ ] Test row not found → fail with "row not found" message
- [ ] Test extra columns in actual result → ignored (pass)
- [ ] Test type mismatch → fail with type information
- [ ] Test missing expected field in result → fail
- [ ] Test empty `Expect` map → pass (no assertions)
- [ ] Test parameterized query generation

---

## Dev Notes

### Implementation Details

#### FinalState Assertion Structure

The `FinalState` assertion type represents a check on the contents of a state table after scenario execution:

```go
type FinalState struct {
    Table  string                    // Name of state table (e.g., "cart_items")
    Where  map[string]ir.IRValue     // Row identification (e.g., {"item_id": "widget"})
    Expect map[string]ir.IRValue     // Expected column values (subset)
}
```

**Key Design Decisions:**
- `Where` uses `ir.IRValue` types for type safety
- `Expect` is a **subset** - only specified columns are checked
- Extra columns in actual DB row are intentionally ignored
- This allows assertions to focus on relevant fields without brittle full-row matching

#### Parameterized SQL Query (HIGH-3)

All SQL queries MUST use parameterized placeholders per HIGH-3:

```go
func (h *Harness) buildWhereClause(where map[string]ir.IRValue) (sql string, args []any) {
    if len(where) == 0 {
        return "", nil
    }

    // Sort keys for determinism
    keys := make([]string, 0, len(where))
    for k := range where {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    clauses := make([]string, 0, len(keys))
    args = make([]any, 0, len(keys))

    for _, key := range keys {
        clauses = append(clauses, fmt.Sprintf("%s = ?", key))
        args = append(args, h.toSQLValue(where[key]))
    }

    return strings.Join(clauses, " AND "), args
}

func (h *Harness) toSQLValue(v ir.IRValue) any {
    switch val := v.(type) {
    case ir.IRString:
        return string(val)
    case ir.IRInt:
        return int64(val)
    case ir.IRBool:
        return bool(val)
    default:
        // For complex types, marshal to JSON
        data, _ := json.Marshal(val)
        return string(data)
    }
}
```

**CRITICAL:** Never use `fmt.Sprintf` to insert values into SQL - always use `?` and pass values separately.

#### Subset Check Behavior

The assertion only validates fields present in `Expect`:

```yaml
# Scenario YAML
assertions:
  - type: final_state
    table: inventory
    where: { item_id: "widget" }
    expect: { quantity: 7 }  # Only checks quantity, ignores other columns
```

```go
// In assertFinalState()
for key, expected := range assertion.Expect {
    // Only iterate over EXPECTED fields, not ALL fields in actualRow
    actual, ok := actualRow[key]
    if !ok {
        return fmt.Errorf("expected field %q not present in result", key)
    }

    if !h.valuesEqual(expected, actual) {
        return fmt.Errorf("field %q: expected %v, got %v", key, expected, actual)
    }
}

// Extra columns in actualRow are never checked - this is intentional!
```

**Rationale:** Allows state tables to evolve with additional columns without breaking existing assertions.

#### Missing Row Handling

When a row is not found, provide a clear, actionable error:

```go
err = row.Scan(valuePtrs...)
if err == sql.ErrNoRows {
    // Build helpful error with WHERE clause info
    whereStr := ""
    for k, v := range assertion.Where {
        whereStr += fmt.Sprintf("%s=%v ", k, v)
    }
    return fmt.Errorf("row not found in table %s with WHERE %s", assertion.Table, whereStr)
}
```

**Example error message:**
```
row not found in table cart_items with WHERE item_id="widget"
```

#### Type-Safe Comparison

The `valuesEqual()` function handles type-aware comparison:

```go
func (h *Harness) valuesEqual(expected ir.IRValue, actual any) bool {
    switch exp := expected.(type) {
    case ir.IRString:
        actualStr, ok := actual.(string)
        if !ok {
            return false // Type mismatch
        }
        return string(exp) == actualStr

    case ir.IRInt:
        actualInt, ok := actual.(int64)
        if !ok {
            return false // Type mismatch
        }
        return int64(exp) == actualInt

    case ir.IRBool:
        actualBool, ok := actual.(bool)
        if !ok {
            return false // Type mismatch
        }
        return bool(exp) == actualBool

    case ir.IRArray, ir.IRObject:
        // For complex types, use deep equality
        return reflect.DeepEqual(expected, actual)

    default:
        return false
    }
}
```

**Key Points:**
- Type assertion (`ok`) detects type mismatches
- Each ir.IRValue type has explicit handling
- Complex types fall back to `reflect.DeepEqual`
- No floats (not in ir.IRValue type system per CP-5)

#### Table Column Metadata

To scan dynamic result sets, we need column metadata:

```go
func (h *Harness) getTableColumns(tableName string) ([]string, error) {
    // Query SQLite metadata
    query := "PRAGMA table_info(?)"
    rows, err := h.store.Query(context.Background(), query, tableName)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    columns := []string{}
    for rows.Next() {
        var cid int
        var name string
        var typ string
        var notNull int
        var dfltValue any
        var pk int

        err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk)
        if err != nil {
            return nil, err
        }
        columns = append(columns, name)
    }

    return columns, nil
}
```

**Alternative:** Cache column metadata per table after first query for performance.

---

## Test Examples

### Test 1: Row Found with Expected Values (Pass)

```go
func TestFinalState_RowFound_Pass(t *testing.T) {
    h := setupHarness(t)

    // Setup: Insert test row
    h.store.Exec(`INSERT INTO cart_items (item_id, quantity) VALUES (?, ?)`,
        "widget", 3)

    // Assertion: Check quantity
    assertion := FinalState{
        Table:  "cart_items",
        Where:  map[string]ir.IRValue{"item_id": ir.IRString("widget")},
        Expect: map[string]ir.IRValue{"quantity": ir.IRInt(3)},
    }

    err := h.assertFinalState(context.Background(), assertion)
    require.NoError(t, err)
}
```

### Test 2: Missing Row Reports Clear Error (Fail)

```go
func TestFinalState_RowNotFound_Error(t *testing.T) {
    h := setupHarness(t)

    // No row inserted

    // Assertion: Expect row that doesn't exist
    assertion := FinalState{
        Table:  "cart_items",
        Where:  map[string]ir.IRValue{"item_id": ir.IRString("widget")},
        Expect: map[string]ir.IRValue{"quantity": ir.IRInt(3)},
    }

    err := h.assertFinalState(context.Background(), assertion)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "row not found")
    assert.Contains(t, err.Error(), "cart_items")
}
```

### Test 3: Extra Columns in Actual Result Ignored (Pass)

```go
func TestFinalState_ExtraColumns_Ignored(t *testing.T) {
    h := setupHarness(t)

    // Setup: Insert row with multiple columns
    h.store.Exec(`INSERT INTO inventory (item_id, quantity, reserved, last_updated)
                   VALUES (?, ?, ?, ?)`,
        "widget", 10, 3, time.Now().Unix())

    // Assertion: Only check quantity (ignore reserved, last_updated)
    assertion := FinalState{
        Table:  "inventory",
        Where:  map[string]ir.IRValue{"item_id": ir.IRString("widget")},
        Expect: map[string]ir.IRValue{"quantity": ir.IRInt(10)},
        // reserved and last_updated are NOT in Expect - should be ignored
    }

    err := h.assertFinalState(context.Background(), assertion)
    require.NoError(t, err) // Pass even though extra columns exist
}
```

### Test 4: Type Mismatch Detected (Fail)

```go
func TestFinalState_TypeMismatch_Error(t *testing.T) {
    h := setupHarness(t)

    // Setup: quantity is an integer in DB
    h.store.Exec(`INSERT INTO cart_items (item_id, quantity) VALUES (?, ?)`,
        "widget", 3)

    // Assertion: Expect quantity as string (type mismatch)
    assertion := FinalState{
        Table:  "cart_items",
        Where:  map[string]ir.IRValue{"item_id": ir.IRString("widget")},
        Expect: map[string]ir.IRValue{"quantity": ir.IRString("3")}, // Wrong type!
    }

    err := h.assertFinalState(context.Background(), assertion)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "expected")
    assert.Contains(t, err.Error(), "type")
}
```

### Test 5: Value Mismatch Shows Clear Diff (Fail)

```go
func TestFinalState_ValueMismatch_ClearError(t *testing.T) {
    h := setupHarness(t)

    // Setup: quantity is 5
    h.store.Exec(`INSERT INTO cart_items (item_id, quantity) VALUES (?, ?)`,
        "widget", 5)

    // Assertion: Expect quantity 3 (wrong value)
    assertion := FinalState{
        Table:  "cart_items",
        Where:  map[string]ir.IRValue{"item_id": ir.IRString("widget")},
        Expect: map[string]ir.IRValue{"quantity": ir.IRInt(3)},
    }

    err := h.assertFinalState(context.Background(), assertion)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "quantity")
    assert.Contains(t, err.Error(), "expected 3")
    assert.Contains(t, err.Error(), "got 5")
}
```

### Test 6: Parameterized Query Construction

```go
func TestBuildWhereClause_Parameterized(t *testing.T) {
    h := setupHarness(t)

    where := map[string]ir.IRValue{
        "item_id": ir.IRString("widget"),
        "status":  ir.IRString("active"),
    }

    sql, args := h.buildWhereClause(where)

    // Verify parameterized SQL (sorted keys for determinism)
    assert.Equal(t, "item_id = ? AND status = ?", sql)
    assert.Equal(t, []any{"widget", "active"}, args)

    // Verify NO string interpolation
    assert.NotContains(t, sql, "widget")
    assert.NotContains(t, sql, "active")
}
```

---

## File List

### New Files
- `internal/harness/assertions.go` - FinalState assertion type and logic
- `internal/harness/assertions_test.go` - Unit tests for FinalState

### Modified Files
- `internal/harness/scenario.go` - Add `final_state` to assertion loading
- `internal/harness/harness.go` - Integrate FinalState into assertion evaluation

---

## Relationship to Other Stories

### Dependencies (Blockers)
- **Story 6.3: Trace Assertions** - Establishes assertion framework and scenario execution

### Dependent Stories (Blocked By This)
- **Story 6.5: Operational Principle Validation** - Uses FinalState to validate state outcomes

### Related Stories
- **Story 6.1: Scenario Definition Format** - Defines YAML assertion format
- **Story 6.2: Test Execution Engine** - Provides harness runner that evaluates assertions
- **Story 2.3: Write Invocations and Completions** - HIGH-3 parameterized queries pattern
- **Story 4.3: SQL Backend Compiler** - Parameterized SQL generation patterns

---

## Story Completion Checklist

### Definition of Done
- [ ] `FinalState` struct implemented with all fields
- [ ] `assertFinalState()` function implemented with parameterized SQL
- [ ] Subset checking works correctly (extra columns ignored)
- [ ] Missing row error is clear and actionable
- [ ] Type mismatch error shows both types
- [ ] All unit tests passing (6+ test cases)
- [ ] Integration test with full scenario passes
- [ ] Code follows HIGH-3 parameterized query pattern
- [ ] Code follows CP-5 constrained IRValue types
- [ ] Error messages are developer-friendly
- [ ] Documentation updated with examples

### Code Quality
- [ ] No `fmt.Sprintf` with SQL values (only `?` placeholders)
- [ ] All `ir.IRValue` types handled in `valuesEqual()`
- [ ] No floats used (CP-5 compliance)
- [ ] Clear separation: query building vs. value comparison
- [ ] Table-driven unit tests with clear names

### Testing
- [ ] Row found with matching values → pass
- [ ] Row found with mismatched values → fail
- [ ] Row not found → clear error
- [ ] Extra columns ignored → pass
- [ ] Type mismatch → clear error
- [ ] Parameterized SQL verified (no interpolation)

---

## References

### Architecture Document
- **HIGH-3: Security Model Foundation** - Parameterized queries only (Architecture page 263)
- **CP-5: Constrained Value Types** - No floats, explicit ir.IRValue types (Architecture page 1020)
- **Naming Patterns** - SQL tables: snake_case (Architecture page 1080)

### PRD
- **FR-6.2: Run scenarios with assertions on action traces** - Final state is part of scenario validation
- **NFR-2.1: Deterministic replay** - State assertions verify projection correctness

### Epic Document
- **Epic 6: Conformance Harness** - Story 6.4 is part of test assertion framework
- **Story 6.3: Trace Assertions** - Establishes assertion pattern
- **Story 6.5: Operational Principle Validation** - Next story, uses FinalState

---

## Dev Agent Record

### Session Log
_This section will be populated by the dev agent during implementation._

**Session Date:** _TBD_
**Agent:** _TBD_
**Time Spent:** _TBD_

#### Implementation Notes
_Dev agent to document:_
- Key decisions made during implementation
- Challenges encountered and solutions
- Deviations from spec (if any) with rationale
- Test results and coverage metrics

#### Questions/Clarifications
_Dev agent to document any questions that arose:_
- Question 1: ...
- Answer: ...

#### Completion Summary
_Dev agent to complete upon story finish:_
- [ ] All acceptance criteria met
- [ ] All tasks completed
- [ ] Tests passing
- [ ] Code reviewed (self or peer)
- [ ] Documentation complete

---

**Story Status:** Ready for implementation
**Last Updated:** 2025-12-12
