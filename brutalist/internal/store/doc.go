// Package store provides SQLite-backed durable storage for NYSM event logs.
//
// The store implements an append-only log with:
//   - Invocations: Action invocation records
//   - Completions: Action completion records
//   - Sync Firings: Sync rule firing records (with binding-level idempotency)
//   - Provenance Edges: Causality links (completion → sync → invocation)
//
// # Critical Patterns
//
// CP-1: Binding-Level Idempotency
//   - UNIQUE(completion_id, sync_id, binding_hash) constraint
//   - Prevents duplicate firings for same binding values
//
// CP-2: Logical Identity and Time
//   - All ordering uses seq INTEGER (logical clock), NEVER timestamps
//   - Enables deterministic replay regardless of wall time
//
// CP-4: Deterministic Query Results
//   - All queries MUST include: ORDER BY seq ASC, id ASC COLLATE BINARY
//   - Ensures identical results across replays
//
// CP-6: Security Context Always Present
//   - security_context column NEVER NULL
//   - Stored as JSON for audit trail
//
// # Database Configuration
//
//   - WAL mode: Concurrent reads during writes
//   - synchronous=NORMAL: Balance durability/performance
//   - busy_timeout=5000: Wait for locks up to 5 seconds
//   - foreign_keys=ON: Enforce referential integrity
//
// All content-addressed IDs are computed via functions in internal/ir/hash.go
// using RFC 8785 canonical JSON and SHA-256 with domain separation.
package store
