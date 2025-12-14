-- NYSM Event Log Schema
--
-- CRITICAL PATTERNS:
-- - CP-2: Use seq INTEGER for logical clocks, NEVER timestamps
-- - CP-1: UNIQUE(completion_id, sync_id, binding_hash) for binding-level idempotency
-- - CP-4: All queries MUST have ORDER BY seq ASC, id ASC COLLATE BINARY
-- - CP-6: SecurityContext always present (JSON column, never NULL)
--
-- All content-addressed IDs are TEXT (hex-encoded SHA-256 hashes).
-- All JSON columns store canonical JSON per RFC 8785.

-- Invocations: Action invocation records
CREATE TABLE IF NOT EXISTS invocations (
    id TEXT PRIMARY KEY,              -- Content-addressed hash via ir.InvocationID()
    flow_token TEXT NOT NULL,         -- UUIDv7 flow token (groups related invocations)
    action_uri TEXT NOT NULL,         -- ActionRef (e.g., "Cart.addItem")
    args TEXT NOT NULL,               -- Canonical JSON (IRObject)
    seq INTEGER NOT NULL,             -- Logical clock (monotonic, per CP-2)
    security_context TEXT NOT NULL,   -- JSON (SecurityContext, always present per CP-6)
    spec_hash TEXT NOT NULL,          -- Hash of concept spec at invoke time
    engine_version TEXT NOT NULL,     -- Engine version string
    ir_version TEXT NOT NULL          -- IR schema version
);

CREATE INDEX IF NOT EXISTS idx_invocations_flow_token
    ON invocations(flow_token);
CREATE INDEX IF NOT EXISTS idx_invocations_seq
    ON invocations(seq);

-- Completions: Action completion records
-- CRITICAL: Each invocation has exactly ONE completion (enforced by UNIQUE constraint).
-- This is required for deterministic replay - multiple completions would break replay.
CREATE TABLE IF NOT EXISTS completions (
    id TEXT PRIMARY KEY,              -- Content-addressed hash via ir.CompletionID()
    invocation_id TEXT NOT NULL UNIQUE REFERENCES invocations(id),
    output_case TEXT NOT NULL,        -- OutputCase name ("Success", error variant)
    result TEXT NOT NULL,             -- Canonical JSON (IRObject)
    seq INTEGER NOT NULL,             -- Logical clock (per CP-2)
    security_context TEXT NOT NULL    -- JSON (SecurityContext, always present per CP-6)
);

-- Note: UNIQUE(invocation_id) above creates an implicit index, but explicit index ensures
-- correct query planning for various access patterns.
CREATE INDEX IF NOT EXISTS idx_completions_invocation
    ON completions(invocation_id);
CREATE INDEX IF NOT EXISTS idx_completions_seq
    ON completions(seq);
-- Composite index for flow token queries (via invocations join)
CREATE INDEX IF NOT EXISTS idx_completions_invocation_seq
    ON completions(invocation_id, seq);

-- Sync Firings: Track each sync rule firing per binding
-- CRITICAL: UNIQUE(completion_id, sync_id, binding_hash) implements CP-1
CREATE TABLE IF NOT EXISTS sync_firings (
    id INTEGER PRIMARY KEY,           -- Auto-increment (store FK only)
    completion_id TEXT NOT NULL REFERENCES completions(id),
    sync_id TEXT NOT NULL,            -- Sync rule identifier
    binding_hash TEXT NOT NULL,       -- Hash of binding values via ir.BindingHash()
    seq INTEGER NOT NULL,             -- Logical clock (per CP-2)
    UNIQUE(completion_id, sync_id, binding_hash)  -- Binding-level idempotency (CP-1)
);

CREATE INDEX IF NOT EXISTS idx_sync_firings_completion
    ON sync_firings(completion_id);
CREATE INDEX IF NOT EXISTS idx_sync_firings_seq
    ON sync_firings(seq);

-- Provenance Edges: Link sync firings to generated invocations
CREATE TABLE IF NOT EXISTS provenance_edges (
    id INTEGER PRIMARY KEY,           -- Auto-increment (store FK only)
    sync_firing_id INTEGER NOT NULL REFERENCES sync_firings(id),
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    UNIQUE(sync_firing_id)            -- Each firing produces exactly one invocation
);

CREATE INDEX IF NOT EXISTS idx_provenance_invocation
    ON provenance_edges(invocation_id);
