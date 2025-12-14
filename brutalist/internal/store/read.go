package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// ReadFlow returns all invocations and completions for a flow token.
// Results are ordered deterministically per CP-4: ORDER BY seq ASC, id ASC COLLATE BINARY.
//
// Returns empty slices (not nil) if no records exist for the flow token.
func (s *Store) ReadFlow(ctx context.Context, flowToken string) ([]ir.Invocation, []ir.Completion, error) {
	invocations, err := s.readFlowInvocations(ctx, flowToken)
	if err != nil {
		return nil, nil, err
	}

	completions, err := s.readFlowCompletions(ctx, flowToken)
	if err != nil {
		return nil, nil, err
	}

	return invocations, completions, nil
}

// readFlowInvocations returns all invocations for a flow token with deterministic ordering.
func (s *Store) readFlowInvocations(ctx context.Context, flowToken string) ([]ir.Invocation, error) {
	// CP-4: Deterministic ordering - ORDER BY seq ASC, id COLLATE BINARY ASC
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
		FROM invocations
		WHERE flow_token = ?
		ORDER BY seq ASC, id COLLATE BINARY ASC
	`, flowToken)
	if err != nil {
		return nil, fmt.Errorf("query invocations: %w", err)
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
		return nil, fmt.Errorf("iterate invocations: %w", err)
	}

	// Return empty slice instead of nil
	if invocations == nil {
		invocations = []ir.Invocation{}
	}

	return invocations, nil
}

// readFlowCompletions returns all completions for a flow token with deterministic ordering.
func (s *Store) readFlowCompletions(ctx context.Context, flowToken string) ([]ir.Completion, error) {
	// CP-4: Deterministic ordering - ORDER BY seq ASC, id COLLATE BINARY ASC
	// Join with invocations to filter by flow_token
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.invocation_id, c.output_case, c.result, c.seq, c.security_context
		FROM completions c
		JOIN invocations i ON c.invocation_id = i.id
		WHERE i.flow_token = ?
		ORDER BY c.seq ASC, c.id COLLATE BINARY ASC
	`, flowToken)
	if err != nil {
		return nil, fmt.Errorf("query completions: %w", err)
	}
	defer rows.Close()

	var completions []ir.Completion
	for rows.Next() {
		comp, err := scanCompletion(rows)
		if err != nil {
			return nil, err
		}
		completions = append(completions, comp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completions: %w", err)
	}

	// Return empty slice instead of nil
	if completions == nil {
		completions = []ir.Completion{}
	}

	return completions, nil
}

// ReadInvocation retrieves a single invocation by ID.
// Returns sql.ErrNoRows if not found.
func (s *Store) ReadInvocation(ctx context.Context, id string) (ir.Invocation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
		FROM invocations
		WHERE id = ?
	`, id)

	return scanInvocationRow(row)
}

// ReadCompletion retrieves a single completion by ID.
// Returns sql.ErrNoRows if not found.
func (s *Store) ReadCompletion(ctx context.Context, id string) (ir.Completion, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, invocation_id, output_case, result, seq, security_context
		FROM completions
		WHERE id = ?
	`, id)

	return scanCompletionRow(row)
}

// ReadAllInvocations returns all invocations with deterministic ordering.
// Used for replay scenarios. Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) ReadAllInvocations(ctx context.Context) ([]ir.Invocation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version
		FROM invocations
		ORDER BY seq ASC, id COLLATE BINARY ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all invocations: %w", err)
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
		return nil, fmt.Errorf("iterate invocations: %w", err)
	}

	if invocations == nil {
		invocations = []ir.Invocation{}
	}

	return invocations, nil
}

// ReadAllCompletions returns all completions with deterministic ordering.
// Used for replay scenarios. Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) ReadAllCompletions(ctx context.Context) ([]ir.Completion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, invocation_id, output_case, result, seq, security_context
		FROM completions
		ORDER BY seq ASC, id COLLATE BINARY ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all completions: %w", err)
	}
	defer rows.Close()

	var completions []ir.Completion
	for rows.Next() {
		comp, err := scanCompletion(rows)
		if err != nil {
			return nil, err
		}
		completions = append(completions, comp)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate completions: %w", err)
	}

	if completions == nil {
		completions = []ir.Completion{}
	}

	return completions, nil
}

// scanInvocation scans a row into an Invocation struct.
func scanInvocation(rows *sql.Rows) (ir.Invocation, error) {
	var inv ir.Invocation
	var actionURI string
	var argsJSON, secCtxJSON string

	if err := rows.Scan(
		&inv.ID, &inv.FlowToken, &actionURI, &argsJSON, &inv.Seq,
		&secCtxJSON, &inv.SpecHash, &inv.EngineVersion, &inv.IRVersion,
	); err != nil {
		return ir.Invocation{}, fmt.Errorf("scan invocation: %w", err)
	}

	inv.ActionURI = ir.ActionRef(actionURI)

	args, err := unmarshalArgs(argsJSON)
	if err != nil {
		return ir.Invocation{}, err
	}
	inv.Args = args

	secCtx, err := unmarshalSecurityContext(secCtxJSON)
	if err != nil {
		return ir.Invocation{}, err
	}
	inv.SecurityContext = secCtx

	return inv, nil
}

// scanInvocationRow scans a single row into an Invocation struct.
func scanInvocationRow(row *sql.Row) (ir.Invocation, error) {
	var inv ir.Invocation
	var actionURI string
	var argsJSON, secCtxJSON string

	if err := row.Scan(
		&inv.ID, &inv.FlowToken, &actionURI, &argsJSON, &inv.Seq,
		&secCtxJSON, &inv.SpecHash, &inv.EngineVersion, &inv.IRVersion,
	); err != nil {
		return ir.Invocation{}, err
	}

	inv.ActionURI = ir.ActionRef(actionURI)

	args, err := unmarshalArgs(argsJSON)
	if err != nil {
		return ir.Invocation{}, err
	}
	inv.Args = args

	secCtx, err := unmarshalSecurityContext(secCtxJSON)
	if err != nil {
		return ir.Invocation{}, err
	}
	inv.SecurityContext = secCtx

	return inv, nil
}

// scanCompletion scans a row into a Completion struct.
func scanCompletion(rows *sql.Rows) (ir.Completion, error) {
	var comp ir.Completion
	var resultJSON, secCtxJSON string

	if err := rows.Scan(
		&comp.ID, &comp.InvocationID, &comp.OutputCase, &resultJSON, &comp.Seq, &secCtxJSON,
	); err != nil {
		return ir.Completion{}, fmt.Errorf("scan completion: %w", err)
	}

	result, err := unmarshalResult(resultJSON)
	if err != nil {
		return ir.Completion{}, err
	}
	comp.Result = result

	secCtx, err := unmarshalSecurityContext(secCtxJSON)
	if err != nil {
		return ir.Completion{}, err
	}
	comp.SecurityContext = secCtx

	return comp, nil
}

// scanCompletionRow scans a single row into a Completion struct.
func scanCompletionRow(row *sql.Row) (ir.Completion, error) {
	var comp ir.Completion
	var resultJSON, secCtxJSON string

	if err := row.Scan(
		&comp.ID, &comp.InvocationID, &comp.OutputCase, &resultJSON, &comp.Seq, &secCtxJSON,
	); err != nil {
		return ir.Completion{}, err
	}

	result, err := unmarshalResult(resultJSON)
	if err != nil {
		return ir.Completion{}, err
	}
	comp.Result = result

	secCtx, err := unmarshalSecurityContext(secCtxJSON)
	if err != nil {
		return ir.Completion{}, err
	}
	comp.SecurityContext = secCtx

	return comp, nil
}

// ReadSyncFiring retrieves a single sync firing by ID.
// Returns sql.ErrNoRows if not found.
func (s *Store) ReadSyncFiring(ctx context.Context, id int64) (ir.SyncFiring, error) {
	var firing ir.SyncFiring
	err := s.db.QueryRowContext(ctx, `
		SELECT id, completion_id, sync_id, binding_hash, seq
		FROM sync_firings
		WHERE id = ?
	`, id).Scan(
		&firing.ID, &firing.CompletionID, &firing.SyncID, &firing.BindingHash, &firing.Seq,
	)
	if err != nil {
		return ir.SyncFiring{}, err
	}
	return firing, nil
}

// ReadSyncFiringsForCompletion returns all sync firings triggered by a completion.
// Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) ReadSyncFiringsForCompletion(ctx context.Context, completionID string) ([]ir.SyncFiring, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, completion_id, sync_id, binding_hash, seq
		FROM sync_firings
		WHERE completion_id = ?
		ORDER BY seq ASC, id ASC
	`, completionID)
	if err != nil {
		return nil, fmt.Errorf("query sync firings: %w", err)
	}
	defer rows.Close()

	var firings []ir.SyncFiring
	for rows.Next() {
		var f ir.SyncFiring
		if err := rows.Scan(&f.ID, &f.CompletionID, &f.SyncID, &f.BindingHash, &f.Seq); err != nil {
			return nil, fmt.Errorf("scan sync firing: %w", err)
		}
		firings = append(firings, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync firings: %w", err)
	}

	if firings == nil {
		firings = []ir.SyncFiring{}
	}

	return firings, nil
}

// ReadAllSyncFirings returns all sync firings with deterministic ordering.
// Used for replay scenarios. Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) ReadAllSyncFirings(ctx context.Context) ([]ir.SyncFiring, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, completion_id, sync_id, binding_hash, seq
		FROM sync_firings
		ORDER BY seq ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all sync firings: %w", err)
	}
	defer rows.Close()

	var firings []ir.SyncFiring
	for rows.Next() {
		var f ir.SyncFiring
		if err := rows.Scan(&f.ID, &f.CompletionID, &f.SyncID, &f.BindingHash, &f.Seq); err != nil {
			return nil, fmt.Errorf("scan sync firing: %w", err)
		}
		firings = append(firings, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sync firings: %w", err)
	}

	if firings == nil {
		firings = []ir.SyncFiring{}
	}

	return firings, nil
}

// ReadProvenance returns all provenance edges for an invocation (backward trace).
// Answers: "what caused this invocation?"
// Results ordered by sync_firing.seq ASC, then provenance_edge.id ASC per CP-4
// for causality-aligned ordering.
func (s *Store) ReadProvenance(ctx context.Context, invocationID string) ([]ir.ProvenanceEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pe.id, pe.sync_firing_id, pe.invocation_id
		FROM provenance_edges pe
		JOIN sync_firings sf ON pe.sync_firing_id = sf.id
		WHERE pe.invocation_id = ?
		ORDER BY sf.seq ASC, pe.id ASC
	`, invocationID)
	if err != nil {
		return nil, fmt.Errorf("query provenance: %w", err)
	}
	defer rows.Close()

	var edges []ir.ProvenanceEdge
	for rows.Next() {
		var e ir.ProvenanceEdge
		if err := rows.Scan(&e.ID, &e.SyncFiringID, &e.InvocationID); err != nil {
			return nil, fmt.Errorf("scan provenance edge: %w", err)
		}
		edges = append(edges, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provenance edges: %w", err)
	}

	if edges == nil {
		edges = []ir.ProvenanceEdge{}
	}

	return edges, nil
}

// ReadTriggered returns all invocations triggered by a completion (forward trace).
// Answers: "what did this completion trigger?"
// Results ordered by sync_firings.seq ASC, invocation_id ASC per CP-4.
func (s *Store) ReadTriggered(ctx context.Context, completionID string) ([]ir.Invocation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT i.id, i.flow_token, i.action_uri, i.args, i.seq,
		       i.security_context, i.spec_hash, i.engine_version, i.ir_version
		FROM invocations i
		JOIN provenance_edges pe ON i.id = pe.invocation_id
		JOIN sync_firings sf ON pe.sync_firing_id = sf.id
		WHERE sf.completion_id = ?
		ORDER BY sf.seq ASC, i.id COLLATE BINARY ASC
	`, completionID)
	if err != nil {
		return nil, fmt.Errorf("query triggered invocations: %w", err)
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
		return nil, fmt.Errorf("iterate triggered invocations: %w", err)
	}

	if invocations == nil {
		invocations = []ir.Invocation{}
	}

	return invocations, nil
}

// hasFiringEdge checks if a sync firing has a corresponding provenance edge.
// Used for crash recovery to detect orphaned firings that didn't complete.
func (s *Store) hasFiringEdge(ctx context.Context, syncFiringID int64) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM provenance_edges WHERE sync_firing_id = ?
	`, syncFiringID).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check firing edge: %w", err)
	}
	return count > 0, nil
}

// countOrphanedFiringsForCompletions returns the number of sync firings without
// provenance edges for a set of completion IDs. This is a batch operation to
// avoid N+1 queries when checking multiple completions.
func (s *Store) countOrphanedFiringsForCompletions(ctx context.Context, completionIDs []string) (int, error) {
	if len(completionIDs) == 0 {
		return 0, nil
	}

	// Build placeholder string for IN clause
	placeholders := make([]byte, 0, len(completionIDs)*2-1)
	args := make([]any, len(completionIDs))
	for i, id := range completionIDs {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args[i] = id
	}

	var count int
	query := `
		SELECT COUNT(*)
		FROM sync_firings sf
		LEFT JOIN provenance_edges pe ON sf.id = pe.sync_firing_id
		WHERE sf.completion_id IN (` + string(placeholders) + `)
		AND pe.id IS NULL
	`
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count orphaned firings: %w", err)
	}
	return count, nil
}

// FindOrphanedSyncFirings returns all sync firings that don't have provenance edges.
// These represent incomplete crash recovery scenarios where sync fired but the
// triggered invocation was never created.
// Results ordered by seq ASC, id ASC per CP-4.
func (s *Store) FindOrphanedSyncFirings(ctx context.Context) ([]ir.SyncFiring, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT sf.id, sf.completion_id, sf.sync_id, sf.binding_hash, sf.seq
		FROM sync_firings sf
		LEFT JOIN provenance_edges pe ON sf.id = pe.sync_firing_id
		WHERE pe.id IS NULL
		ORDER BY sf.seq ASC, sf.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("find orphaned sync firings: %w", err)
	}
	defer rows.Close()

	var firings []ir.SyncFiring
	for rows.Next() {
		var f ir.SyncFiring
		if err := rows.Scan(&f.ID, &f.CompletionID, &f.SyncID, &f.BindingHash, &f.Seq); err != nil {
			return nil, fmt.Errorf("scan orphaned sync firing: %w", err)
		}
		firings = append(firings, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate orphaned sync firings: %w", err)
	}

	if firings == nil {
		firings = []ir.SyncFiring{}
	}

	return firings, nil
}

// ReadAllProvenanceEdges returns all provenance edges with deterministic ordering.
// Used for replay scenarios. Results ordered by sync_firing.seq ASC, then id ASC
// per CP-4 for causality-aligned ordering.
func (s *Store) ReadAllProvenanceEdges(ctx context.Context) ([]ir.ProvenanceEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pe.id, pe.sync_firing_id, pe.invocation_id
		FROM provenance_edges pe
		JOIN sync_firings sf ON pe.sync_firing_id = sf.id
		ORDER BY sf.seq ASC, pe.id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("query all provenance edges: %w", err)
	}
	defer rows.Close()

	var edges []ir.ProvenanceEdge
	for rows.Next() {
		var e ir.ProvenanceEdge
		if err := rows.Scan(&e.ID, &e.SyncFiringID, &e.InvocationID); err != nil {
			return nil, fmt.Errorf("scan provenance edge: %w", err)
		}
		edges = append(edges, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provenance edges: %w", err)
	}

	if edges == nil {
		edges = []ir.ProvenanceEdge{}
	}

	return edges, nil
}

// ReadProvenanceEdgesForFiring returns all provenance edges for a specific sync firing.
// Used for forward trace queries (what invocations were triggered by this firing).
// Results ordered by id ASC per CP-4.
func (s *Store) ReadProvenanceEdgesForFiring(ctx context.Context, syncFiringID int64) ([]ir.ProvenanceEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, sync_firing_id, invocation_id
		FROM provenance_edges
		WHERE sync_firing_id = ?
		ORDER BY id ASC
	`, syncFiringID)
	if err != nil {
		return nil, fmt.Errorf("query provenance edges for firing: %w", err)
	}
	defer rows.Close()

	var edges []ir.ProvenanceEdge
	for rows.Next() {
		var e ir.ProvenanceEdge
		if err := rows.Scan(&e.ID, &e.SyncFiringID, &e.InvocationID); err != nil {
			return nil, fmt.Errorf("scan provenance edge: %w", err)
		}
		edges = append(edges, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate provenance edges: %w", err)
	}

	if edges == nil {
		edges = []ir.ProvenanceEdge{}
	}

	return edges, nil
}
