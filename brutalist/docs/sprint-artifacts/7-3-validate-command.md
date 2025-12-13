---
story_id: 7.3
epic_id: 7
title: Validate Command
status: ready-for-dev
created: 2025-12-12
updated: 2025-12-12
assignee: unassigned
---

# Story 7.3: Validate Command

## Story Statement

As a **developer checking specs**,
I want **a validate command that checks specs without full compilation**,
So that **I can get fast feedback during development**.

## Acceptance Criteria

### AC-1: Basic Validation with Success Output
**Given** valid CUE spec files
**When** I run `nysm validate ./specs`
**Then** I get validation results:
```bash
$ nysm validate ./specs
✓ All specs valid
```
**And** exit code is 0

### AC-2: Validation with Errors
**Given** CUE spec files with validation errors
**When** I run `nysm validate ./specs-with-errors`
**Then** I get validation results with file:line references:
```bash
$ nysm validate ./specs-with-errors
✗ Validation failed

specs/cart.concept.cue:15:3
  E101: Missing required field: purpose

specs/cart-inventory.sync.cue:8:5
  E102: Unknown action reference: Cart.invalid
```
**And** exit code is 1
**And** all errors are collected and reported (not fail-fast)

### AC-3: Exit Code Semantics
**Given** any validation run
**When** the command completes
**Then** exit code is:
- 0 for success (all specs valid)
- 1 for validation errors

### AC-4: Performance Characteristics
**Given** any spec files
**When** I run validate vs compile
**Then** validate is faster than compile (skips IR generation)

### Signature Checklist

All acceptance criteria implementations must use these Go signatures:

```go
// internal/cli/validate.go
func ValidateCommand() *cobra.Command

// internal/compiler/validate.go
func ValidateSpecs(specsDir string) ([]ValidationError, error)

// internal/compiler/validate.go
type ValidationError struct {
    File    string
    Line    int
    Column  int
    Code    string  // E101, E102, etc.
    Message string
}

func (e ValidationError) String() string
```

## Quick Reference

### Related Patterns

| Pattern | Location | Usage |
|---------|----------|-------|
| CLI Command Structure | `internal/cli/*.go` | Cobra command implementation |
| Error Code Format | Architecture § Error Codes | E001-E099 compilation, E100-E199 validation |
| Exit Code Convention | Architecture § CLI Output | 0 = success, 1 = errors |
| Validation Rules | `internal/compiler/validate.go` | Concept/sync validation logic |

### Key Architecture References

| Reference | Section | Key Details |
|-----------|---------|-------------|
| Error Collection | Architecture § Error Wrapping | Collect all errors, don't fail-fast |
| CLI Flags | Architecture § CLI Flags | `kebab-case` long flags, single-letter short |
| Error Output Format | Architecture § CLI Output Structure | file:line:column format with error code |
| Validation Scope | Epic 1, Story 1.8 | ConceptSpec and SyncRule validation |

## Tasks/Subtasks

### Task 1: Implement ValidateCommand CLI
- [ ] Create `internal/cli/validate.go` with Cobra command
- [ ] Add `--format` flag (text/json)
- [ ] Add `--verbose` flag for detailed output
- [ ] Wire command into root command in `cmd/nysm/main.go`
- [ ] Return appropriate exit codes (0/1)

### Task 2: Implement Fast Validation Path
- [ ] Create `ValidateSpecs()` in `internal/compiler/validate.go`
- [ ] Parse CUE files without full IR generation
- [ ] Run schema validation on parsed specs
- [ ] Collect all validation errors (non-fail-fast)
- [ ] Format errors with file:line:column

### Task 3: ConceptSpec Validation Rules
- [ ] Check `purpose` is non-empty
- [ ] Check at least one `action` defined
- [ ] Check each action has at least one output case
- [ ] Check state field types are valid IRValue types
- [ ] Check no duplicate action or state names

### Task 4: SyncRule Validation Rules
- [ ] Check `when` references a valid action
- [ ] Check `scope` is one of: "flow", "global", or keyed("field")
- [ ] Check `where` clause is syntactically valid
- [ ] Check `then` references a valid action
- [ ] Check bound variables in `then` are defined in `when` or `where`

### Task 5: Error Output Formatting
- [ ] Implement `ValidationError.String()` for human output
- [ ] Implement JSON output format for `--format=json`
- [ ] Sort errors by file, then line, then column
- [ ] Add color coding for terminal output (red for errors)
- [ ] Test output matches format in acceptance criteria

### Task 6: Tests
- [ ] Unit tests for `ValidateSpecs()` with table-driven cases
- [ ] Test valid concept spec
- [ ] Test invalid concept spec (missing purpose, no actions, etc.)
- [ ] Test valid sync rule
- [ ] Test invalid sync rule (unknown action, bad scope, etc.)
- [ ] Test error collection (multiple errors in same file)
- [ ] Test CLI exit codes
- [ ] Golden file tests for formatted output

## Dev Notes

### Implementation Details

#### Command Implementation
```go
// internal/cli/validate.go
package cli

import (
    "fmt"
    "github.com/spf13/cobra"
    "github.com/tyler/nysm/internal/compiler"
)

func ValidateCommand() *cobra.Command {
    var format string
    var verbose bool

    cmd := &cobra.Command{
        Use:   "validate <specs-dir>",
        Short: "Validate specs without full compilation",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            specsDir := args[0]

            errors, err := compiler.ValidateSpecs(specsDir)
            if err != nil {
                return fmt.Errorf("validate: %w", err)
            }

            if len(errors) == 0 {
                fmt.Println("✓ All specs valid")
                return nil
            }

            // Format and print errors
            fmt.Println("✗ Validation failed")
            fmt.Println()
            for _, e := range errors {
                fmt.Println(e.String())
            }

            // Exit with error code
            cmd.SilenceUsage = true  // Don't show usage on validation errors
            return fmt.Errorf("validation failed with %d errors", len(errors))
        },
    }

    cmd.Flags().StringVar(&format, "format", "text", "Output format (text|json)")
    cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")

    return cmd
}
```

#### Fast Validation Path
The validate command should:
1. Parse CUE files using CUE SDK
2. Extract ConceptSpec and SyncRule structures
3. Run validation rules WITHOUT generating canonical IR
4. Collect all errors (don't stop at first error)
5. Return errors with file location information

This is faster than compile because:
- Skips canonical JSON generation (RFC 8785 sorting)
- Skips content-addressed ID computation
- Skips IR schema serialization
- Only validates structure, not full compilation

#### Error Code Assignment
Follow Architecture § Error Codes:
- E100-E199: Validation errors
- E101: Missing required field
- E102: Unknown action reference
- E103: Invalid scope mode
- E104: Unbound variable in then-clause
- E105: Duplicate name
- E106: Invalid field type
- etc.

#### Error Format
```
<file-path>:<line>:<column>
  <error-code>: <message>
```

Example:
```
specs/cart.concept.cue:15:3
  E101: Missing required field: purpose

specs/cart-inventory.sync.cue:8:5
  E102: Unknown action reference: Cart.invalid
```

#### Exit Code Handling
Use Cobra's return error pattern:
- Return nil → exit code 0
- Return error → exit code 1
- Set `cmd.SilenceUsage = true` to prevent usage text on validation errors

### Performance Optimization

The validate command should be significantly faster than compile:
- **Compile**: Parse → Validate → Generate IR → Serialize → Hash
- **Validate**: Parse → Validate → Stop

Target: < 100ms for demo specs (3 concepts, 2 syncs)

### Integration with Editor/CI

This command is designed for:
1. **Editor integration**: Fast feedback loop during development
2. **CI pipelines**: Pre-commit hook or PR check
3. **Development workflow**: Quick sanity check before full compile

Example usage:
```bash
# In development
watch -n 1 nysm validate ./specs

# In CI
nysm validate ./specs || exit 1

# Pre-commit hook
#!/bin/sh
nysm validate ./specs
```

## Test Examples

### Test Case 1: Valid Concept Spec
```cue
// testdata/fixtures/concepts/valid_cart.cue
concept Cart {
    purpose: "Manages shopping cart state for a user session"

    state CartItem {
        item_id: string
        quantity: int
    }

    action addItem {
        args: {
            item_id: string
            quantity: int
        }
        outputs: [{
            case: "Success"
            fields: { item_id: string, new_quantity: int }
        }]
    }

    operational_principle: """
        Adding an item increases quantity or creates new entry
        """
}
```

Expected: No errors

### Test Case 2: Invalid Concept - Missing Purpose
```cue
// testdata/fixtures/concepts/invalid_missing_purpose.cue
concept Cart {
    // Missing purpose field

    action addItem {
        args: { item_id: string }
        outputs: [{ case: "Success", fields: {} }]
    }
}
```

Expected:
```
testdata/fixtures/concepts/invalid_missing_purpose.cue:2:1
  E101: Missing required field: purpose
```

### Test Case 3: Invalid Sync - Unknown Action
```cue
// testdata/fixtures/syncs/invalid_unknown_action.cue
sync "cart-inventory" {
    scope: "flow"

    when: Cart.invalidAction.completed {
        case: "Success"
        bind: {}
    }

    where: {}
    then: Inventory.reserve { args: {} }
}
```

Expected:
```
testdata/fixtures/syncs/invalid_unknown_action.cue:4:11
  E102: Unknown action reference: Cart.invalidAction
```

### Test Case 4: Multiple Errors in Same File
```cue
// testdata/fixtures/concepts/invalid_multiple_errors.cue
concept Cart {
    // Error 1: Missing purpose

    state CartItem {
        item_id: string
        price: float  // Error 2: Invalid type (no floats)
    }

    // Error 3: No actions defined
}
```

Expected (all errors reported):
```
testdata/fixtures/concepts/invalid_multiple_errors.cue:2:1
  E101: Missing required field: purpose

testdata/fixtures/concepts/invalid_multiple_errors.cue:7:16
  E106: Invalid field type: float (floats not allowed in IR)

testdata/fixtures/concepts/invalid_multiple_errors.cue:2:1
  E107: Concept must define at least one action
```

## File List

### New Files
- `/Users/tyler/dev/ideas/brutalist/internal/cli/validate.go` - Cobra command implementation
- `/Users/tyler/dev/ideas/brutalist/internal/cli/validate_test.go` - CLI tests
- `/Users/tyler/dev/ideas/brutalist/testdata/fixtures/concepts/valid_cart.cue` - Valid test fixture
- `/Users/tyler/dev/ideas/brutalist/testdata/fixtures/concepts/invalid_missing_purpose.cue` - Invalid test fixture
- `/Users/tyler/dev/ideas/brutalist/testdata/fixtures/syncs/invalid_unknown_action.cue` - Invalid test fixture
- `/Users/tyler/dev/ideas/brutalist/testdata/golden/cli/validate_success.golden` - Golden output
- `/Users/tyler/dev/ideas/brutalist/testdata/golden/cli/validate_errors.golden` - Golden output

### Modified Files
- `/Users/tyler/dev/ideas/brutalist/cmd/nysm/main.go` - Add validate command to root
- `/Users/tyler/dev/ideas/brutalist/internal/compiler/validate.go` - Add `ValidateSpecs()` function (may already exist from Story 1.8)

## Relationship to Other Stories

### Depends On
- **Story 1.8: IR Schema Validation** - Uses validation rules defined in Epic 1
- **Story 7.1: CLI Framework Setup** - Uses Cobra CLI structure
- **Story 7.2: Compile Command** - Similar pattern, but faster (no IR generation)

### Enables
- **Story 7.4: Run Command** - Can validate before running
- **Story 7.6: Test Command** - Can validate before testing
- Editor integration (future) - Fast feedback for developers

### Related
- **Story 1.8: IR Schema Validation** - Shared validation logic
- **Story 7.2: Compile Command** - Compile includes validation + IR generation

## Story Completion Checklist

### Definition of Done
- [ ] All acceptance criteria met
- [ ] All tasks completed
- [ ] Unit tests written and passing
- [ ] Golden file tests passing
- [ ] CLI command wired into root
- [ ] Exit codes work correctly (0/1)
- [ ] Error format matches specification
- [ ] Performance target met (< 100ms for demo specs)
- [ ] Code reviewed against patterns
- [ ] Documentation updated (if needed)

### Code Review Checklist
- [ ] Follows Architecture § CLI Command Structure
- [ ] Follows Architecture § Error Codes (E100-E199)
- [ ] Follows Architecture § Naming Patterns (kebab-case flags)
- [ ] Error handling uses `fmt.Errorf` with `%w`
- [ ] No `time.Now()`, `rand.*` (if applicable)
- [ ] Table-driven tests for all validation rules
- [ ] Golden files committed

### Testing Checklist
- [ ] Valid concept spec validates successfully
- [ ] Invalid concept specs produce correct errors
- [ ] Valid sync rule validates successfully
- [ ] Invalid sync rules produce correct errors
- [ ] Multiple errors collected (not fail-fast)
- [ ] File:line:column format correct
- [ ] Exit codes correct (0 for success, 1 for errors)
- [ ] CLI flags work (`--format`, `--verbose`)
- [ ] Performance acceptable (faster than compile)

## References

### Primary References
- Epic 7: CLI & Demo Application (in `/Users/tyler/dev/ideas/brutalist/docs/epics.md`)
- Story 1.8: IR Schema Validation (in `/Users/tyler/dev/ideas/brutalist/docs/epics.md`)
- Architecture § CLI Commands (in `/Users/tyler/dev/ideas/brutalist/docs/architecture.md`)
- Architecture § Error Codes (in `/Users/tyler/dev/ideas/brutalist/docs/architecture.md`)

### Related PRD Sections
- FR-1.2: Validate concept specs against canonical IR schema
- NFR-4.1: Concept spec validation errors actionable

### External References
- CUE Language Documentation: https://cuelang.org/docs/
- Cobra CLI Framework: https://github.com/spf13/cobra

## Dev Agent Record

### Session Log
| Date | Agent | Action | Notes |
|------|-------|--------|-------|
| 2025-12-12 | - | Story Created | Initial creation from Epic 7 Story 7.3 |

### Questions & Decisions
| Question | Decision | Rationale |
|----------|----------|-----------|
| Should validate run all validation rules or subset? | All rules from Story 1.8 | Need comprehensive validation, not just syntax |
| Should validate support JSON output? | Yes, via `--format` flag | Enables CI/editor integration |
| Should validate stop at first error? | No, collect all | Better DX to see all problems at once |
| Should validate generate IR? | No, parse only | Performance - validate should be fast |

### Implementation Notes
- Use CUE SDK's Go API directly (not CLI subprocess)
- Share validation logic with `internal/compiler/validate.go` from Story 1.8
- Consider caching parsed CUE values for subsequent compiles
- Exit code handling via Cobra's return error pattern
- Color output only when stdout is a terminal (check `isatty`)

### Future Enhancements (Post-Story)
- Watch mode: `nysm validate --watch ./specs`
- Editor integration: LSP server for CUE specs
- Auto-fix suggestions for common errors
- Validation severity levels (error/warning/info)
- Configuration file for custom validation rules
