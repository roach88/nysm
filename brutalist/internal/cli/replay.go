package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/store"
)

// ReplayOptions holds flags for the replay command.
type ReplayOptions struct {
	*RootOptions
	Database  string
	FlowToken string // optional - specific flow only
}

// ReplayFlowResult holds the replay result for a single flow.
type ReplayFlowResult struct {
	FlowToken    string `json:"flow_token"`
	Invocations  int    `json:"invocations"`
	Completions  int    `json:"completions"`
	SyncFirings  int    `json:"sync_firings"`
	IsComplete   bool   `json:"is_complete"`
	Deterministic bool  `json:"deterministic"`
}

// ReplayResult holds the overall replay result.
type ReplayResult struct {
	Flows         []ReplayFlowResult `json:"flows"`
	TotalFlows    int                `json:"total_flows"`
	AllDeterministic bool            `json:"all_deterministic"`
}

// NewReplayCommand creates the replay command.
func NewReplayCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &ReplayOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "replay",
		Short: "Replay event log and verify determinism",
		Long: `Replay the event log to verify determinism and report flow statistics.

This command re-reads all events in order, replays them twice to verify
deterministic behavior, and reports per-flow statistics including invocations,
completions, and sync firings.

Exit codes:
  0 - All flows are deterministic
  1 - Determinism verification failed (differences detected)
  2 - Command error (database not found, etc.)

Examples:
  nysm replay --db ./nysm.db
  nysm replay --db ./nysm.db --flow test-flow-1
  nysm replay --db ./nysm.db --format json`,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReplay(opts, cmd)
		},
	}

	cmd.Flags().StringVar(&opts.Database, "db", "", "path to SQLite database (required)")
	_ = cmd.MarkFlagRequired("db")
	cmd.Flags().StringVar(&opts.FlowToken, "flow", "", "replay specific flow only")

	return cmd
}

func runReplay(opts *ReplayOptions, cmd *cobra.Command) error {
	ctx := context.Background()

	// Open database
	st, err := store.Open(opts.Database)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to open database", err)
	}
	defer st.Close()

	// Get flow tokens to process
	var flowTokens []string
	if opts.FlowToken != "" {
		flowTokens = []string{opts.FlowToken}
	} else {
		flowTokens, err = st.ListFlowTokens(ctx)
		if err != nil {
			return WrapExitError(ExitCommandError, "failed to list flow tokens", err)
		}
	}

	if len(flowTokens) == 0 {
		if opts.Format == "json" {
			result := ReplayResult{
				Flows:            []ReplayFlowResult{},
				TotalFlows:       0,
				AllDeterministic: true,
			}
			return outputReplayJSON(cmd, result)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No flows found in database.")
		return nil
	}

	// Process each flow
	result := ReplayResult{
		Flows:            make([]ReplayFlowResult, 0, len(flowTokens)),
		TotalFlows:       len(flowTokens),
		AllDeterministic: true,
	}

	for _, token := range flowTokens {
		flowResult, err := replayAndVerifyFlow(ctx, st, token, opts.Verbose, cmd)
		if err != nil {
			return WrapExitError(ExitCommandError, fmt.Sprintf("failed to replay flow %s", token), err)
		}

		result.Flows = append(result.Flows, flowResult)
		if !flowResult.Deterministic {
			result.AllDeterministic = false
		}
	}

	// Output results
	if opts.Format == "json" {
		return outputReplayJSON(cmd, result)
	}

	return outputReplayText(cmd, result, opts.Verbose)
}

// replayAndVerifyFlow replays a single flow twice and verifies determinism.
func replayAndVerifyFlow(ctx context.Context, st *store.Store, flowToken string, verbose bool, cmd *cobra.Command) (ReplayFlowResult, error) {
	// Get flow state for statistics
	state, err := st.GetFlowState(ctx, flowToken)
	if err != nil {
		return ReplayFlowResult{}, err
	}

	// Replay the flow twice
	events1, err := st.ReplayFlow(ctx, flowToken)
	if err != nil {
		return ReplayFlowResult{}, fmt.Errorf("first replay failed: %w", err)
	}

	events2, err := st.ReplayFlow(ctx, flowToken)
	if err != nil {
		return ReplayFlowResult{}, fmt.Errorf("second replay failed: %w", err)
	}

	// Compare event sequences for determinism
	deterministic := compareEventSequences(events1, events2)

	return ReplayFlowResult{
		FlowToken:     flowToken,
		Invocations:   len(state.Invocations),
		Completions:   len(state.Completions),
		SyncFirings:   len(state.SyncFirings),
		IsComplete:    state.IsComplete,
		Deterministic: deterministic,
	}, nil
}

// compareEventSequences compares two event sequences for equality.
func compareEventSequences(events1, events2 []store.FlowEvent) bool {
	if len(events1) != len(events2) {
		return false
	}

	for i := range events1 {
		if !eventsEqual(events1[i], events2[i]) {
			return false
		}
	}

	return true
}

// eventsEqual compares two FlowEvents for equality.
func eventsEqual(a, b store.FlowEvent) bool {
	if a.Type != b.Type || a.Seq != b.Seq || a.ID != b.ID {
		return false
	}

	// Compare invocations if present
	if (a.Invocation == nil) != (b.Invocation == nil) {
		return false
	}
	if a.Invocation != nil {
		if !reflect.DeepEqual(a.Invocation, b.Invocation) {
			return false
		}
	}

	// Compare completions if present
	if (a.Completion == nil) != (b.Completion == nil) {
		return false
	}
	if a.Completion != nil {
		if !reflect.DeepEqual(a.Completion, b.Completion) {
			return false
		}
	}

	return true
}

// outputReplayJSON outputs the replay result as JSON.
func outputReplayJSON(cmd *cobra.Command, result ReplayResult) error {
	response := CLIResponse{
		Status: "ok",
		Data:   result,
	}

	if !result.AllDeterministic {
		response.Status = "error"
		response.Error = &CLIError{
			Code:    "E_DETERMINISM",
			Message: "determinism verification failed",
		}
	}

	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(response); err != nil {
		return err
	}

	if !result.AllDeterministic {
		// Determinism failure = exit code 1
		return NewExitError(ExitFailure, "determinism verification failed")
	}
	return nil
}

// outputReplayText outputs the replay result as text.
func outputReplayText(cmd *cobra.Command, result ReplayResult, verbose bool) error {
	w := cmd.OutOrStdout()

	fmt.Fprintf(w, "Replay Summary: %d flow(s)\n", result.TotalFlows)
	fmt.Fprintln(w)

	for _, flow := range result.Flows {
		status := "✓"
		if !flow.Deterministic {
			status = "✗"
		}

		fmt.Fprintf(w, "%s Flow: %s\n", status, flow.FlowToken)

		if verbose {
			fmt.Fprintf(w, "  Invocations: %d\n", flow.Invocations)
			fmt.Fprintf(w, "  Completions: %d\n", flow.Completions)
			fmt.Fprintf(w, "  Sync Firings: %d\n", flow.SyncFirings)
			fmt.Fprintf(w, "  Complete: %v\n", flow.IsComplete)
		} else {
			fmt.Fprintf(w, "  Events: %d invocations, %d completions\n", flow.Invocations, flow.Completions)
		}

		if !flow.Deterministic {
			fmt.Fprintln(w, "  Warning: Non-deterministic replay detected!")
		}
		fmt.Fprintln(w)
	}

	if result.AllDeterministic {
		fmt.Fprintln(w, "✓ All flows verified deterministic")
		return nil
	}

	fmt.Fprintln(w, "✗ Determinism verification failed")
	// Determinism failure = exit code 1
	return NewExitError(ExitFailure, "determinism verification failed")
}
