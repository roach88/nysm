package store

import (
	"context"
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// FlowState represents the state of a flow for recovery purposes.
type FlowState struct {
	FlowToken       string
	Invocations     []ir.Invocation
	Completions     []ir.Completion
	SyncFirings     []ir.SyncFiring
	LastSeq         int64
	IsComplete      bool   // True if all invocations complete AND all sync firings have triggered invocations
	PendingCount    int    // Invocations without completions
	OrphanedFirings int    // Sync firings without provenance edges (crash recovery indicator)
	TerminalStatus  string // Empty, "Success", or error case
}

// GetFlowState retrieves the complete state of a flow for recovery analysis.
// Returns all invocations, completions, and sync firings with analysis of completeness.
func (s *Store) GetFlowState(ctx context.Context, flowToken string) (FlowState, error) {
	state := FlowState{
		FlowToken: flowToken,
	}

	// Get all invocations for the flow
	invocations, err := s.readFlowInvocations(ctx, flowToken)
	if err != nil {
		return state, fmt.Errorf("get flow state: %w", err)
	}
	state.Invocations = invocations

	// Get all completions for the flow
	completions, err := s.readFlowCompletions(ctx, flowToken)
	if err != nil {
		return state, fmt.Errorf("get flow state: %w", err)
	}
	state.Completions = completions

	// Build completion map for analysis
	completedInvocations := make(map[string]bool)
	for _, comp := range completions {
		completedInvocations[comp.InvocationID] = true
		if comp.Seq > state.LastSeq {
			state.LastSeq = comp.Seq
		}
	}

	// Update max seq from invocations too
	for _, inv := range invocations {
		if inv.Seq > state.LastSeq {
			state.LastSeq = inv.Seq
		}
		if !completedInvocations[inv.ID] {
			state.PendingCount++
		}
	}

	// Get sync firings for the flow (via completions)
	for _, comp := range completions {
		firings, err := s.ReadSyncFiringsForCompletion(ctx, comp.ID)
		if err != nil {
			return state, fmt.Errorf("get flow state: %w", err)
		}
		state.SyncFirings = append(state.SyncFirings, firings...)
	}

	// Count orphaned firings in a single batch query (avoids N+1)
	// These indicate crash recovery is needed - sync fired but didn't generate invocation
	completionIDs := make([]string, len(completions))
	for i, comp := range completions {
		completionIDs[i] = comp.ID
	}
	orphanCount, err := s.countOrphanedFiringsForCompletions(ctx, completionIDs)
	if err != nil {
		return state, fmt.Errorf("get flow state: %w", err)
	}
	state.OrphanedFirings = orphanCount

	// Determine if flow is complete
	// A flow is complete if:
	// 1. All invocations have completions (PendingCount == 0)
	// 2. No sync firings are orphaned (all triggered invocations exist)
	// 3. At least one invocation exists (not an empty flow)
	state.IsComplete = state.PendingCount == 0 && state.OrphanedFirings == 0 && len(invocations) > 0

	// Get terminal status from last completion
	if len(completions) > 0 {
		state.TerminalStatus = completions[len(completions)-1].OutputCase
	}

	return state, nil
}

// FindIncompleteFlows returns all flows that need recovery attention.
// A flow is incomplete if:
// 1. Some invocations don't have corresponding completions, OR
// 2. Some sync firings don't have provenance edges (orphaned firings)
//
// Used for crash recovery to identify flows that need to be resumed.
func (s *Store) FindIncompleteFlows(ctx context.Context) ([]FlowState, error) {
	// Get all unique flow tokens that have pending invocations OR orphaned firings
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT flow_token FROM (
			-- Flows with pending invocations (no completion)
			SELECT i.flow_token
			FROM invocations i
			LEFT JOIN completions c ON i.id = c.invocation_id
			WHERE c.id IS NULL

			UNION

			-- Flows with orphaned sync firings (no provenance edge)
			SELECT i.flow_token
			FROM sync_firings sf
			LEFT JOIN provenance_edges pe ON sf.id = pe.sync_firing_id
			JOIN completions c ON sf.completion_id = c.id
			JOIN invocations i ON c.invocation_id = i.id
			WHERE pe.id IS NULL
		)
		ORDER BY flow_token
	`)
	if err != nil {
		return nil, fmt.Errorf("find incomplete flows: %w", err)
	}
	defer rows.Close()

	var flowTokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("scan flow token: %w", err)
		}
		flowTokens = append(flowTokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flow tokens: %w", err)
	}

	// Get full state for each incomplete flow
	var states []FlowState
	for _, token := range flowTokens {
		state, err := s.GetFlowState(ctx, token)
		if err != nil {
			return nil, err
		}
		states = append(states, state)
	}

	return states, nil
}

// GetPendingInvocations returns invocations that don't have completions.
// Used for recovery to identify which actions need to be re-executed.
// Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) GetPendingInvocations(ctx context.Context, flowToken string) ([]ir.Invocation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.flow_token, i.action_uri, i.args, i.seq,
		       i.security_context, i.spec_hash, i.engine_version, i.ir_version
		FROM invocations i
		LEFT JOIN completions c ON i.id = c.invocation_id
		WHERE i.flow_token = ? AND c.id IS NULL
		ORDER BY i.seq ASC, i.id COLLATE BINARY ASC
	`, flowToken)
	if err != nil {
		return nil, fmt.Errorf("get pending invocations: %w", err)
	}
	defer rows.Close()

	var invocations []ir.Invocation
	for rows.Next() {
		inv, err := scanInvocation(rows)
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending invocations: %w", err)
	}

	if invocations == nil {
		invocations = []ir.Invocation{}
	}

	return invocations, nil
}

// ReplayFlow returns all events for a flow in replay order.
// Events are returned as a merged, time-ordered stream of invocations and completions.
// This is used for explicit replay scenarios to verify determinism.
//
// The returned events maintain the original seq ordering, ensuring replay
// produces identical results per CP-2 and CP-4.
func (s *Store) ReplayFlow(ctx context.Context, flowToken string) ([]FlowEvent, error) {
	state, err := s.GetFlowState(ctx, flowToken)
	if err != nil {
		return nil, err
	}

	// Merge invocations and completions into event stream
	var events []FlowEvent

	// Add invocations
	for _, inv := range state.Invocations {
		events = append(events, FlowEvent{
			Type:       EventInvocation,
			Seq:        inv.Seq,
			ID:         inv.ID,
			Invocation: &inv,
		})
	}

	// Add completions
	for _, comp := range state.Completions {
		events = append(events, FlowEvent{
			Type:       EventCompletion,
			Seq:        comp.Seq,
			ID:         comp.ID,
			Completion: &comp,
		})
	}

	// Sort by seq, then by type (invocations before completions for same seq)
	sortFlowEvents(events)

	return events, nil
}

// FlowEvent represents a single event in a flow (invocation or completion).
type FlowEvent struct {
	Type       FlowEventType
	Seq        int64
	ID         string
	Invocation *ir.Invocation
	Completion *ir.Completion
}

// FlowEventType distinguishes between invocations and completions.
type FlowEventType int

const (
	EventInvocation FlowEventType = iota
	EventCompletion
)

// String returns the event type as a string.
func (t FlowEventType) String() string {
	switch t {
	case EventInvocation:
		return "invocation"
	case EventCompletion:
		return "completion"
	default:
		return "unknown"
	}
}

// sortFlowEvents sorts events by seq, with invocations before completions for equal seq.
// This ensures deterministic ordering per CP-4.
func sortFlowEvents(events []FlowEvent) {
	// Simple insertion sort (events are typically small)
	for i := 1; i < len(events); i++ {
		j := i
		for j > 0 && eventLess(events[j], events[j-1]) {
			events[j], events[j-1] = events[j-1], events[j]
			j--
		}
	}
}

// eventLess compares two events for ordering.
// Orders by seq first, then by type (invocations before completions), then by ID.
func eventLess(a, b FlowEvent) bool {
	if a.Seq != b.Seq {
		return a.Seq < b.Seq
	}
	if a.Type != b.Type {
		return a.Type < b.Type // Invocation (0) before Completion (1)
	}
	return a.ID < b.ID
}

// GetLastSeq returns the highest seq number used in the store.
// Used for recovery to resume the logical clock from the correct position.
func (s *Store) GetLastSeq(ctx context.Context) (int64, error) {
	var maxSeq int64

	// Check invocations
	var invSeq int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM invocations
	`).Scan(&invSeq)
	if err != nil {
		return 0, fmt.Errorf("get last seq from invocations: %w", err)
	}
	maxSeq = invSeq

	// Check completions
	var compSeq int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM completions
	`).Scan(&compSeq)
	if err != nil {
		return 0, fmt.Errorf("get last seq from completions: %w", err)
	}
	if compSeq > maxSeq {
		maxSeq = compSeq
	}

	// Check sync_firings
	var firingSeq int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM sync_firings
	`).Scan(&firingSeq)
	if err != nil {
		return 0, fmt.Errorf("get last seq from sync_firings: %w", err)
	}
	if firingSeq > maxSeq {
		maxSeq = firingSeq
	}

	return maxSeq, nil
}

// ListFlowTokens returns all distinct flow tokens in the database.
// Used for replay and analysis commands to enumerate all flows.
// Results ordered alphabetically by flow token.
func (s *Store) ListFlowTokens(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT flow_token FROM invocations
		ORDER BY flow_token
	`)
	if err != nil {
		return nil, fmt.Errorf("list flow tokens: %w", err)
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return nil, fmt.Errorf("scan flow token: %w", err)
		}
		tokens = append(tokens, token)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate flow tokens: %w", err)
	}

	if tokens == nil {
		tokens = []string{}
	}

	return tokens, nil
}

// GetLastSeqForFlow returns the highest seq number used in a specific flow.
// Used for flow-scoped recovery.
func (s *Store) GetLastSeqForFlow(ctx context.Context, flowToken string) (int64, error) {
	var maxSeq int64

	// Check invocations
	var invSeq int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(seq), 0) FROM invocations WHERE flow_token = ?
	`, flowToken).Scan(&invSeq)
	if err != nil {
		return 0, fmt.Errorf("get last seq from invocations: %w", err)
	}
	maxSeq = invSeq

	// Check completions (join with invocations for flow_token)
	var compSeq int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(c.seq), 0)
		FROM completions c
		JOIN invocations i ON c.invocation_id = i.id
		WHERE i.flow_token = ?
	`, flowToken).Scan(&compSeq)
	if err != nil {
		return 0, fmt.Errorf("get last seq from completions: %w", err)
	}
	if compSeq > maxSeq {
		maxSeq = compSeq
	}

	return maxSeq, nil
}
