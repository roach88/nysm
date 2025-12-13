# Story 1.7: CUE Sync Rule Parser

Status: ready-for-dev

## Story

As a **developer defining synchronizations**,
I want **to write sync rules in CUE with when/where/then clauses**,
So that **I can define reactive coordination between concepts**.

## Acceptance Criteria

1. **`CompileSync(cueValue cue.Value) (*SyncRule, error)` in `internal/compiler/sync.go`**
   - Takes a CUE value representing a sync definition
   - Returns parsed `ir.SyncRule` or error

2. **SyncRule IR struct defined**
   ```go
   type SyncRule struct {
       ID    string      `json:"id"`
       Scope ScopeSpec   `json:"scope"`
       When  WhenClause  `json:"when"`
       Where *WhereClause `json:"where,omitempty"`
       Then  ThenClause  `json:"then"`
   }
   ```

3. **Scope modes supported: flow, global, keyed**
   ```go
   type ScopeSpec struct {
       Mode string `json:"mode"` // "flow", "global", or "keyed"
       Key  string `json:"key,omitempty"` // field name for keyed mode
   }
   ```

4. **When clause with action ref, event type, output case, bindings**
   ```go
   type WhenClause struct {
       ActionRef  string            `json:"action_ref"`  // "Cart.checkout"
       EventType  string            `json:"event_type"`  // "completed" or "invoked"
       OutputCase string            `json:"output_case,omitempty"` // "Success", etc.
       Bindings   map[string]string `json:"bindings"`    // var name → path
   }
   ```

5. **Where clause with source, filter, bindings**
   ```go
   type WhereClause struct {
       Source   string            `json:"source"`   // "CartItem"
       Filter   string            `json:"filter"`   // expression string
       Bindings map[string]string `json:"bindings"` // var name → path
   }
   ```

6. **Then clause with action ref and arg template**
   ```go
   type ThenClause struct {
       ActionRef string            `json:"action_ref"` // "Inventory.reserve"
       Args      map[string]string `json:"args"`       // arg name → expression
   }
   ```

7. **CUE sync spec format parsed**
   ```cue
   // NOTE: All references use string literals for consistent parsing.
   // This avoids CUE expression evaluation complexities.
   sync "cart-inventory" {
       scope: "flow"  // or "global" or "keyed(\"user_id\")"

       when: {
           action: "Cart.checkout"
           event: "completed"
           case: "Success"
           bind: { cart_id: "result.cart_id" }
       }

       where: {
           from: "CartItem"
           filter: "cart_id == bound.cart_id"
           bind: { item_id: "item_id", quantity: "quantity" }
       }

       then: {
           action: "Inventory.reserve"
           args: { item_id: "bound.item_id", quantity: "bound.quantity" }
       }
   }
   ```

8. **Validation of scope modes**
   - Only "flow", "global", or keyed("field") allowed
   - Invalid scope produces clear error

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-2.1** | 3-clause sync rules (when/where/then) |
| **HIGH-1** | Flow token scoping modes |
| **MEDIUM-2** | Output case matching in when-clause |

## Tasks / Subtasks

- [ ] Task 1: Define SyncRule IR structs (AC: #2-6)
  - [ ] 1.1 Create `internal/ir/sync.go`
  - [ ] 1.2 Define SyncRule, ScopeSpec, WhenClause, WhereClause, ThenClause

- [ ] Task 2: Create sync compiler (AC: #1)
  - [ ] 2.1 Create `internal/compiler/sync.go`
  - [ ] 2.2 Implement CompileSync function

- [ ] Task 3: Parse scope modes (AC: #3, #8)
  - [ ] 3.1 Parse "flow" scope
  - [ ] 3.2 Parse "global" scope
  - [ ] 3.3 Parse keyed("field") scope
  - [ ] 3.4 Validate scope modes

- [ ] Task 4: Parse when clause (AC: #4)
  - [ ] 4.1 Parse action reference (Concept.action)
  - [ ] 4.2 Parse event type (completed/invoked)
  - [ ] 4.3 Parse output case (for completed events)
  - [ ] 4.4 Parse bindings

- [ ] Task 5: Parse where clause (AC: #5)
  - [ ] 5.1 Parse source (state reference)
  - [ ] 5.2 Parse filter expression
  - [ ] 5.3 Parse bindings

- [ ] Task 6: Parse then clause (AC: #6)
  - [ ] 6.1 Parse action reference
  - [ ] 6.2 Parse arg templates with bound variables

- [ ] Task 7: Write comprehensive tests
  - [ ] 7.1 Test valid sync rule parsing
  - [ ] 7.2 Test all scope modes
  - [ ] 7.3 Test output case matching
  - [ ] 7.4 Test invalid scope error

## Dev Notes

### SyncRule IR Structs

```go
// internal/ir/sync.go
package ir

// SyncRule represents a compiled synchronization rule.
// Sync rules define reactive coordination between concepts.
type SyncRule struct {
    ID    string       `json:"id"`
    Scope ScopeSpec    `json:"scope"`
    When  WhenClause   `json:"when"`
    Where *WhereClause `json:"where,omitempty"` // Optional
    Then  ThenClause   `json:"then"`
}

// ScopeSpec defines the scoping mode for a sync rule.
type ScopeSpec struct {
    Mode string `json:"mode"` // "flow", "global", or "keyed"
    Key  string `json:"key,omitempty"` // field name for keyed mode
}

// ValidScopeModes defines allowed scope modes
var ValidScopeModes = map[string]bool{
    "flow":   true,
    "global": true,
    "keyed":  true,
}

// WhenClause defines the trigger condition for a sync rule.
type WhenClause struct {
    ActionRef  string            `json:"action_ref"`  // "Cart.checkout"
    EventType  string            `json:"event_type"`  // "completed" or "invoked"
    OutputCase string            `json:"output_case,omitempty"` // For completed events
    Bindings   map[string]string `json:"bindings"`    // var → path expression
}

// WhereClause defines the query/filter for a sync rule.
type WhereClause struct {
    Source   string            `json:"source"`   // State reference
    Filter   string            `json:"filter"`   // Filter expression
    Bindings map[string]string `json:"bindings"` // var → path expression
}

// ThenClause defines the action to invoke when sync fires.
type ThenClause struct {
    ActionRef string            `json:"action_ref"` // "Inventory.reserve"
    Args      map[string]string `json:"args"`       // arg → expression using bound vars
}
```

### Sync Compiler Implementation

```go
// internal/compiler/sync.go
package compiler

import (
    "fmt"
    "regexp"
    "strings"

    "cuelang.org/go/cue"
    // Note: Uses CompileError and formatCUEError from concept.go (same package)
    // token.Pos is used for source positions - imported in concept.go

    "github.com/your-org/nysm/internal/ir"
)

// keyedPattern matches keyed("field_name")
var keyedPattern = regexp.MustCompile(`^keyed\("([^"]+)"\)$`)

// CompileSync parses a CUE value into a SyncRule.
func CompileSync(v cue.Value) (*ir.SyncRule, error) {
    if err := v.Err(); err != nil {
        return nil, formatCUEError(err)
    }

    rule := &ir.SyncRule{}

    // Parse sync ID from struct label
    // e.g., `sync "cart-inventory" { ... }` → id is "cart-inventory"
    labels := v.Path().Selectors()
    if len(labels) > 0 {
        // The ID is in quotes in CUE, extract it
        rule.ID = strings.Trim(labels[len(labels)-1].String(), `"`)
    }

    // Parse scope (required)
    var err error
    rule.Scope, err = parseScope(v)
    if err != nil {
        return nil, err
    }

    // Parse when clause (required)
    rule.When, err = parseWhenClause(v)
    if err != nil {
        return nil, err
    }

    // Parse where clause (optional)
    whereVal := v.LookupPath(cue.ParsePath("where"))
    if whereVal.Exists() {
        where, err := parseWhereClause(whereVal)
        if err != nil {
            return nil, err
        }
        rule.Where = where
    }

    // Parse then clause (required)
    rule.Then, err = parseThenClause(v)
    if err != nil {
        return nil, err
    }

    return rule, nil
}

func parseScope(v cue.Value) (ir.ScopeSpec, error) {
    scopeVal := v.LookupPath(cue.ParsePath("scope"))
    if !scopeVal.Exists() {
        return ir.ScopeSpec{}, &CompileError{
            Field:   "scope",
            Message: "scope is required",
            Pos:     v.Pos(),
        }
    }

    scopeStr, err := scopeVal.String()
    if err != nil {
        return ir.ScopeSpec{}, formatCUEError(err)
    }

    // Check for keyed("field") pattern
    if matches := keyedPattern.FindStringSubmatch(scopeStr); matches != nil {
        return ir.ScopeSpec{
            Mode: "keyed",
            Key:  matches[1],
        }, nil
    }

    // Check for simple modes: flow, global
    if !ir.ValidScopeModes[scopeStr] {
        return ir.ScopeSpec{}, &CompileError{
            Field:   "scope",
            Message: fmt.Sprintf("invalid scope %q, must be 'flow', 'global', or keyed(\"field\")", scopeStr),
            Pos:     scopeVal.Pos(),
        }
    }

    return ir.ScopeSpec{Mode: scopeStr}, nil
}

func parseWhenClause(v cue.Value) (ir.WhenClause, error) {
    whenVal := v.LookupPath(cue.ParsePath("when"))
    if !whenVal.Exists() {
        return ir.WhenClause{}, &CompileError{
            Field:   "when",
            Message: "when clause is required",
            Pos:     v.Pos(),
        }
    }

    when := ir.WhenClause{
        Bindings: make(map[string]string),
    }

    // Parse action reference (required string field)
    actionRefVal := whenVal.LookupPath(cue.ParsePath("action"))
    if !actionRefVal.Exists() {
        return when, &CompileError{
            Field:   "when.action",
            Message: "when clause requires 'action' field",
            Pos:     whenVal.Pos(),
        }
    }
    actionRef, err := actionRefVal.String()
    if err != nil {
        return when, formatCUEError(err)
    }
    when.ActionRef = actionRef

    // Parse event type (required string field)
    eventVal := whenVal.LookupPath(cue.ParsePath("event"))
    if !eventVal.Exists() {
        return when, &CompileError{
            Field:   "when.event",
            Message: "when clause requires 'event' field (\"completed\" or \"invoked\")",
            Pos:     whenVal.Pos(),
        }
    }
    event, err := eventVal.String()
    if err != nil {
        return when, formatCUEError(err)
    }
    if event != "completed" && event != "invoked" {
        return when, &CompileError{
            Field:   "when.event",
            Message: fmt.Sprintf("invalid event type %q, must be \"completed\" or \"invoked\"", event),
            Pos:     eventVal.Pos(),
        }
    }
    when.EventType = event

    // Parse output case (optional, for completed events)
    caseVal := whenVal.LookupPath(cue.ParsePath("case"))
    if caseVal.Exists() {
        caseName, err := caseVal.String()
        if err != nil {
            return when, formatCUEError(err)
        }
        when.OutputCase = caseName
    }

    // Parse bindings (string values only)
    bindVal := whenVal.LookupPath(cue.ParsePath("bind"))
    if bindVal.Exists() {
        iter, err := bindVal.Fields()
        if err != nil {
            return when, formatCUEError(err)
        }

        for iter.Next() {
            varName := iter.Label()
            pathExpr, err := iter.Value().String()
            if err != nil {
                return when, &CompileError{
                    Field:   fmt.Sprintf("when.bind.%s", varName),
                    Message: "binding value must be a string path expression",
                    Pos:     iter.Value().Pos(),
                }
            }
            when.Bindings[varName] = pathExpr
        }
    }

    return when, nil
}

func parseWhereClause(v cue.Value) (*ir.WhereClause, error) {
    where := &ir.WhereClause{
        Bindings: make(map[string]string),
    }

    // Parse source (from field - required string)
    fromVal := v.LookupPath(cue.ParsePath("from"))
    if !fromVal.Exists() {
        return nil, &CompileError{
            Field:   "where.from",
            Message: "where clause requires 'from' field",
            Pos:     v.Pos(),
        }
    }
    from, err := fromVal.String()
    if err != nil {
        return nil, &CompileError{
            Field:   "where.from",
            Message: "from field must be a string state reference",
            Pos:     fromVal.Pos(),
        }
    }
    where.Source = from

    // Parse filter expression (string)
    filterVal := v.LookupPath(cue.ParsePath("filter"))
    if filterVal.Exists() {
        filter, err := filterVal.String()
        if err != nil {
            return nil, &CompileError{
                Field:   "where.filter",
                Message: "filter must be a string expression",
                Pos:     filterVal.Pos(),
            }
        }
        where.Filter = filter
    }

    // Parse bindings (all string values)
    bindVal := v.LookupPath(cue.ParsePath("bind"))
    if bindVal.Exists() {
        iter, err := bindVal.Fields()
        if err != nil {
            return nil, formatCUEError(err)
        }

        for iter.Next() {
            varName := iter.Label()
            pathExpr, err := iter.Value().String()
            if err != nil {
                return nil, &CompileError{
                    Field:   fmt.Sprintf("where.bind.%s", varName),
                    Message: "binding value must be a string path expression",
                    Pos:     iter.Value().Pos(),
                }
            }
            where.Bindings[varName] = pathExpr
        }
    }

    return where, nil
}

func parseThenClause(v cue.Value) (ir.ThenClause, error) {
    thenVal := v.LookupPath(cue.ParsePath("then"))
    if !thenVal.Exists() {
        return ir.ThenClause{}, &CompileError{
            Field:   "then",
            Message: "then clause is required",
            Pos:     v.Pos(),
        }
    }

    then := ir.ThenClause{
        Args: make(map[string]string),
    }

    // Parse action reference (required string field)
    actionVal := thenVal.LookupPath(cue.ParsePath("action"))
    if !actionVal.Exists() {
        return then, &CompileError{
            Field:   "then.action",
            Message: "then clause requires 'action' field",
            Pos:     thenVal.Pos(),
        }
    }
    action, err := actionVal.String()
    if err != nil {
        return then, &CompileError{
            Field:   "then.action",
            Message: "action must be a string action reference",
            Pos:     actionVal.Pos(),
        }
    }
    then.ActionRef = action

    // Parse args (all string values)
    argsVal := thenVal.LookupPath(cue.ParsePath("args"))
    if argsVal.Exists() {
        iter, err := argsVal.Fields()
        if err != nil {
            return then, formatCUEError(err)
        }

        for iter.Next() {
            argName := iter.Label()
            argExpr, err := iter.Value().String()
            if err != nil {
                return then, &CompileError{
                    Field:   fmt.Sprintf("then.args.%s", argName),
                    Message: "arg value must be a string expression",
                    Pos:     iter.Value().Pos(),
                }
            }
            then.Args[argName] = argExpr
        }
    }

    return then, nil
}
```

### Test Examples

```go
func TestCompileSyncBasic(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "cart-inventory" {
            scope: "flow"

            when: {
                action: "Cart.checkout"
                event: "completed"
                case: "Success"
                bind: { cart_id: "result.cart_id" }
            }

            where: {
                from: "CartItem"
                filter: "cart_id == bound.cart_id"
                bind: { item_id: "item_id", quantity: "quantity" }
            }

            then: {
                action: "Inventory.reserve"
                args: {
                    item_id: "bound.item_id"
                    quantity: "bound.quantity"
                }
            }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."cart-inventory"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Equal(t, "cart-inventory", rule.ID)
    assert.Equal(t, "flow", rule.Scope.Mode)
    assert.Equal(t, "Cart.checkout", rule.When.ActionRef)
    assert.Equal(t, "completed", rule.When.EventType)
    assert.Equal(t, "Success", rule.When.OutputCase)
    assert.NotNil(t, rule.Where)
    assert.Equal(t, "Inventory.reserve", rule.Then.ActionRef)
}

func TestCompileSyncScopeFlow(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "test" {
            scope: "flow"
            when: { action: "A.b", event: "completed" }
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Equal(t, "flow", rule.Scope.Mode)
    assert.Empty(t, rule.Scope.Key)
}

func TestCompileSyncScopeGlobal(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "test" {
            scope: "global"
            when: { action: "A.b", event: "completed" }
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Equal(t, "global", rule.Scope.Mode)
}

func TestCompileSyncScopeKeyed(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "test" {
            scope: "keyed(\"user_id\")"
            when: { action: "A.b", event: "completed" }
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Equal(t, "keyed", rule.Scope.Mode)
    assert.Equal(t, "user_id", rule.Scope.Key)
}

func TestCompileSyncInvalidScope(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "test" {
            scope: "invalid_scope"
            when: { action: "A.b", event: "completed" }
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."test"`))
    _, err := CompileSync(syncVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "invalid scope")
}

func TestCompileSyncOutputCaseMatching(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "handle-error" {
            scope: "flow"

            when: {
                action: "Cart.checkout"
                event: "completed"
                case: "InsufficientFunds"
                bind: { amount: "result.required_amount" }
            }

            then: {
                action: "Notification.send"
                args: { message: "bound.amount" }
            }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."handle-error"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Equal(t, "InsufficientFunds", rule.When.OutputCase)
}

func TestCompileSyncNoWhereClause(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "simple" {
            scope: "flow"
            when: { action: "A.b", event: "completed" }
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."simple"`))
    rule, err := CompileSync(syncVal)

    require.NoError(t, err)
    assert.Nil(t, rule.Where, "where clause should be optional")
}

func TestCompileSyncMissingWhen(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "bad" {
            scope: "flow"
            then: { action: "C.d" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
    _, err := CompileSync(syncVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "when")
}

func TestCompileSyncMissingThen(t *testing.T) {
    ctx := cuecontext.New()
    v := ctx.CompileString(`
        sync "bad" {
            scope: "flow"
            when: { action: "A.b", event: "completed" }
        }
    `)

    syncVal := v.LookupPath(cue.ParsePath(`sync."bad"`))
    _, err := CompileSync(syncVal)

    require.Error(t, err)
    assert.Contains(t, err.Error(), "then")
}
```

### File List

Files to create/modify:

1. `internal/ir/sync.go` - SyncRule and related structs
2. `internal/compiler/sync.go` - CompileSync function
3. `internal/compiler/sync_test.go` - Comprehensive tests

### Relationship to Other Stories

- **Story 1-6:** Similar pattern to concept parsing
- **Story 1-8:** SyncRule validated after parsing
- **Story 3-2:** Sync rules used by sync engine

### Story Completion Checklist

- [ ] SyncRule struct defined
- [ ] ScopeSpec struct with flow/global/keyed modes
- [ ] WhenClause struct with action ref, event type, output case, bindings
- [ ] WhereClause struct with source, filter, bindings
- [ ] ThenClause struct with action ref, args
- [ ] CompileSync function implemented
- [ ] All three scope modes parsed correctly
- [ ] keyed("field") pattern recognized
- [ ] Invalid scope produces clear error
- [ ] Output case matching supported (MEDIUM-2)
- [ ] All tests pass
- [ ] `go vet ./internal/...` passes

### References

- [Source: docs/architecture.md#Sync Rules] - Sync rule structure
- [Source: docs/epics.md#Story 1.7] - Story definition
- [Source: docs/prd.md#FR-2.1] - 3-clause sync rules
- [Source: docs/prd.md#HIGH-1] - Flow token scoping modes
- [Source: docs/prd.md#MEDIUM-2] - Output case matching

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- 2025-12-12: codex review - changed DSL from expressions to string literals for consistent parsing, made parser stricter about string types, added required field validation

### Completion Notes

- Three scope modes: flow (default), global, keyed("field")
- When clause supports output case matching for error handling
- Where clause is optional (not all syncs need queries)
- Then clause references action with bound variable args
