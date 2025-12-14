package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// TraceOptions holds flags for the trace command.
type TraceOptions struct {
	*RootOptions
	Database  string
	FlowToken string
	Action    string // optional - filter to specific action
}

// TraceEvent represents a single event in the trace timeline.
type TraceEvent struct {
	Seq        int64                  `json:"seq"`
	Type       string                 `json:"type"` // "invocation" or "completion"
	ID         string                 `json:"id"`
	ActionURI  string                 `json:"action_uri,omitempty"`
	Args       map[string]interface{} `json:"args,omitempty"`
	OutputCase string                 `json:"output_case,omitempty"`
	Result     map[string]interface{} `json:"result,omitempty"`
}

// ProvenanceEdge represents a causal relationship in the provenance graph.
type ProvenanceEdge struct {
	FromCompletion string `json:"from_completion"`
	SyncRule       string `json:"sync_rule"`
	ToInvocation   string `json:"to_invocation"`
}

// TraceResult holds the complete trace output.
type TraceResult struct {
	FlowToken   string           `json:"flow_token"`
	Timeline    []TraceEvent     `json:"timeline"`
	Provenance  []ProvenanceEdge `json:"provenance"`
	Stats       TraceStats       `json:"stats"`
}

// TraceStats holds summary statistics for the trace.
type TraceStats struct {
	TotalEvents  int  `json:"total_events"`
	Invocations  int  `json:"invocations"`
	Completions  int  `json:"completions"`
	SyncFirings  int  `json:"sync_firings"`
	IsComplete   bool `json:"is_complete"`
}

// NewTraceCommand creates the trace command.
func NewTraceCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &TraceOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "trace",
		Short: "Query provenance for a flow",
		Long: `Query the provenance graph for a specific flow.

Shows the causal chain of events: which invocations triggered
which sync rules, and their resulting completions.

The output includes:
- Timeline: Chronological list of invocations and completions
- Provenance: Causal relationships showing how events triggered other events
- Stats: Summary statistics for the flow

Examples:
  nysm trace --db ./nysm.db --flow test-flow-1
  nysm trace --db ./nysm.db --flow test-flow-1 --action Cart.addItem
  nysm trace --db ./nysm.db --flow test-flow-1 --format json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTrace(opts, cmd)
		},
	}

	cmd.Flags().StringVar(&opts.Database, "db", "", "path to SQLite database (required)")
	_ = cmd.MarkFlagRequired("db")
	cmd.Flags().StringVar(&opts.FlowToken, "flow", "", "flow token to trace (required)")
	_ = cmd.MarkFlagRequired("flow")
	cmd.Flags().StringVar(&opts.Action, "action", "", "filter to specific action URI")

	return cmd
}

func runTrace(opts *TraceOptions, cmd *cobra.Command) error {
	ctx := context.Background()

	// Open database
	st, err := store.Open(opts.Database)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to open database", err)
	}
	defer st.Close()

	// Get flow state and events
	state, err := st.GetFlowState(ctx, opts.FlowToken)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to get flow state", err)
	}

	// Get replay events for timeline
	events, err := st.ReplayFlow(ctx, opts.FlowToken)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to replay flow", err)
	}

	// Check if flow exists
	if len(events) == 0 {
		if opts.Format == "json" {
			return outputTraceJSON(cmd, TraceResult{
				FlowToken:  opts.FlowToken,
				Timeline:   []TraceEvent{},
				Provenance: []ProvenanceEdge{},
				Stats:      TraceStats{},
			})
		}
		fmt.Fprintf(cmd.OutOrStdout(), "No events found for flow: %s\n", opts.FlowToken)
		return nil
	}

	// Build timeline
	timeline := buildTimeline(events, opts.Action)

	// Build provenance graph
	provenance, err := buildProvenance(ctx, st, state)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to build provenance", err)
	}

	// Build result
	result := TraceResult{
		FlowToken:  opts.FlowToken,
		Timeline:   timeline,
		Provenance: provenance,
		Stats: TraceStats{
			TotalEvents: len(timeline),
			Invocations: len(state.Invocations),
			Completions: len(state.Completions),
			SyncFirings: len(state.SyncFirings),
			IsComplete:  state.IsComplete,
		},
	}

	// Output results
	if opts.Format == "json" {
		return outputTraceJSON(cmd, result)
	}

	return outputTraceText(cmd, result, opts.Verbose)
}

// buildTimeline converts store events to trace timeline events.
// When actionFilter is set, only includes invocations matching that action
// and their corresponding completions.
func buildTimeline(events []store.FlowEvent, actionFilter string) []TraceEvent {
	var timeline []TraceEvent

	// Track invocation IDs that match the filter (for filtering completions)
	matchedInvocationIDs := make(map[string]bool)

	// First pass: identify matching invocations if filter is set
	if actionFilter != "" {
		for _, event := range events {
			if event.Type == store.EventInvocation && event.Invocation != nil {
				if string(event.Invocation.ActionURI) == actionFilter {
					matchedInvocationIDs[event.Invocation.ID] = true
				}
			}
		}
	}

	// Second pass: build timeline with filtered events
	for _, event := range events {
		var traceEvent TraceEvent

		switch event.Type {
		case store.EventInvocation:
			if event.Invocation == nil {
				continue
			}
			inv := event.Invocation

			// Apply action filter
			if actionFilter != "" && string(inv.ActionURI) != actionFilter {
				continue
			}

			traceEvent = TraceEvent{
				Seq:       event.Seq,
				Type:      "invocation",
				ID:        inv.ID,
				ActionURI: string(inv.ActionURI),
				Args:      irObjectToMap(inv.Args),
			}

		case store.EventCompletion:
			if event.Completion == nil {
				continue
			}
			comp := event.Completion

			// If filtering by action, only include completions for matching invocations
			if actionFilter != "" && !matchedInvocationIDs[comp.InvocationID] {
				continue
			}

			traceEvent = TraceEvent{
				Seq:        event.Seq,
				Type:       "completion",
				ID:         comp.ID,
				OutputCase: comp.OutputCase,
				Result:     irObjectToMap(comp.Result),
			}
		}

		timeline = append(timeline, traceEvent)
	}

	return timeline
}

// irObjectToMap converts an ir.IRObject to a plain map.
func irObjectToMap(obj ir.IRObject) map[string]interface{} {
	if obj == nil {
		return nil
	}

	result := make(map[string]interface{})
	for k, v := range obj {
		result[k] = irValueToInterface(v)
	}
	return result
}

// irValueToInterface converts an ir.IRValue to a plain interface{}.
func irValueToInterface(v ir.IRValue) interface{} {
	switch val := v.(type) {
	case ir.IRString:
		return string(val)
	case ir.IRInt:
		return int64(val)
	case ir.IRBool:
		return bool(val)
	case ir.IRArray:
		result := make([]interface{}, len(val))
		for i, elem := range val {
			result[i] = irValueToInterface(elem)
		}
		return result
	case ir.IRObject:
		return irObjectToMap(val)
	default:
		return nil
	}
}

// buildProvenance constructs the provenance graph from sync firings.
func buildProvenance(ctx context.Context, st *store.Store, state store.FlowState) ([]ProvenanceEdge, error) {
	var edges []ProvenanceEdge

	// Get provenance edges for each completion
	for _, comp := range state.Completions {
		// Get sync firings that were triggered by this completion
		firings, err := st.ReadSyncFiringsForCompletion(ctx, comp.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to read sync firings: %w", err)
		}

		for _, firing := range firings {
			// Get the provenance edge (which invocation was created by this firing)
			provenanceEdges, err := st.ReadProvenanceEdgesForFiring(ctx, firing.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to read provenance edges: %w", err)
			}

			for _, pe := range provenanceEdges {
				edges = append(edges, ProvenanceEdge{
					FromCompletion: comp.ID,
					SyncRule:       firing.SyncID,
					ToInvocation:   pe.InvocationID,
				})
			}
		}
	}

	return edges, nil
}

// outputTraceJSON outputs the trace result as JSON.
func outputTraceJSON(cmd *cobra.Command, result TraceResult) error {
	response := CLIResponse{
		Status: "ok",
		Data:   result,
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}

// outputTraceText outputs the trace result as text.
func outputTraceText(cmd *cobra.Command, result TraceResult, verbose bool) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Trace for Flow: %s\n", result.FlowToken)
	fmt.Fprintf(w, "Status: %s\n", completeStatus(result.Stats.IsComplete))
	fmt.Fprintln(w)

	// Timeline section
	fmt.Fprintln(w, "=== Timeline ===")
	if len(result.Timeline) == 0 {
		fmt.Fprintln(w, "  (no events)")
	} else {
		for _, event := range result.Timeline {
			formatTimelineEvent(w, event, verbose)
		}
	}
	fmt.Fprintln(w)

	// Provenance section
	fmt.Fprintln(w, "=== Provenance ===")
	if len(result.Provenance) == 0 {
		fmt.Fprintln(w, "  (no causal relationships)")
	} else {
		for _, edge := range result.Provenance {
			fmt.Fprintf(w, "  %s -[%s]-> %s\n",
				truncateID(edge.FromCompletion),
				edge.SyncRule,
				truncateID(edge.ToInvocation))
		}
	}
	fmt.Fprintln(w)

	// Stats section
	fmt.Fprintln(w, "=== Stats ===")
	fmt.Fprintf(w, "  Total Events: %d\n", result.Stats.TotalEvents)
	fmt.Fprintf(w, "  Invocations:  %d\n", result.Stats.Invocations)
	fmt.Fprintf(w, "  Completions:  %d\n", result.Stats.Completions)
	fmt.Fprintf(w, "  Sync Firings: %d\n", result.Stats.SyncFirings)

	return nil
}

// formatTimelineEvent formats a single timeline event for text output.
func formatTimelineEvent(w interface{ Write([]byte) (int, error) }, event TraceEvent, verbose bool) {
	switch event.Type {
	case "invocation":
		fmt.Fprintf(w, "  [%d] INV %s\n", event.Seq, event.ActionURI)
		if verbose && len(event.Args) > 0 {
			fmt.Fprintf(w, "       Args: %s\n", formatArgs(event.Args))
		}
		if verbose {
			fmt.Fprintf(w, "       ID: %s\n", truncateID(event.ID))
		}

	case "completion":
		fmt.Fprintf(w, "  [%d] COMP %s\n", event.Seq, event.OutputCase)
		if verbose && len(event.Result) > 0 {
			fmt.Fprintf(w, "       Result: %s\n", formatArgs(event.Result))
		}
		if verbose {
			fmt.Fprintf(w, "       ID: %s\n", truncateID(event.ID))
		}
	}
}

// formatArgs formats a map of args for display.
// Uses sorted keys to ensure deterministic output.
func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return "{}"
	}

	// Sort keys for deterministic output
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, formatValue(args[k])))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// formatValue formats a single value for display, handling nested structures deterministically.
func formatValue(v interface{}) string {
	switch val := v.(type) {
	case map[string]interface{}:
		return formatArgs(val)
	case []interface{}:
		parts := make([]string, len(val))
		for i, elem := range val {
			parts[i] = formatValue(elem)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case string:
		return val
	default:
		return fmt.Sprintf("%v", v)
	}
}

// truncateID truncates a long ID for display.
func truncateID(id string) string {
	if len(id) <= 16 {
		return id
	}
	return id[:8] + "..." + id[len(id)-8:]
}

// completeStatus returns a human-readable completion status.
func completeStatus(isComplete bool) string {
	if isComplete {
		return "Complete"
	}
	return "Incomplete (pending events)"
}
