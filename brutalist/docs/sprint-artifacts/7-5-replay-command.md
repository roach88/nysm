# Story 7.5: Replay Command

Status: done

## Story

As a **developer debugging issues**,
I want **a replay command that re-executes the event log and verifies determinism**,
So that **I can validate crash recovery works and debug flow behavior**.

## Acceptance Criteria

1. **nysm replay command executes event log replay**
   ```bash
   nysm replay --db ./nysm.db
   ```
   - Replays all flows in the database
   - Reports statistics per flow
   - Verifies determinism (IDENTICAL results across replays)
   - Exits with code 0 if determinism verified, 1 if differences detected

2. **--flow flag replays specific flow only**
   ```bash
   nysm replay --db ./nysm.db --flow 019376f8-...
   ```
   - Replays single flow by flow token
   - Reports statistics for that flow only
   - Faster than full replay for debugging

3. **Replay output shows determinism verification**
   ```
   Replaying event log...
     Flow: 019376f8-4a2c-7b91-8e3f-123456789abc
       Invocations: 15
       Completions: 15
       Sync firings: 8
       Status: complete

     Flow: 019376f8-4a2c-7b91-8e3f-abcdefabcdef
       Invocations: 3
       Completions: 2
       Sync firings: 1
       Status: incomplete

   Replay complete
     Total flows: 2
     Complete: 1
     Incomplete: 1
     Result: IDENTICAL (determinism verified)
   ```

4. **Differences detected if replay produces different results**
   ```
   Replay complete
     Total flows: 5
     Result: DIFFERENCES DETECTED

   Differences:
     Flow 019376f8-...
       Expected sync firings: 8
       Actual sync firings: 9
       Difference: Extra firing for sync 'cart-inventory' with binding {...}

   ERROR: Replay produced different results
   ```

5. **--format json outputs structured results**
   ```bash
   nysm replay --db ./nysm.db --format json
   ```
   ```json
   {
     "status": "ok",
     "data": {
       "total_flows": 2,
       "complete_flows": 1,
       "incomplete_flows": 1,
       "determinism_verified": true,
       "flows": [
         {
           "flow_token": "019376f8-...",
           "invocations": 15,
           "completions": 15,
           "sync_firings": 8,
           "status": "complete"
         }
       ]
     }
   }
   ```

6. **Help text documents replay command**
   ```bash
   nysm replay --help
   ```
   Shows:
   - Usage: `nysm replay --db <path> [--flow <token>]`
   - Description: Replay event log and verify determinism
   - Flags: --db, --flow, --format, --verbose
   - Examples

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-5.3** | Support crash recovery and replay |
| **CLI Pattern** | Cobra commands in `internal/cli/` |
| **Output Pattern** | Human-readable default, JSON with --format json |
| **Error Codes** | E300-E399 for store errors |
| **Exit Codes** | 0 = success, 1 = error or differences detected |

## Tasks / Subtasks

- [ ] Task 1: Create replay command structure (AC: #1, #6)
  - [ ] 1.1 Create `internal/cli/replay.go`
  - [ ] 1.2 Define replayCmd with Cobra
  - [ ] 1.3 Add --db flag (required)
  - [ ] 1.4 Add --flow flag (optional)
  - [ ] 1.5 Add --format flag (text|json, default text)
  - [ ] 1.6 Add --verbose flag for detailed output
  - [ ] 1.7 Register replayCmd with root command

- [ ] Task 2: Implement replay logic (AC: #1, #2, #3)
  - [ ] 2.1 Open store from --db path
  - [ ] 2.2 If --flow specified, replay single flow
  - [ ] 2.3 If no --flow, detect all flows and replay each
  - [ ] 2.4 Use store.DetectIncompleteFlows() and store.ReplayFlow() from Story 2.7
  - [ ] 2.5 Collect statistics per flow
  - [ ] 2.6 Aggregate totals across all flows
  - [ ] 2.7 Verify determinism (replay twice, compare results)

- [ ] Task 3: Implement determinism verification (AC: #4)
  - [ ] 3.1 Replay each flow twice
  - [ ] 3.2 Compare ReplayResult from both runs
  - [ ] 3.3 Detect differences in invocation/completion/sync_firing counts
  - [ ] 3.4 Report specific differences if found
  - [ ] 3.5 Set exit code 1 if differences detected

- [ ] Task 4: Implement output formatting (AC: #3, #4, #5)
  - [ ] 4.1 Define ReplaySummary struct for aggregated results
  - [ ] 4.2 Implement human-readable text output
  - [ ] 4.3 Implement JSON output format
  - [ ] 4.4 Use output.go helper functions
  - [ ] 4.5 Show flow-by-flow progress if --verbose

- [ ] Task 5: Implement help and examples (AC: #6)
  - [ ] 5.1 Add command description
  - [ ] 5.2 Add flag descriptions
  - [ ] 5.3 Add usage examples
  - [ ] 5.4 Document determinism verification behavior

- [ ] Task 6: Write tests
  - [ ] 6.1 Create `internal/cli/replay_test.go`
  - [ ] 6.2 Test replay all flows
  - [ ] 6.3 Test replay single flow with --flow flag
  - [ ] 6.4 Test determinism verification (identical results)
  - [ ] 6.5 Test difference detection (simulate non-determinism)
  - [ ] 6.6 Test JSON output format
  - [ ] 6.7 Test error handling (missing db, invalid flow token)
  - [ ] 6.8 Test --verbose output

## Dev Notes

### Command Implementation Pattern

```go
// internal/cli/replay.go

package cli

import (
    "context"
    "fmt"
    "os"

    "github.com/spf13/cobra"
    "github.com/tyler/nysm/internal/store"
)

var replayCmd = &cobra.Command{
    Use:   "replay",
    Short: "Replay event log and verify determinism",
    Long: `Replay the event log from scratch and verify that determinism holds.

This command re-executes the event log and verifies that the same events
are produced. This validates crash recovery correctness.

If --flow is specified, only that flow is replayed. Otherwise, all flows
in the database are replayed.

Examples:
  # Replay all flows
  nysm replay --db ./nysm.db

  # Replay specific flow
  nysm replay --db ./nysm.db --flow 019376f8-...

  # Output JSON
  nysm replay --db ./nysm.db --format json

  # Verbose output
  nysm replay --db ./nysm.db --verbose
`,
    RunE: runReplay,
}

var (
    replayDBPath   string
    replayFlow     string
    replayFormat   string
    replayVerbose  bool
)

func init() {
    replayCmd.Flags().StringVar(&replayDBPath, "db", "", "Path to database file (required)")
    replayCmd.MarkFlagRequired("db")
    replayCmd.Flags().StringVar(&replayFlow, "flow", "", "Flow token to replay (optional, replays all if not specified)")
    replayCmd.Flags().StringVar(&replayFormat, "format", "text", "Output format: text or json")
    replayCmd.Flags().BoolVarP(&replayVerbose, "verbose", "v", false, "Verbose output")
}

func runReplay(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Open store
    s, err := store.Open(replayDBPath)
    if err != nil {
        return fmt.Errorf("open database: %w", err)
    }
    defer s.Close()

    // Execute replay
    var summary *ReplaySummary
    if replayFlow != "" {
        // Replay single flow
        summary, err = replaySingleFlow(ctx, s, replayFlow)
    } else {
        // Replay all flows
        summary, err = replayAllFlows(ctx, s)
    }
    if err != nil {
        return err
    }

    // Output results
    if replayFormat == "json" {
        return outputReplayJSON(summary)
    }
    return outputReplayText(summary)
}

type ReplaySummary struct {
    TotalFlows        int                `json:"total_flows"`
    CompleteFlows     int                `json:"complete_flows"`
    IncompleteFlows   int                `json:"incomplete_flows"`
    DeterminismVerified bool             `json:"determinism_verified"`
    Flows             []FlowReplayResult `json:"flows"`
    Differences       []string           `json:"differences,omitempty"`
}

type FlowReplayResult struct {
    FlowToken     string `json:"flow_token"`
    Invocations   int    `json:"invocations"`
    Completions   int    `json:"completions"`
    SyncFirings   int    `json:"sync_firings"`
    Status        string `json:"status"` // "complete" or "incomplete"
}

func replaySingleFlow(ctx context.Context, s *store.Store, flowToken string) (*ReplaySummary, error) {
    if replayVerbose {
        fmt.Fprintf(os.Stderr, "Replaying flow: %s\n", flowToken)
    }

    // Replay twice for determinism verification
    result1, err := s.ReplayFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("first replay: %w", err)
    }

    result2, err := s.ReplayFlow(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("second replay: %w", err)
    }

    // Compare results
    differences := compareReplayResults(result1, result2)

    summary := &ReplaySummary{
        TotalFlows:          1,
        DeterminismVerified: len(differences) == 0,
        Differences:         differences,
        Flows: []FlowReplayResult{
            {
                FlowToken:   result1.FlowToken,
                Invocations: result1.InvocationsFound,
                Completions: result1.CompletionsFound,
                SyncFirings: result1.SyncFiringsFound,
                Status:      string(result1.Status),
            },
        },
    }

    if result1.Status == store.FlowComplete {
        summary.CompleteFlows = 1
    } else {
        summary.IncompleteFlows = 1
    }

    return summary, nil
}

func replayAllFlows(ctx context.Context, s *store.Store) (*ReplaySummary, error) {
    if replayVerbose {
        fmt.Fprintln(os.Stderr, "Detecting all flows...")
    }

    // Get all flows (both complete and incomplete)
    allFlows, err := getAllFlowTokens(ctx, s)
    if err != nil {
        return nil, fmt.Errorf("get all flows: %w", err)
    }

    if replayVerbose {
        fmt.Fprintf(os.Stderr, "Found %d flows\n", len(allFlows))
    }

    summary := &ReplaySummary{
        TotalFlows:          len(allFlows),
        DeterminismVerified: true,
        Flows:               make([]FlowReplayResult, 0, len(allFlows)),
    }

    for i, flowToken := range allFlows {
        if replayVerbose {
            fmt.Fprintf(os.Stderr, "Replaying flow %d/%d: %s\n", i+1, len(allFlows), flowToken)
        }

        // Replay twice for determinism
        result1, err := s.ReplayFlow(ctx, flowToken)
        if err != nil {
            return nil, fmt.Errorf("replay flow %s (first): %w", flowToken, err)
        }

        result2, err := s.ReplayFlow(ctx, flowToken)
        if err != nil {
            return nil, fmt.Errorf("replay flow %s (second): %w", flowToken, err)
        }

        // Compare results
        differences := compareReplayResults(result1, result2)
        if len(differences) > 0 {
            summary.DeterminismVerified = false
            summary.Differences = append(summary.Differences, differences...)
        }

        // Add to summary
        summary.Flows = append(summary.Flows, FlowReplayResult{
            FlowToken:   result1.FlowToken,
            Invocations: result1.InvocationsFound,
            Completions: result1.CompletionsFound,
            SyncFirings: result1.SyncFiringsFound,
            Status:      string(result1.Status),
        })

        if result1.Status == store.FlowComplete {
            summary.CompleteFlows++
        } else {
            summary.IncompleteFlows++
        }
    }

    return summary, nil
}

func getAllFlowTokens(ctx context.Context, s *store.Store) ([]string, error) {
    // Query for all distinct flow tokens
    query := `
        SELECT DISTINCT flow_token
        FROM invocations
        ORDER BY MIN(seq) ASC
    `

    rows, err := s.Query(ctx, query)
    if err != nil {
        return nil, fmt.Errorf("query flow tokens: %w", err)
    }
    defer rows.Close()

    var flowTokens []string
    for rows.Next() {
        var flowToken string
        if err := rows.Scan(&flowToken); err != nil {
            return nil, fmt.Errorf("scan flow token: %w", err)
        }
        flowTokens = append(flowTokens, flowToken)
    }

    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("iterate rows: %w", err)
    }

    return flowTokens, nil
}

func compareReplayResults(r1, r2 *store.ReplayResult) []string {
    var differences []string

    if r1.FlowToken != r2.FlowToken {
        differences = append(differences, fmt.Sprintf("Flow tokens differ: %s vs %s", r1.FlowToken, r2.FlowToken))
    }

    if r1.InvocationsFound != r2.InvocationsFound {
        differences = append(differences, fmt.Sprintf("Flow %s: invocations differ: %d vs %d",
            r1.FlowToken, r1.InvocationsFound, r2.InvocationsFound))
    }

    if r1.CompletionsFound != r2.CompletionsFound {
        differences = append(differences, fmt.Sprintf("Flow %s: completions differ: %d vs %d",
            r1.FlowToken, r1.CompletionsFound, r2.CompletionsFound))
    }

    if r1.SyncFiringsFound != r2.SyncFiringsFound {
        differences = append(differences, fmt.Sprintf("Flow %s: sync firings differ: %d vs %d",
            r1.FlowToken, r1.SyncFiringsFound, r2.SyncFiringsFound))
    }

    if r1.LastSeq != r2.LastSeq {
        differences = append(differences, fmt.Sprintf("Flow %s: last seq differs: %d vs %d",
            r1.FlowToken, r1.LastSeq, r2.LastSeq))
    }

    if r1.Status != r2.Status {
        differences = append(differences, fmt.Sprintf("Flow %s: status differs: %s vs %s",
            r1.FlowToken, r1.Status, r2.Status))
    }

    return differences
}

func outputReplayText(summary *ReplaySummary) error {
    fmt.Println("Replaying event log...")

    // Print per-flow results
    for _, flow := range summary.Flows {
        fmt.Printf("  Flow: %s\n", flow.FlowToken)
        fmt.Printf("    Invocations: %d\n", flow.Invocations)
        fmt.Printf("    Completions: %d\n", flow.Completions)
        fmt.Printf("    Sync firings: %d\n", flow.SyncFirings)
        fmt.Printf("    Status: %s\n", flow.Status)
        fmt.Println()
    }

    // Print summary
    fmt.Println("Replay complete")
    fmt.Printf("  Total flows: %d\n", summary.TotalFlows)
    fmt.Printf("  Complete: %d\n", summary.CompleteFlows)
    fmt.Printf("  Incomplete: %d\n", summary.IncompleteFlows)

    if summary.DeterminismVerified {
        fmt.Println("  Result: IDENTICAL (determinism verified)")
        return nil
    }

    fmt.Println("  Result: DIFFERENCES DETECTED")
    fmt.Println()
    fmt.Println("Differences:")
    for _, diff := range summary.Differences {
        fmt.Printf("  %s\n", diff)
    }
    fmt.Println()
    fmt.Println("ERROR: Replay produced different results")

    // Exit with code 1 for differences
    os.Exit(1)
    return nil
}

func outputReplayJSON(summary *ReplaySummary) error {
    status := "ok"
    var errorMsg *string

    if !summary.DeterminismVerified {
        status = "error"
        msg := "Replay produced different results"
        errorMsg = &msg
    }

    response := struct {
        Status string         `json:"status"`
        Data   *ReplaySummary `json:"data,omitempty"`
        Error  *string        `json:"error,omitempty"`
    }{
        Status: status,
        Data:   summary,
        Error:  errorMsg,
    }

    encoder := json.NewEncoder(os.Stdout)
    encoder.SetIndent("", "  ")
    if err := encoder.Encode(response); err != nil {
        return fmt.Errorf("encode JSON: %w", err)
    }

    if !summary.DeterminismVerified {
        os.Exit(1)
    }

    return nil
}
```

### Test Examples

**Test: Replay All Flows**
```go
func TestReplayCommand_AllFlows(t *testing.T) {
    // Setup: Create test database with 2 flows
    dbPath := setupTestDatabase(t, []testFlow{
        {
            token:       "flow-1",
            invocations: 3,
            completions: 3,
            syncFirings: 1,
            status:      "complete",
        },
        {
            token:       "flow-2",
            invocations: 5,
            completions: 4,
            syncFirings: 2,
            status:      "incomplete",
        },
    })

    // Execute command
    output, exitCode := runCommand("replay", "--db", dbPath)

    // Assertions
    assert.Equal(t, 0, exitCode, "should exit with code 0")
    assert.Contains(t, output, "Total flows: 2")
    assert.Contains(t, output, "Complete: 1")
    assert.Contains(t, output, "Incomplete: 1")
    assert.Contains(t, output, "IDENTICAL (determinism verified)")
}
```

**Test: Replay Single Flow**
```go
func TestReplayCommand_SingleFlow(t *testing.T) {
    dbPath := setupTestDatabase(t, []testFlow{
        {token: "flow-1", invocations: 3, completions: 3, syncFirings: 1},
        {token: "flow-2", invocations: 5, completions: 4, syncFirings: 2},
    })

    // Execute with --flow flag
    output, exitCode := runCommand("replay", "--db", dbPath, "--flow", "flow-1")

    // Assertions
    assert.Equal(t, 0, exitCode)
    assert.Contains(t, output, "Flow: flow-1")
    assert.Contains(t, output, "Invocations: 3")
    assert.NotContains(t, output, "flow-2", "should only replay specified flow")
}
```

**Test: Determinism Verification**
```go
func TestReplayCommand_DeterminismVerified(t *testing.T) {
    // Setup: Create database with deterministic flow
    dbPath := setupDeterministicFlow(t)

    // Execute replay twice
    output1, exitCode1 := runCommand("replay", "--db", dbPath)
    output2, exitCode2 := runCommand("replay", "--db", dbPath)

    // Both runs should produce identical output
    assert.Equal(t, output1, output2, "replay output should be identical")
    assert.Equal(t, 0, exitCode1)
    assert.Equal(t, 0, exitCode2)
    assert.Contains(t, output1, "IDENTICAL (determinism verified)")
}
```

**Test: Difference Detection**
```go
func TestReplayCommand_DetectsDifferences(t *testing.T) {
    // Setup: Create database with non-deterministic flow (simulate bug)
    // This is a contrived test - real NYSM should NEVER have differences
    dbPath := setupNonDeterministicFlow(t)

    // Execute replay
    output, exitCode := runCommand("replay", "--db", dbPath)

    // Assertions
    assert.Equal(t, 1, exitCode, "should exit with code 1 for differences")
    assert.Contains(t, output, "DIFFERENCES DETECTED")
    assert.Contains(t, output, "ERROR: Replay produced different results")
}
```

**Test: JSON Output**
```go
func TestReplayCommand_JSONOutput(t *testing.T) {
    dbPath := setupTestDatabase(t, []testFlow{
        {token: "flow-1", invocations: 3, completions: 3, syncFirings: 1, status: "complete"},
    })

    // Execute with --format json
    output, exitCode := runCommand("replay", "--db", dbPath, "--format", "json")

    // Parse JSON
    var response struct {
        Status string `json:"status"`
        Data   struct {
            TotalFlows          int  `json:"total_flows"`
            DeterminismVerified bool `json:"determinism_verified"`
        } `json:"data"`
    }
    err := json.Unmarshal([]byte(output), &response)
    require.NoError(t, err)

    // Assertions
    assert.Equal(t, 0, exitCode)
    assert.Equal(t, "ok", response.Status)
    assert.Equal(t, 1, response.Data.TotalFlows)
    assert.True(t, response.Data.DeterminismVerified)
}
```

**Test: Verbose Output**
```go
func TestReplayCommand_VerboseOutput(t *testing.T) {
    dbPath := setupTestDatabase(t, []testFlow{
        {token: "flow-1", invocations: 3, completions: 3},
        {token: "flow-2", invocations: 5, completions: 4},
    })

    // Execute with --verbose
    output, exitCode := runCommand("replay", "--db", dbPath, "--verbose")

    // Assertions
    assert.Equal(t, 0, exitCode)
    assert.Contains(t, output, "Detecting all flows...")
    assert.Contains(t, output, "Found 2 flows")
    assert.Contains(t, output, "Replaying flow 1/2")
    assert.Contains(t, output, "Replaying flow 2/2")
}
```

**Test: Error Handling - Missing Database**
```go
func TestReplayCommand_MissingDatabase(t *testing.T) {
    output, exitCode := runCommand("replay", "--db", "/nonexistent/nysm.db")

    assert.Equal(t, 1, exitCode)
    assert.Contains(t, output, "open database")
}
```

**Test: Error Handling - Invalid Flow Token**
```go
func TestReplayCommand_InvalidFlowToken(t *testing.T) {
    dbPath := setupTestDatabase(t, []testFlow{
        {token: "flow-1", invocations: 3, completions: 3},
    })

    output, exitCode := runCommand("replay", "--db", dbPath, "--flow", "nonexistent-flow")

    // Should succeed but report 0 flows found
    assert.Equal(t, 0, exitCode)
    assert.Contains(t, output, "Total flows: 1")
    assert.Contains(t, output, "Invocations: 0") // No invocations for nonexistent flow
}
```

### File List

Files to create:

1. `internal/cli/replay.go` - Replay command implementation
2. `internal/cli/replay_test.go` - Command tests

Files to reference (must exist):

1. `internal/store/store.go` - Store.Open, Store.ReplayFlow
2. `internal/store/replay.go` - DetectIncompleteFlows, ReplayFlow (from Story 2.7)
3. `internal/cli/output.go` - Output formatting helpers
4. `cmd/nysm/main.go` - Root command registration

### Story Completion Checklist

- [ ] replayCmd defined with Cobra
- [ ] --db flag implemented (required)
- [ ] --flow flag implemented (optional)
- [ ] --format flag implemented (text|json)
- [ ] --verbose flag implemented
- [ ] Command registered with root command
- [ ] runReplay function executes replay logic
- [ ] replaySingleFlow handles --flow case
- [ ] replayAllFlows handles all-flows case
- [ ] getAllFlowTokens queries distinct flow tokens
- [ ] Determinism verification (replay twice, compare)
- [ ] compareReplayResults detects differences
- [ ] outputReplayText formats human-readable output
- [ ] outputReplayJSON formats JSON output
- [ ] Help text documents command usage
- [ ] Exit code 0 for success, 1 for differences
- [ ] Tests verify replay all flows
- [ ] Tests verify replay single flow
- [ ] Tests verify determinism verification
- [ ] Tests verify difference detection
- [ ] Tests verify JSON output
- [ ] Tests verify verbose output
- [ ] Tests verify error handling
- [ ] `go test ./internal/cli/...` passes
- [ ] `go vet ./internal/cli/...` passes

### Relationship to Other Stories

**Dependencies:**
- Story 2.7 (Crash Recovery and Replay) - Required for DetectIncompleteFlows and ReplayFlow
- Story 7.1 (CLI Framework Setup) - Required for Cobra and root command
- Story 7.2 (Compile Command) - Reference for CLI patterns
- Story 2.1 (SQLite Store) - Required for database access

**Enables:**
- User-facing replay functionality for debugging
- Determinism verification in production environments
- CI/CD validation of replay correctness
- Developer confidence in crash recovery

**Note:** This is Story 7.5 in Epic 7 (CLI & Demo Application). It provides user-facing access to the crash recovery functionality built in Story 2.7.

### References

- [Source: docs/prd.md#FR-5.3] - Support crash recovery and replay
- [Source: docs/epics.md#Story 7.5] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CLI Commands] - CLI command structure
- [Source: Story 2.7] - DetectIncompleteFlows and ReplayFlow implementation
- [Source: Story 7.1] - CLI framework and Cobra setup
- [Source: Story 7.2] - CLI patterns and output formatting

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation for replay command

### Completion Notes

- Replay command provides user-facing access to Story 2.7 crash recovery functionality
- Determinism verification runs replay twice and compares results (CRITICAL for validating correctness)
- --flow flag enables targeted debugging of specific flows
- Exit code 1 for differences detected enables CI/CD validation
- JSON output format enables programmatic consumption (scripts, monitoring)
- Verbose output shows progress for long-running replays
- getAllFlowTokens queries all flows (not just incomplete) for comprehensive verification
- compareReplayResults checks all fields: invocations, completions, sync_firings, last seq, status
- Human-readable output is default (matches NYSM CLI convention)
- Error handling covers missing database, invalid flow tokens
- Tests verify determinism verification works correctly
- This completes user-facing replay functionality - developers can now validate crash recovery works
