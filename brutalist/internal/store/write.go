package store

import (
	"context"
	"fmt"

	"github.com/roach88/nysm/internal/ir"
)

// WriteInvocation inserts an invocation record into the store.
// Uses ON CONFLICT(id) DO NOTHING for idempotency - duplicate IDs are silently ignored.
// Other constraint violations (e.g., NOT NULL) will still return errors.
//
// The invocation's Args and SecurityContext are serialized to canonical JSON
// per RFC 8785 for deterministic replay.
func (s *Store) WriteInvocation(ctx context.Context, inv ir.Invocation) error {
	argsJSON, err := marshalArgs(inv.Args)
	if err != nil {
		return fmt.Errorf("write invocation: %w", err)
	}

	secCtxJSON, err := marshalSecurityContext(inv.SecurityContext)
	if err != nil {
		return fmt.Errorf("write invocation: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO invocations
		(id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`,
		inv.ID,
		inv.FlowToken,
		string(inv.ActionURI),
		argsJSON,
		inv.Seq,
		secCtxJSON,
		inv.SpecHash,
		inv.EngineVersion,
		inv.IRVersion,
	)
	if err != nil {
		return fmt.Errorf("write invocation: %w", err)
	}

	return nil
}

// WriteCompletion inserts a completion record into the store.
// Uses ON CONFLICT DO NOTHING for idempotency - duplicate writes are silently ignored.
// Each invocation can have exactly ONE completion (enforced by UNIQUE constraint on invocation_id).
//
// The completion's Result and SecurityContext are serialized to canonical JSON
// per RFC 8785 for deterministic replay.
//
// Note: The invocation referenced by InvocationID must exist (foreign key constraint).
// Note: Attempting to write a second completion for an invocation will silently fail (idempotent).
func (s *Store) WriteCompletion(ctx context.Context, comp ir.Completion) error {
	resultJSON, err := marshalResult(comp.Result)
	if err != nil {
		return fmt.Errorf("write completion: %w", err)
	}

	secCtxJSON, err := marshalSecurityContext(comp.SecurityContext)
	if err != nil {
		return fmt.Errorf("write completion: %w", err)
	}

	// ON CONFLICT DO NOTHING handles both:
	// 1. Duplicate completion ID (same completion written twice)
	// 2. Duplicate invocation_id (second completion for same invocation)
	// Both are silently ignored for idempotency.
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO completions
		(id, invocation_id, output_case, result, seq, security_context)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT DO NOTHING
	`,
		comp.ID,
		comp.InvocationID,
		comp.OutputCase,
		resultJSON,
		comp.Seq,
		secCtxJSON,
	)
	if err != nil {
		return fmt.Errorf("write completion: %w", err)
	}

	return nil
}

// WriteSyncFiring inserts a sync firing record into the store.
// Returns the ID and whether a new record was inserted.
//
// Uses ON CONFLICT(completion_id, sync_id, binding_hash) DO NOTHING for idempotency
// per CP-1 (binding-level idempotency). If the firing already exists, returns the
// existing ID and inserted=false.
//
// Note: The completion referenced by CompletionID must exist (foreign key constraint).
func (s *Store) WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) (id int64, inserted bool, err error) {
	// Use a transaction to ensure atomicity of insert-or-select
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("write sync firing: begin tx: %w", err)
	}
	defer tx.Rollback() // No-op if committed

	// Try to insert
	result, err := tx.ExecContext(ctx, `
		INSERT INTO sync_firings
		(completion_id, sync_id, binding_hash, seq)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(completion_id, sync_id, binding_hash) DO NOTHING
	`,
		firing.CompletionID,
		firing.SyncID,
		firing.BindingHash,
		firing.Seq,
	)
	if err != nil {
		return 0, false, fmt.Errorf("write sync firing: insert: %w", err)
	}

	// Check if a row was actually inserted
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("write sync firing: rows affected: %w", err)
	}

	if rowsAffected > 0 {
		// New row inserted - get the auto-generated ID
		id, err = result.LastInsertId()
		if err != nil {
			return 0, false, fmt.Errorf("write sync firing: last insert id: %w", err)
		}
		inserted = true
	} else {
		// Conflict - row already exists, fetch the existing ID
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM sync_firings
			WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
		`, firing.CompletionID, firing.SyncID, firing.BindingHash).Scan(&id)
		if err != nil {
			return 0, false, fmt.Errorf("write sync firing: select existing: %w", err)
		}
		inserted = false
	}

	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("write sync firing: commit: %w", err)
	}

	return id, inserted, nil
}

// HasFiring checks if a sync firing already exists for the given triple.
// Used for idempotency checks per CP-1 (binding-level idempotency).
func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sync_firings
		WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
	`, completionID, syncID, bindingHash).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check firing: %w", err)
	}
	return count > 0, nil
}

// WriteProvenanceEdge inserts a provenance edge linking a sync firing to its generated invocation.
// Uses ON CONFLICT(sync_firing_id) DO NOTHING - each firing produces exactly one invocation.
//
// Note: Both sync_firing_id and invocation_id must exist (foreign key constraints).
func (s *Store) WriteProvenanceEdge(ctx context.Context, syncFiringID int64, invocationID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO provenance_edges
		(sync_firing_id, invocation_id)
		VALUES (?, ?)
		ON CONFLICT(sync_firing_id) DO NOTHING
	`,
		syncFiringID,
		invocationID,
	)
	if err != nil {
		return fmt.Errorf("write provenance edge: %w", err)
	}
	return nil
}

// WriteSyncFiringAtomic atomically writes a sync firing, invocation, and provenance edge
// in a single transaction. This ensures crash atomicity for CP-1.
//
// Returns:
//   - firingID: the ID of the sync firing (new or existing)
//   - inserted: true if this was a new firing, false if it already existed
//   - error: any error that occurred
//
// If inserted=false, the invocation and provenance edge are NOT written (sync already fired).
// This is the crash-safe variant of the non-atomic sequence:
// HasFiring → WriteInvocation → WriteSyncFiring → WriteProvenanceEdge
func (s *Store) WriteSyncFiringAtomic(
	ctx context.Context,
	firing ir.SyncFiring,
	inv ir.Invocation,
) (firingID int64, inserted bool, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Step 1: Try to insert firing (claims the slot atomically via unique constraint)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO sync_firings
		(completion_id, sync_id, binding_hash, seq)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(completion_id, sync_id, binding_hash) DO NOTHING
	`,
		firing.CompletionID,
		firing.SyncID,
		firing.BindingHash,
		firing.Seq,
	)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: insert firing: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Conflict - firing already exists, nothing more to do
		err = tx.QueryRowContext(ctx, `
			SELECT id FROM sync_firings
			WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
		`, firing.CompletionID, firing.SyncID, firing.BindingHash).Scan(&firingID)
		if err != nil {
			return 0, false, fmt.Errorf("atomic sync firing: select existing: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return 0, false, fmt.Errorf("atomic sync firing: commit (existing): %w", err)
		}
		return firingID, false, nil
	}

	// New firing inserted - get the ID
	firingID, err = result.LastInsertId()
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: last insert id: %w", err)
	}

	// Step 2: Marshal and write invocation
	argsJSON, err := marshalArgs(inv.Args)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: marshal args: %w", err)
	}

	secCtxJSON, err := marshalSecurityContext(inv.SecurityContext)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: marshal security context: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO invocations
		(id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`,
		inv.ID,
		inv.FlowToken,
		string(inv.ActionURI),
		argsJSON,
		inv.Seq,
		secCtxJSON,
		inv.SpecHash,
		inv.EngineVersion,
		inv.IRVersion,
	)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: write invocation: %w", err)
	}

	// Step 3: Write provenance edge
	_, err = tx.ExecContext(ctx, `
		INSERT INTO provenance_edges
		(sync_firing_id, invocation_id)
		VALUES (?, ?)
		ON CONFLICT(sync_firing_id) DO NOTHING
	`,
		firingID,
		inv.ID,
	)
	if err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: write provenance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, false, fmt.Errorf("atomic sync firing: commit: %w", err)
	}

	return firingID, true, nil
}
