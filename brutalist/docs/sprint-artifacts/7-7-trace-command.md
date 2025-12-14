# Story 7.7: Trace Command

Status: done

## Story

As a **developer debugging flows**,
I want **a trace command that shows provenance**,
So that **I can understand "why did this happen?"**.

## Acceptance Criteria

1. **CLI trace command with --db and --flow flags (AC: #1)**
   - Command: `nysm trace --db <path> --flow <token>`
   - `--db` specifies SQLite database path (required)
   - `--flow` specifies flow token to trace (required)
   - Reads all invocations/completions/provenance for the flow
   - Displays timeline view by default (human-readable)
   - Exits with code 0 on success, 1 on errors

2. **Timeline output showing invocations and completions (AC: #2)**
   - Format: `[seq] ActionName event_type`
   - Event types: "invoked", "completed (OutputCase)"
   - Shows args for invocations
   - Shows result for completions
   - Indicates sync-triggered invocations: "triggered by: sync <name> from [seq]"
   - Ordered by seq ASC (chronological)

3. **Provenance section showing causality (AC: #3)**
   - Header: "Provenance:"
   - For each sync-triggered invocation:
     - `[inv_seq] ← [comp_seq] via <sync_id> (binding: <fields>)`
   - Shows binding values that caused the trigger
   - Ordered by invocation seq
   - Empty section if no provenance (user-initiated only)

4. **--format json flag for structured output (AC: #4)**
   - JSON structure with flow_token, timeline array, provenance array
   - Timeline: [{seq, event_type, action_uri, args, result, output_case}]
   - Provenance: [{invocation_seq, completion_seq, sync_id, binding_hash, bindings}]
   - Canonical JSON format for consistency
   - Enables programmatic parsing and debugging

5. **--action filter for specific actions (AC: #5)**
   - `--action <pattern>` filters timeline to matching actions
   - Pattern matches action_uri (e.g., "Cart.checkout", "Inventory.*")
   - Glob-style matching: `*` for any, exact match otherwise
   - Provenance still shows full causality chain
   - Useful for large flows with many actions

6. **Error handling for missing/invalid flows (AC: #6)**
   - Clear error if flow token not found: "flow <token> not found"
   - Clear error if database doesn't exist: "database not found: <path>"
   - Clear error if database schema invalid: "invalid database schema"
   - All errors exit with code 1
   - JSON format includes error in `{status: "error", error: {...}}` structure

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-4.1** | Provenance edges enable "why did this happen?" queries |
| **NFR-1.3** | Full trace reconstruction from provenance |
| **NFR-4.2** | Sync rule evaluation traceable via debug output |
| **CP-4** | Query ordering must be deterministic (seq ASC, id ASC) |

## Tasks / Subtasks

- [ ] Task 1: Implement trace command structure (AC: #1)
  - [ ] 1.1 Create `internal/cli/trace.go`
  - [ ] 1.2 Add `traceCmd` to Cobra CLI root
  - [ ] 1.3 Add `--db` flag (required)
  - [ ] 1.4 Add `--flow` flag (required)
  - [ ] 1.5 Add `--format` flag (default: "text", options: "text"|"json")
  - [ ] 1.6 Add `--action` flag (optional filter)
  - [ ] 1.7 Validate required flags present
  - [ ] 1.8 Handle help text and examples

- [ ] Task 2: Implement data loading (AC: #1, #6)
  - [ ] 2.1 Open database at --db path
  - [ ] 2.2 Handle database not found error
  - [ ] 2.3 Query invocations for flow token
  - [ ] 2.4 Query completions for flow token
  - [ ] 2.5 Query provenance edges for all invocations in flow
  - [ ] 2.6 Handle flow not found (empty results)
  - [ ] 2.7 Close database after use

- [ ] Task 3: Build timeline view (AC: #2)
  - [ ] 3.1 Merge invocations and completions by seq
  - [ ] 3.2 Sort events by seq ASC, id ASC (deterministic)
  - [ ] 3.3 Format invocation events: `[seq] ActionName invoked`
  - [ ] 3.4 Format completion events: `[seq] ActionName completed (OutputCase)`
  - [ ] 3.5 Show args for invocations (formatted JSON or key=value)
  - [ ] 3.6 Show result for completions (formatted JSON or key=value)
  - [ ] 3.7 Annotate sync-triggered invocations with provenance

- [ ] Task 4: Build provenance view (AC: #3)
  - [ ] 4.1 Query ReadProvenance for each invocation in flow
  - [ ] 4.2 Format: `[inv_seq] ← [comp_seq] via <sync_id> (binding: {fields})`
  - [ ] 4.3 Extract binding fields from binding_hash (or query sync_firings)
  - [ ] 4.4 Order by invocation seq
  - [ ] 4.5 Show empty section if no provenance
  - [ ] 4.6 Handle multi-binding cases (multiple edges for same completion)

- [ ] Task 5: Implement text output format (AC: #2, #3)
  - [ ] 5.1 Print header: `Flow: <token>`
  - [ ] 5.2 Print timeline section
  - [ ] 5.3 Print provenance section
  - [ ] 5.4 Use color/formatting for readability (optional)
  - [ ] 5.5 Truncate long values with ellipsis
  - [ ] 5.6 Handle terminal width gracefully

- [ ] Task 6: Implement JSON output format (AC: #4)
  - [ ] 6.1 Create TraceOutput struct with flow_token, timeline, provenance
  - [ ] 6.2 Serialize to canonical JSON
  - [ ] 6.3 Print JSON to stdout
  - [ ] 6.4 Include error field if query failed
  - [ ] 6.5 Validate JSON structure with test

- [ ] Task 7: Implement --action filter (AC: #5)
  - [ ] 7.1 Parse --action pattern
  - [ ] 7.2 Filter timeline events by action_uri
  - [ ] 7.3 Use glob-style matching (exact or `*` wildcard)
  - [ ] 7.4 Keep full provenance (don't filter causality)
  - [ ] 7.5 Show message if filter excludes all events

- [ ] Task 8: Write comprehensive tests (AC: #1-6)
  - [ ] 8.1 Create `internal/cli/trace_test.go`
  - [ ] 8.2 Test normal flow trace (text format)
  - [ ] 8.3 Test JSON format output
  - [ ] 8.4 Test --action filter
  - [ ] 8.5 Test flow not found error
  - [ ] 8.6 Test database not found error
  - [ ] 8.7 Test multi-binding provenance display
  - [ ] 8.8 Test user-initiated flow (no provenance)
  - [ ] 8.9 Test timeline ordering determinism

- [ ] Task 9: Add example output to documentation (AC: #2, #3)
  - [ ] 9.1 Document text format example
  - [ ] 9.2 Document JSON format example
  - [ ] 9.3 Document --action filter example
  - [ ] 9.4 Add to README or CLI help

## Dev Notes

### Critical Implementation Details

**CLI Command Definition**
```go
// internal/cli/trace.go

package cli

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "strings"

    "github.com/spf13/cobra"
    "github.com/tyler/nysm/internal/ir"
    "github.com/tyler/nysm/internal/store"
)

var traceCmd = &cobra.Command{
    Use:   "trace",
    Short: "Query provenance for a flow",
    Long: `Query provenance for a flow and show causality chain.

Displays a timeline of invocations and completions for the specified flow,
along with provenance edges showing "why did this happen?"

Examples:
  # Show trace for a flow
  nysm trace --db ./nysm.db --flow 019376f8-...

  # JSON output for programmatic use
  nysm trace --db ./nysm.db --flow 019376f8-... --format json

  # Filter to specific action
  nysm trace --db ./nysm.db --flow 019376f8-... --action "Cart.checkout"

  # Filter with wildcard
  nysm trace --db ./nysm.db --flow 019376f8-... --action "Inventory.*"
`,
    RunE: runTrace,
}

var (
    traceDBPath     string
    traceFlowToken  string
    traceFormat     string
    traceActionFilter string
)

func init() {
    traceCmd.Flags().StringVar(&traceDBPath, "db", "", "Database path (required)")
    traceCmd.Flags().StringVar(&traceFlowToken, "flow", "", "Flow token to trace (required)")
    traceCmd.Flags().StringVar(&traceFormat, "format", "text", "Output format: text|json")
    traceCmd.Flags().StringVar(&traceActionFilter, "action", "", "Filter to specific action (glob pattern)")

    traceCmd.MarkFlagRequired("db")
    traceCmd.MarkFlagRequired("flow")
}

func runTrace(cmd *cobra.Command, args []string) error {
    ctx := context.Background()

    // Validate format
    if traceFormat != "text" && traceFormat != "json" {
        return fmt.Errorf("invalid format: %s (must be text or json)", traceFormat)
    }

    // Open database
    db, err := store.Open(traceDBPath)
    if err != nil {
        if os.IsNotExist(err) {
            return fmt.Errorf("database not found: %s", traceDBPath)
        }
        return fmt.Errorf("open database: %w", err)
    }
    defer db.Close()

    // Query flow data
    trace, err := queryFlowTrace(ctx, db, traceFlowToken)
    if err != nil {
        return fmt.Errorf("query flow trace: %w", err)
    }

    // Check if flow found
    if len(trace.Timeline) == 0 {
        return fmt.Errorf("flow %s not found", traceFlowToken)
    }

    // Apply action filter if specified
    if traceActionFilter != "" {
        trace = filterByAction(trace, traceActionFilter)
        if len(trace.Timeline) == 0 {
            fmt.Fprintf(os.Stderr, "Warning: filter excluded all events\n")
        }
    }

    // Output in requested format
    switch traceFormat {
    case "text":
        printTextTrace(os.Stdout, trace)
    case "json":
        printJSONTrace(os.Stdout, trace)
    }

    return nil
}
```

**Data Structures**
```go
// FlowTrace represents complete trace for a flow
type FlowTrace struct {
    FlowToken  string          `json:"flow_token"`
    Timeline   []TimelineEvent `json:"timeline"`
    Provenance []ProvenanceRecord `json:"provenance"`
}

// TimelineEvent is a union type for invocations and completions
type TimelineEvent struct {
    Seq        int64             `json:"seq"`
    EventType  string            `json:"event_type"` // "invoked" | "completed"
    ActionURI  string            `json:"action_uri"`
    Args       ir.IRObject       `json:"args,omitempty"`
    Result     ir.IRObject       `json:"result,omitempty"`
    OutputCase string            `json:"output_case,omitempty"`
    TriggeredBy *TriggerInfo     `json:"triggered_by,omitempty"`
}

// TriggerInfo describes provenance for sync-triggered invocations
type TriggerInfo struct {
    CompletionSeq int64  `json:"completion_seq"`
    SyncID        string `json:"sync_id"`
    BindingHash   string `json:"binding_hash"`
}

// ProvenanceRecord shows causality: invocation ← completion via sync
type ProvenanceRecord struct {
    InvocationSeq int64       `json:"invocation_seq"`
    CompletionSeq int64       `json:"completion_seq"`
    SyncID        string      `json:"sync_id"`
    BindingHash   string      `json:"binding_hash"`
    Bindings      ir.IRObject `json:"bindings,omitempty"`
}
```

**Query Flow Trace**
```go
func queryFlowTrace(ctx context.Context, db *store.Store, flowToken string) (*FlowTrace, error) {
    // Read all invocations for flow
    invocations, err := db.ReadFlowInvocations(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("read invocations: %w", err)
    }

    // Read all completions for flow
    completions, err := db.ReadFlowCompletions(ctx, flowToken)
    if err != nil {
        return nil, fmt.Errorf("read completions: %w", err)
    }

    // Build invocation ID → seq map for provenance lookup
    invSeqMap := make(map[string]int64)
    for _, inv := range invocations {
        invSeqMap[inv.ID] = inv.Seq
    }

    // Build completion ID → seq map
    compSeqMap := make(map[string]int64)
    for _, comp := range completions {
        compSeqMap[comp.ID] = comp.Seq
    }

    // Read provenance for all invocations
    provenanceMap := make(map[string]*store.ProvenanceRecord) // invocation_id → provenance
    var provenanceRecords []ProvenanceRecord

    for _, inv := range invocations {
        provs, err := db.ReadProvenance(ctx, inv.ID)
        if err != nil {
            return nil, fmt.Errorf("read provenance for %s: %w", inv.ID, err)
        }
        if len(provs) > 0 {
            // Store first provenance (should be only one per invocation)
            prov := provs[0]
            provenanceMap[inv.ID] = &prov

            // Add to provenance records list
            provenanceRecords = append(provenanceRecords, ProvenanceRecord{
                InvocationSeq: inv.Seq,
                CompletionSeq: compSeqMap[prov.CompletionID],
                SyncID:        prov.SyncID,
                BindingHash:   prov.BindingHash,
                // Bindings: would need to query sync_firings to get actual binding values
            })
        }
    }

    // Merge invocations and completions into timeline
    timeline := mergeTimeline(invocations, completions, provenanceMap, compSeqMap)

    return &FlowTrace{
        FlowToken:  flowToken,
        Timeline:   timeline,
        Provenance: provenanceRecords,
    }, nil
}

func mergeTimeline(
    invocations []ir.Invocation,
    completions []ir.Completion,
    provenanceMap map[string]*store.ProvenanceRecord,
    compSeqMap map[string]int64,
) []TimelineEvent {
    var events []TimelineEvent

    // Add invocations
    for _, inv := range invocations {
        event := TimelineEvent{
            Seq:       inv.Seq,
            EventType: "invoked",
            ActionURI: string(inv.ActionURI),
            Args:      inv.Args,
        }

        // Check if sync-triggered
        if prov, ok := provenanceMap[inv.ID]; ok {
            event.TriggeredBy = &TriggerInfo{
                CompletionSeq: compSeqMap[prov.CompletionID],
                SyncID:        prov.SyncID,
                BindingHash:   prov.BindingHash,
            }
        }

        events = append(events, event)
    }

    // Add completions
    for _, comp := range completions {
        events = append(events, TimelineEvent{
            Seq:        comp.Seq,
            EventType:  "completed",
            ActionURI:  string(comp.ActionURI),
            Result:     comp.Result,
            OutputCase: comp.OutputCase,
        })
    }

    // Sort by seq, then by event type (invoked before completed for same seq)
    // Actually, seq should be unique across all events if logical clock is correct
    sort.Slice(events, func(i, j int) bool {
        if events[i].Seq != events[j].Seq {
            return events[i].Seq < events[j].Seq
        }
        // Tiebreaker: invoked before completed
        return events[i].EventType < events[j].EventType
    })

    return events
}
```

**Text Output Format**
```go
func printTextTrace(w io.Writer, trace *FlowTrace) {
    fmt.Fprintf(w, "Flow: %s\n\n", trace.FlowToken)

    // Timeline section
    fmt.Fprintf(w, "Timeline:\n")
    for _, event := range trace.Timeline {
        switch event.EventType {
        case "invoked":
            fmt.Fprintf(w, "  [%d] %s invoked\n", event.Seq, actionName(event.ActionURI))
            if len(event.Args) > 0 {
                fmt.Fprintf(w, "      args: %s\n", formatValues(event.Args))
            }
            if event.TriggeredBy != nil {
                fmt.Fprintf(w, "      triggered by: sync %q from [%d]\n",
                    event.TriggeredBy.SyncID, event.TriggeredBy.CompletionSeq)
            }

        case "completed":
            fmt.Fprintf(w, "  [%d] %s completed (%s)\n",
                event.Seq, actionName(event.ActionURI), event.OutputCase)
            if len(event.Result) > 0 {
                fmt.Fprintf(w, "      result: %s\n", formatValues(event.Result))
            }
        }
    }

    // Provenance section
    fmt.Fprintf(w, "\nProvenance:\n")
    if len(trace.Provenance) == 0 {
        fmt.Fprintf(w, "  (none - all invocations user-initiated)\n")
    } else {
        for _, prov := range trace.Provenance {
            fmt.Fprintf(w, "  [%d] ← [%d] via %s (binding: %s)\n",
                prov.InvocationSeq, prov.CompletionSeq, prov.SyncID, prov.BindingHash[:8])
        }
    }
}

// actionName extracts the action name from URI (e.g., "Cart.checkout" from "nysm://app/action/Cart/checkout")
func actionName(uri string) string {
    parts := strings.Split(uri, "/")
    if len(parts) >= 2 {
        return parts[len(parts)-2] + "." + parts[len(parts)-1]
    }
    return uri
}

// formatValues formats IR values for human reading
func formatValues(obj ir.IRObject) string {
    parts := []string{}
    for k, v := range obj {
        parts = append(parts, fmt.Sprintf("%s: %v", k, formatValue(v)))
    }
    return "{" + strings.Join(parts, ", ") + "}"
}

func formatValue(v ir.IRValue) string {
    switch val := v.(type) {
    case ir.IRString:
        return fmt.Sprintf("%q", string(val))
    case ir.IRInt:
        return fmt.Sprintf("%d", int64(val))
    case ir.IRBool:
        return fmt.Sprintf("%t", bool(val))
    case ir.IRArray:
        return "[...]" // Truncate arrays for readability
    case ir.IRObject:
        return "{...}" // Truncate nested objects
    default:
        return fmt.Sprintf("%v", v)
    }
}
```

**JSON Output Format**
```go
func printJSONTrace(w io.Writer, trace *FlowTrace) {
    encoder := json.NewEncoder(w)
    encoder.SetIndent("", "  ")
    encoder.Encode(trace)
}
```

**Action Filter Implementation**
```go
func filterByAction(trace *FlowTrace, pattern string) *FlowTrace {
    filtered := &FlowTrace{
        FlowToken:  trace.FlowToken,
        Timeline:   []TimelineEvent{},
        Provenance: trace.Provenance, // Keep full provenance
    }

    for _, event := range trace.Timeline {
        if matchesAction(event.ActionURI, pattern) {
            filtered.Timeline = append(filtered.Timeline, event)
        }
    }

    return filtered
}

func matchesAction(actionURI, pattern string) bool {
    // Extract action name from URI
    name := actionName(actionURI)

    // Exact match
    if name == pattern {
        return true
    }

    // Glob-style wildcard matching
    if strings.Contains(pattern, "*") {
        // Simple implementation: * matches any substring
        prefix := strings.Split(pattern, "*")[0]
        return strings.HasPrefix(name, prefix)
    }

    return false
}
```

### Example Output

**Text Format (Normal Flow)**
```bash
$ nysm trace --db ./nysm.db --flow 019376f8-...

Flow: 019376f8-...

Timeline:
  [1] Cart.addItem invoked
      args: {item_id: "widget", quantity: 3}
  [2] Cart.addItem completed (Success)
      result: {item_id: "widget", new_quantity: 3}
  [3] Cart.checkout invoked
  [4] Cart.checkout completed (Success)
  [5] Inventory.reserve invoked
      triggered by: sync "cart-inventory" from [4]
      args: {item_id: "widget", quantity: 3}
  [6] Inventory.reserve completed (Success)

Provenance:
  [5] ← [4] via cart-inventory (binding: abc123de)
```

**JSON Format**
```bash
$ nysm trace --db ./nysm.db --flow 019376f8-... --format json

{
  "flow_token": "019376f8-...",
  "timeline": [
    {
      "seq": 1,
      "event_type": "invoked",
      "action_uri": "nysm://demo/action/Cart/addItem",
      "args": {
        "item_id": "widget",
        "quantity": 3
      }
    },
    {
      "seq": 2,
      "event_type": "completed",
      "action_uri": "nysm://demo/action/Cart/addItem",
      "output_case": "Success",
      "result": {
        "item_id": "widget",
        "new_quantity": 3
      }
    },
    {
      "seq": 3,
      "event_type": "invoked",
      "action_uri": "nysm://demo/action/Cart/checkout"
    },
    {
      "seq": 4,
      "event_type": "completed",
      "action_uri": "nysm://demo/action/Cart/checkout",
      "output_case": "Success"
    },
    {
      "seq": 5,
      "event_type": "invoked",
      "action_uri": "nysm://demo/action/Inventory/reserve",
      "args": {
        "item_id": "widget",
        "quantity": 3
      },
      "triggered_by": {
        "completion_seq": 4,
        "sync_id": "cart-inventory",
        "binding_hash": "abc123de..."
      }
    },
    {
      "seq": 6,
      "event_type": "completed",
      "action_uri": "nysm://demo/action/Inventory/reserve",
      "output_case": "Success"
    }
  ],
  "provenance": [
    {
      "invocation_seq": 5,
      "completion_seq": 4,
      "sync_id": "cart-inventory",
      "binding_hash": "abc123de..."
    }
  ]
}
```

**Action Filter**
```bash
$ nysm trace --db ./nysm.db --flow 019376f8-... --action "Inventory.*"

Flow: 019376f8-...

Timeline:
  [5] Inventory.reserve invoked
      triggered by: sync "cart-inventory" from [4]
      args: {item_id: "widget", quantity: 3}
  [6] Inventory.reserve completed (Success)

Provenance:
  [5] ← [4] via cart-inventory (binding: abc123de)
```

**User-Initiated Flow (No Provenance)**
```bash
$ nysm trace --db ./nysm.db --flow 019376f8-user-flow

Flow: 019376f8-user-flow

Timeline:
  [1] Order.Create invoked
      args: {product: "widget", qty: 5}
  [2] Order.Create completed (Success)
      result: {order_id: "ord-123"}

Provenance:
  (none - all invocations user-initiated)
```

**Flow Not Found**
```bash
$ nysm trace --db ./nysm.db --flow nonexistent

Error: flow nonexistent not found
```

### Test Examples

**Test Normal Flow Trace**
```go
func TestTraceCommand_NormalFlow(t *testing.T) {
    // Setup: create test database with flow
    db := setupTestDB(t)
    flowToken := "flow-test-123"

    // Create invocation
    inv1 := testInvocation("Cart.addItem", 1, flowToken)
    db.WriteInvocation(context.Background(), inv1)

    // Create completion
    comp1 := testCompletion(inv1.ID, "Success", 2)
    db.WriteCompletion(context.Background(), comp1)

    // Create sync-triggered invocation
    inv2 := testInvocation("Inventory.reserve", 3, flowToken)
    db.WriteInvocation(context.Background(), inv2)

    // Create sync firing
    firing := ir.SyncFiring{
        CompletionID: comp1.ID,
        SyncID: "cart-inventory",
        BindingHash: "hash123",
        Seq: 3,
    }
    result, _ := db.WriteSyncFiring(context.Background(), firing)
    firing.ID = result.LastInsertId()

    // Create provenance edge
    edge := ir.ProvenanceEdge{
        SyncFiringID: firing.ID,
        InvocationID: inv2.ID,
    }
    db.WriteProvenanceEdge(context.Background(), edge)

    // Run trace command
    output := runTraceCommand(t, db.Path(), flowToken, "text", "")

    // Verify output
    assert.Contains(t, output, "Flow: flow-test-123")
    assert.Contains(t, output, "[1] Cart.addItem invoked")
    assert.Contains(t, output, "[2] Cart.addItem completed (Success)")
    assert.Contains(t, output, "[3] Inventory.reserve invoked")
    assert.Contains(t, output, "triggered by: sync \"cart-inventory\" from [2]")
    assert.Contains(t, output, "Provenance:")
    assert.Contains(t, output, "[3] ← [2] via cart-inventory")
}
```

**Test JSON Format**
```go
func TestTraceCommand_JSONFormat(t *testing.T) {
    db := setupTestDB(t)
    flowToken := "flow-json-test"

    // Create simple flow
    inv := testInvocation("Test.Action", 1, flowToken)
    db.WriteInvocation(context.Background(), inv)
    comp := testCompletion(inv.ID, "Success", 2)
    db.WriteCompletion(context.Background(), comp)

    // Run trace with JSON format
    output := runTraceCommand(t, db.Path(), flowToken, "json", "")

    // Parse JSON
    var trace FlowTrace
    err := json.Unmarshal([]byte(output), &trace)
    require.NoError(t, err)

    // Verify structure
    assert.Equal(t, flowToken, trace.FlowToken)
    assert.Len(t, trace.Timeline, 2)
    assert.Equal(t, "invoked", trace.Timeline[0].EventType)
    assert.Equal(t, "completed", trace.Timeline[1].EventType)
}
```

**Test Action Filter**
```go
func TestTraceCommand_ActionFilter(t *testing.T) {
    db := setupTestDB(t)
    flowToken := "flow-filter-test"

    // Create flow with multiple actions
    inv1 := testInvocation("Cart.addItem", 1, flowToken)
    db.WriteInvocation(context.Background(), inv1)
    comp1 := testCompletion(inv1.ID, "Success", 2)
    db.WriteCompletion(context.Background(), comp1)

    inv2 := testInvocation("Inventory.reserve", 3, flowToken)
    db.WriteInvocation(context.Background(), inv2)
    comp2 := testCompletion(inv2.ID, "Success", 4)
    db.WriteCompletion(context.Background(), comp2)

    // Filter to Inventory.* only
    output := runTraceCommand(t, db.Path(), flowToken, "text", "Inventory.*")

    // Verify only Inventory actions shown in timeline
    assert.Contains(t, output, "Inventory.reserve invoked")
    assert.NotContains(t, output, "Cart.addItem invoked")

    // Provenance still shown (full causality)
    assert.Contains(t, output, "Provenance:")
}
```

**Test Flow Not Found**
```go
func TestTraceCommand_FlowNotFound(t *testing.T) {
    db := setupTestDB(t)

    // Run trace for nonexistent flow
    err := runTraceCommandErr(t, db.Path(), "nonexistent", "text", "")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "flow nonexistent not found")
}
```

**Test Database Not Found**
```go
func TestTraceCommand_DatabaseNotFound(t *testing.T) {
    err := runTraceCommandErr(t, "/nonexistent/path.db", "flow-123", "text", "")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "database not found")
}
```

### File List

Files to create:

1. `internal/cli/trace.go` - Trace command implementation
2. `internal/cli/trace_test.go` - Comprehensive trace command tests

Files to modify:

1. `cmd/nysm/main.go` - Register trace command
2. `internal/store/read.go` - Add ReadFlowInvocations, ReadFlowCompletions (if not exist)

Files that must exist (from previous stories):

1. `internal/store/read.go` - ReadProvenance function (Story 2.6)
2. `internal/store/store.go` - Store struct with Open function (Story 2.1)
3. `internal/ir/types.go` - Invocation, Completion types (Story 1.1)
4. `internal/ir/store_types.go` - ProvenanceRecord type (Story 2.6)

### Relationship to Other Stories

**Dependencies:**
- Story 2.4: ReadFlow function for querying invocations/completions by flow token
- Story 2.6: ReadProvenance function for causality queries
- Story 7.1: CLI framework setup (Cobra commands)

**Builds on:**
- Story 1.5: Content-addressed IDs for invocation/completion identity
- Story 2.2: Logical clocks (seq) for deterministic ordering
- Story 3.5: Flow token generation and propagation
- Story 3.6: Flow token on all records

**Enables:**
- Story 7.10: Demo scenarios use trace command for validation
- Epic 6: Conformance harness can use trace output for assertions

### Story Completion Checklist

- [ ] `traceCmd` added to Cobra CLI
- [ ] `--db` flag (required)
- [ ] `--flow` flag (required)
- [ ] `--format` flag (text|json)
- [ ] `--action` flag (optional filter)
- [ ] Database opening with error handling
- [ ] Query invocations for flow
- [ ] Query completions for flow
- [ ] Query provenance for all invocations
- [ ] Merge timeline by seq (deterministic ordering)
- [ ] Text format output (timeline + provenance)
- [ ] JSON format output (structured)
- [ ] Action filter implementation (glob matching)
- [ ] Provenance annotation on sync-triggered invocations
- [ ] Error handling: flow not found
- [ ] Error handling: database not found
- [ ] Test: normal flow trace (text)
- [ ] Test: JSON format
- [ ] Test: action filter
- [ ] Test: flow not found
- [ ] Test: database not found
- [ ] Test: multi-binding provenance
- [ ] Test: user-initiated flow (no provenance)
- [ ] Test: timeline ordering determinism
- [ ] Help text with examples
- [ ] `go vet ./internal/cli/...` passes
- [ ] `go test ./internal/cli/...` passes

### References

- [Source: docs/prd.md#FR-4.1] - Provenance edges enable causality queries
- [Source: docs/prd.md#NFR-1.3] - Full trace reconstruction
- [Source: docs/prd.md#NFR-4.2] - Traceable sync evaluation
- [Source: docs/architecture.md#CLI Commands] - Trace command definition
- [Source: docs/architecture.md#Structured Logging] - Correlation keys
- [Source: docs/architecture.md#CP-4] - Deterministic ordering
- [Source: docs/epics.md#Story 7.7] - Story definition
- [Source: Story 2.4] - ReadFlow function (prerequisite)
- [Source: Story 2.6] - ReadProvenance function (prerequisite)
- [Source: Story 7.1] - CLI framework setup (prerequisite)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: comprehensive story generation based on epics.md Story 7.7

### Completion Notes

- Trace command is the "why did this happen?" query from the WYSIWYG paper
- Timeline view shows chronological flow of invocations and completions
- Provenance section shows causality: invocation ← completion via sync rule
- Text format optimized for human debugging and understanding
- JSON format enables programmatic analysis and tooling integration
- Action filter useful for large flows with many concepts
- Provenance annotation on sync-triggered invocations provides inline causality
- Multi-binding syncs create multiple provenance edges (one per invocation)
- User-initiated invocations have no provenance (valid, not error)
- Deterministic ordering (seq ASC, id ASC) ensures consistent output
- Story implements FR-4.1, NFR-1.3, NFR-4.2 (traceable evaluation)
- Trace command is essential for debugging and understanding NYSM applications
