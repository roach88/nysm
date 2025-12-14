// Package engine implements the NYSM sync engine.
//
// # Replay and Idempotency
//
// This file documents how replay works and why it produces identical results.
//
// ## Structural Idempotency
//
// Idempotency in NYSM is STRUCTURAL, not a special "replay mode".
// The same code path handles both initial execution and replay.
//
// Three mechanisms enforce idempotency:
//
// 1. Database Constraint
//
//	UNIQUE(completion_id, sync_id, binding_hash)
//
// Prevents duplicate (completion, sync, binding) triples at write time.
//
// 2. Atomic Write with Check
//
//	WriteSyncFiringAtomic returns inserted=false if firing already exists.
//	No separate HasFiring check needed - the atomic operation handles both.
//
// 3. Content-Addressed Binding Hash
//
//	bindingHash := ir.BindingHash(binding)
//
// Same binding values always produce the same hash via RFC 8785 canonical JSON.
//
// ## Replay Flow
//
// Normal execution and replay follow the identical path:
//
//	[Completion] → [Match Syncs] → [Execute Where] → [Bindings]
//	                                                     ↓
//	                                         [For each binding]
//	                                                     ↓
//	                                         [WriteSyncFiringAtomic]
//	                                                     ↓
//	                                  inserted=true → Enqueue invocation
//	                                  inserted=false → Skip (already processed)
//
// ## Why Replay is Safe
//
// Consider a sync rule that fires 3 invocations from a cart checkout:
//
//	Scenario: Cart with 3 items, crash after 2 fired
//
//	Before Crash:
//	  [Cart.checkout completed] → Where-clause returns bindings B1, B2, B3
//	                            → WriteSyncFiringAtomic(B1) → inserted=true → Enqueue
//	                            → WriteSyncFiringAtomic(B2) → inserted=true → Enqueue
//	                            → CRASH before B3
//
//	After Replay:
//	  [Cart.checkout completed] → Where-clause returns bindings B1, B2, B3
//	                            → WriteSyncFiringAtomic(B1) → inserted=false → Skip
//	                            → WriteSyncFiringAtomic(B2) → inserted=false → Skip
//	                            → WriteSyncFiringAtomic(B3) → inserted=true → Enqueue
//
//	Result: Only B3 fires. Final state identical to non-crash run.
//
// ## Content-Addressed IDs
//
// Content-addressed IDs ensure same inputs → same outputs:
//
//   - BindingHash uses RFC 8785 canonical JSON (sorted keys, no whitespace)
//   - InvocationID includes flow token, action, args, and seq
//   - Same computation always produces same ID
//
// This means replay can safely be run multiple times:
//
//	for i := 0; i < 100; i++ {
//	    engine.ProcessCompletion(ctx, completion)
//	}
//	// Final state is identical to running once
//
// ## Crash Safety
//
// WriteSyncFiringAtomic writes firing, invocation, and provenance edge
// in a single transaction. If any write fails, none persist.
//
// This prevents the scenario where:
//   - Firing is written
//   - CRASH before invocation written
//   - Replay sees firing exists → skips
//   - Invocation never created!
//
// With atomic writes, either all 3 records exist or none do.
//
// ## Key Functions
//
//   - executeThen: Uses WriteSyncFiringAtomic for crash-safe idempotent writes
//   - store.WriteSyncFiringAtomic: Atomic write with duplicate detection
//   - ir.BindingHash: Deterministic hash via canonical JSON
//   - store.FindIncompleteFlows: Identifies flows needing recovery
//   - store.ReplayFlow: Returns events for explicit replay
//
// ## References
//
//   - CP-1: Binding-Level Idempotency (UNIQUE constraint on binding hash)
//   - CP-2: Logical Identity and Time (seq numbers, no wall clocks)
//   - CP-3: RFC 8785 Canonical JSON (deterministic serialization)
//   - CP-4: Deterministic Ordering (ORDER BY seq ASC, id COLLATE BINARY ASC)
//   - FR-4.2: Support idempotency check via sync edges
//   - FR-4.3: Enable crash/restart replay with identical results
package engine
