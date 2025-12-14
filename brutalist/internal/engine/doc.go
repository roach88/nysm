// Package engine implements the NYSM reactive sync engine.
//
// The engine is the heart of NYSM - it receives invocations and completions,
// matches sync rules, executes queries, and generates follow-on invocations.
//
// ARCHITECTURE:
//
// Single-Writer Event Loop:
// The engine processes all events in a single goroutine for deterministic
// behavior. This ensures:
// - Predictable sync rule evaluation order
// - Reproducible event log on replay
// - Simple reasoning about causality
//
// Event Processing Flow:
// 1. Events enqueued to FIFO queue (invocations or completions)
// 2. Engine.Run() dequeues events one at a time
// 3. processEvent() routes to appropriate handler
// 4. Handler writes to SQLite (single writer)
// 5. For completions: sync rules evaluated, generated invocations written to store
//
// Note: Generated invocations are written directly to the store (not re-enqueued).
// External action executors poll the store for pending invocations.
//
// The engine is designed for correctness and determinism, not throughput.
// External action execution may be parallelized, but the core evaluation
// loop is strictly single-threaded.
//
// CRITICAL PATTERNS:
//
// CP-2: Logical Clock
// All events stamped with monotonic seq counter from Clock.Next().
// NEVER use wall-clock timestamps for ordering.
//
// CRITICAL-3: Deterministic Scheduling
// Sync rules evaluated in declaration order.
// Query results processed in ORDER BY seq, id order.
// No randomness, no concurrency, no non-determinism.
package engine
