# Story 6.6: Golden Trace Snapshots

Status: ready-for-dev

## Story

As a **developer maintaining tests**,
I want **golden file comparison for trace snapshots**,
So that **I can easily detect unexpected changes**.

## Acceptance Criteria

1. **`RunWithGolden(scenario *Scenario) error` function in `internal/harness/golden.go`**
   - Executes scenario and captures full trace
   - Compares trace against golden file in `testdata/golden/{scenario}.golden`
   - Uses `github.com/sebdah/goldie/v2` for golden file comparison

2. **Golden files use canonical JSON format**
   - Uses `ir.MarshalCanonical()` from Story 1-4 for deterministic output
   - Ensures golden files are stable across runs
   - Keys sorted by UTF-16 code units per RFC 8785

3. **`-update` flag regenerates golden files**
   - Running `go test ./internal/harness -update` updates all golden files
   - Only updates on explicit flag - never auto-updates
   - Preserves determinism for regression testing

4. **Golden files stored in `testdata/golden/` directory**
   - One `.golden` file per scenario
   - Files committed to repository for version control
   - Clear naming: `{scenario_name}.golden`

5. **Test failure shows readable diff**
   - When golden file doesn't match, test output shows diff
   - Uses goldie's built-in diff formatting
   - Clearly indicates what changed in the trace

6. **Trace format includes all relevant data**
   - Invocations with flow_token, action_uri, args, seq
   - Completions with output_case, result
   - Sync firings with sync_id, binding_hash
   - Provenance edges linking completions → invocations

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-6.4** | Generate golden trace snapshots |
| **CP-2** | Logical clocks (seq) for deterministic ordering |
| **CP-3** | RFC 8785 canonical JSON for stable hashing |

## Tasks / Subtasks

- [ ] Task 1: Implement RunWithGolden function (AC: #1)
  - [ ] 1.1 Create `internal/harness/golden.go`
  - [ ] 1.2 Implement RunWithGolden using goldie.New()
  - [ ] 1.3 Add goldie dependency to go.mod

- [ ] Task 2: Format trace as canonical JSON (AC: #2)
  - [ ] 2.1 Create TraceSnapshot struct with all trace data
  - [ ] 2.2 Marshal using ir.MarshalCanonical()
  - [ ] 2.3 Verify deterministic output with multiple runs

- [ ] Task 3: Support -update flag (AC: #3)
  - [ ] 3.1 Configure goldie to use -update flag
  - [ ] 3.2 Document flag usage in test comments
  - [ ] 3.3 Test regeneration workflow

- [ ] Task 4: Set up golden file directory structure (AC: #4)
  - [ ] 4.1 Create `testdata/golden/` directory
  - [ ] 4.2 Add .gitignore rules (keep .golden files)
  - [ ] 4.3 Document naming convention

- [ ] Task 5: Implement readable diff output (AC: #5)
  - [ ] 5.1 Use goldie's default diff formatter
  - [ ] 5.2 Add test case for mismatch scenario
  - [ ] 5.3 Verify diff readability

- [ ] Task 6: Define trace snapshot format (AC: #6)
  - [ ] 6.1 Define TraceSnapshot struct
  - [ ] 6.2 Include invocations, completions, firings, edges
  - [ ] 6.3 Ensure all seq values included for ordering

## Dev Notes

### RunWithGolden Implementation

```go
// internal/harness/golden.go
package harness

import (
    "testing"

    "github.com/sebdah/goldie/v2"
    "github.com/tyler/nysm/internal/ir"
)

// RunWithGolden executes a scenario and compares the trace against a golden file.
// The golden file is stored in testdata/golden/{scenario.Name}.golden
//
// To regenerate golden files, run:
//   go test ./internal/harness -update
func (h *Harness) RunWithGolden(t *testing.T, scenario *Scenario) error {
    t.Helper()

    // Run the scenario
    result, err := h.Run(scenario)
    if err != nil {
        return err
    }

    // Build trace snapshot
    snapshot := TraceSnapshot{
        ScenarioName:  scenario.Name,
        Invocations:   result.Invocations,
        Completions:   result.Completions,
        SyncFirings:   result.SyncFirings,
        ProvenanceEdges: result.ProvenanceEdges,
    }

    // Marshal to canonical JSON for deterministic comparison
    traceJSON, err := ir.MarshalCanonical(snapshot)
    if err != nil {
        return err
    }

    // Compare with golden file using goldie
    g := goldie.New(t,
        goldie.WithFixtureDir("testdata/golden"),
        goldie.WithNameSuffix(".golden"),
    )
    g.Assert(t, scenario.Name, traceJSON)

    return nil
}

// TraceSnapshot captures the complete trace for a scenario execution.
// All fields use canonical JSON serialization for deterministic comparison.
type TraceSnapshot struct {
    ScenarioName    string              `json:"scenario_name"`
    Invocations     []InvocationRecord  `json:"invocations"`
    Completions     []CompletionRecord  `json:"completions"`
    SyncFirings     []SyncFiringRecord  `json:"sync_firings"`
    ProvenanceEdges []ProvenanceEdge    `json:"provenance_edges"`
}

// InvocationRecord captures invocation data for golden comparison.
type InvocationRecord struct {
    ID              string           `json:"id"`
    FlowToken       string           `json:"flow_token"`
    ActionURI       string           `json:"action_uri"`
    Args            ir.IRObject      `json:"args"`
    Seq             int64            `json:"seq"`  // Logical clock
    SecurityContext ir.SecurityContext `json:"security_context"`
}

// CompletionRecord captures completion data for golden comparison.
type CompletionRecord struct {
    ID           string      `json:"id"`
    InvocationID string      `json:"invocation_id"`
    OutputCase   string      `json:"output_case"`
    Result       ir.IRObject `json:"result"`
    Seq          int64       `json:"seq"`  // Logical clock
}

// SyncFiringRecord captures sync firing data for golden comparison.
type SyncFiringRecord struct {
    ID           int64  `json:"id"`
    CompletionID string `json:"completion_id"`
    SyncID       string `json:"sync_id"`
    BindingHash  string `json:"binding_hash"`
    Seq          int64  `json:"seq"`  // Logical clock
}

// ProvenanceEdge links a sync firing to the invocation it produced.
type ProvenanceEdge struct {
    SyncFiringID int64  `json:"sync_firing_id"`
    InvocationID string `json:"invocation_id"`
}
```

### Golden File Format

Golden files are stored as canonical JSON with deterministic ordering:

```json
{
  "completions": [
    {
      "id": "completion-hash-1",
      "invocation_id": "invocation-hash-1",
      "output_case": "Success",
      "result": {
        "item_id": "widget",
        "new_quantity": 3
      },
      "seq": 2
    }
  ],
  "invocations": [
    {
      "action_uri": "nysm://myapp/Cart/addItem@1.0.0",
      "args": {
        "item_id": "widget",
        "quantity": 3
      },
      "flow_token": "test-flow-token-1",
      "id": "invocation-hash-1",
      "security_context": {
        "permissions": [],
        "tenant_id": "",
        "user_id": ""
      },
      "seq": 1
    }
  ],
  "provenance_edges": [
    {
      "invocation_id": "invocation-hash-2",
      "sync_firing_id": 1
    }
  ],
  "scenario_name": "cart_checkout_success",
  "sync_firings": [
    {
      "binding_hash": "binding-hash-1",
      "completion_id": "completion-hash-1",
      "id": 1,
      "seq": 3,
      "sync_id": "cart-inventory"
    }
  ]
}
```

**Key Features:**
- All keys sorted alphabetically (RFC 8785 UTF-16 ordering)
- No whitespace variations (compact output from MarshalCanonical)
- Logical clocks (seq) instead of timestamps
- Content-addressed IDs ensure determinism

### Using -update Flag

```bash
# Run tests normally - fails if golden file doesn't match
go test ./internal/harness

# Update all golden files when trace changes intentionally
go test ./internal/harness -update

# Update specific test
go test ./internal/harness -run TestCartCheckout -update
```

**Important:** Only use `-update` when you've intentionally changed behavior.
Golden files should be committed to git so reviewers can see trace changes.

### Deterministic Test Setup

To ensure golden files are stable:

```go
func TestCartCheckoutWithGolden(t *testing.T) {
    // Use deterministic test helpers
    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator("test-flow-token-1")

    // Create harness with deterministic dependencies
    h := NewHarness(
        WithClock(clock),
        WithFlowGenerator(flowGen),
    )

    // Load scenario
    scenario := LoadScenario(t, "testdata/scenarios/cart_checkout_success.yaml")

    // Run with golden comparison
    err := h.RunWithGolden(t, scenario)
    require.NoError(t, err)
}
```

### Test Examples

```go
// internal/harness/golden_test.go
package harness

import (
    "testing"

    "github.com/stretchr/testify/require"
    "github.com/tyler/nysm/internal/testutil"
)

func TestRunWithGolden_Match(t *testing.T) {
    // Setup deterministic harness
    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator("test-flow-1")

    h := NewHarness(
        WithClock(clock),
        WithFlowGenerator(flowGen),
    )

    // Load test scenario
    scenario := &Scenario{
        Name: "simple_invocation",
        Flow: []Step{
            {
                Invoke: "Cart.addItem",
                Args: map[string]any{
                    "item_id": "widget",
                    "quantity": 3,
                },
                Expect: ExpectedCompletion{
                    Case: "Success",
                    Result: map[string]any{
                        "item_id": "widget",
                        "new_quantity": 3,
                    },
                },
            },
        },
    }

    // Run with golden comparison - should pass
    err := h.RunWithGolden(t, scenario)
    require.NoError(t, err)
}

func TestRunWithGolden_Mismatch(t *testing.T) {
    // This test demonstrates what happens when trace doesn't match
    // Goldie will show a diff and fail the test

    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator("test-flow-1")

    h := NewHarness(
        WithClock(clock),
        WithFlowGenerator(flowGen),
    )

    scenario := &Scenario{
        Name: "mismatch_test",
        Flow: []Step{
            {
                Invoke: "Cart.addItem",
                Args: map[string]any{
                    "item_id": "different",  // Changed from golden
                    "quantity": 5,           // Changed from golden
                },
            },
        },
    }

    // This will fail with a readable diff showing the mismatch
    err := h.RunWithGolden(t, scenario)
    // In real scenarios, this would fail the test
    // We skip assertion here since this is a demo test
    _ = err
}

func TestRunWithGolden_Update(t *testing.T) {
    // When running with -update flag, this regenerates the golden file

    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator("test-flow-1")

    h := NewHarness(
        WithClock(clock),
        WithFlowGenerator(flowGen),
    )

    scenario := &Scenario{
        Name: "updated_trace",
        Flow: []Step{
            {
                Invoke: "Cart.checkout",
                Args: map[string]any{},
            },
        },
    }

    // Run: go test ./internal/harness -update
    // This will write the current trace to testdata/golden/updated_trace.golden
    err := h.RunWithGolden(t, scenario)
    require.NoError(t, err)
}

func TestCanonicalJSONDeterminism(t *testing.T) {
    // Verify that multiple runs produce identical golden files

    clock := testutil.NewDeterministicClock()
    flowGen := testutil.NewFixedFlowGenerator("test-flow-1")

    scenario := &Scenario{
        Name: "determinism_test",
        Flow: []Step{
            {
                Invoke: "Cart.addItem",
                Args: map[string]any{
                    "item_id": "widget",
                    "quantity": 3,
                },
            },
        },
    }

    // Run twice with identical setup
    h1 := NewHarness(WithClock(clock), WithFlowGenerator(flowGen))
    result1, err := h1.Run(scenario)
    require.NoError(t, err)

    h2 := NewHarness(WithClock(clock), WithFlowGenerator(flowGen))
    result2, err := h2.Run(scenario)
    require.NoError(t, err)

    // Both should produce identical canonical JSON
    json1, _ := ir.MarshalCanonical(TraceSnapshot{
        ScenarioName: scenario.Name,
        Invocations: result1.Invocations,
        Completions: result1.Completions,
    })

    json2, _ := ir.MarshalCanonical(TraceSnapshot{
        ScenarioName: scenario.Name,
        Invocations: result2.Invocations,
        Completions: result2.Completions,
    })

    require.Equal(t, json1, json2, "canonical JSON must be deterministic")
}
```

### File List

Files to create/modify:

1. `internal/harness/golden.go` - RunWithGolden implementation
2. `internal/harness/golden_test.go` - Golden file tests
3. `testdata/golden/` - Directory for golden files (create)
4. `testdata/golden/.gitkeep` - Ensure directory is committed
5. `go.mod` - Add `github.com/sebdah/goldie/v2 v2.8.0` dependency

### Relationship to Other Stories

- **Story 1-4 (RFC 8785):** Uses MarshalCanonical for deterministic JSON
- **Story 1-5 (Content-Addressed Identity):** IDs in traces are content-addressed
- **Story 2-2 (Logical Clocks):** All seq fields use logical time, not timestamps
- **Story 6-1 (Scenario Definition):** Scenarios are the input to RunWithGolden
- **Story 6-2 (Test Execution):** RunWithGolden wraps the Run() function
- **Story 6-5 (Operational Principles):** Golden traces validate principles

### Story Completion Checklist

- [ ] RunWithGolden function implemented in golden.go
- [ ] TraceSnapshot struct defined with all trace data
- [ ] Uses ir.MarshalCanonical for deterministic output
- [ ] Goldie configured with correct fixture directory
- [ ] -update flag works for regenerating golden files
- [ ] testdata/golden/ directory created and .gitkeep added
- [ ] Test for golden match passes
- [ ] Test for golden mismatch shows readable diff
- [ ] Test for determinism (multiple runs → identical JSON)
- [ ] Golden files use canonical JSON (no timestamp drift)
- [ ] All tests pass
- [ ] `go vet ./internal/harness/...` passes

### References

- [Source: docs/epics.md#Story 6.6] - Story definition
- [Source: docs/architecture.md#CP-2] - Logical clocks for determinism
- [Source: docs/architecture.md#CP-3] - RFC 8785 canonical JSON
- [Source: docs/prd.md#FR-6.4] - Golden trace snapshots requirement
- [Source: github.com/sebdah/goldie] - Golden file testing library

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow

### Completion Notes

- Golden files are the "source of truth" for expected trace behavior
- MarshalCanonical ensures golden files are stable across runs
- Always use deterministic clock and flow generator for golden tests
- Commit golden files to git so reviewers can see trace changes
- Only use -update flag when behavior change is intentional
- Diffs should clearly show what changed in the trace
