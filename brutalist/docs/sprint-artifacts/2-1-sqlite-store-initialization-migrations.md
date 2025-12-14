# Story 2.1: SQLite Store Initialization & Migrations

Status: done

## Story

As a **developer running NYSM**,
I want **a SQLite store that initializes with the correct schema**,
So that **the event log is ready for use**.

## Acceptance Criteria

1. **Store initialization in `internal/store/store.go`**
   - `Open(path string) (*Store, error)` function creates/opens database
   - Returns configured Store instance ready for use
   - Database file created if it doesn't exist

2. **WAL mode and required pragmas set**
   ```sql
   PRAGMA journal_mode = WAL;     -- Concurrent reads during writes
   PRAGMA synchronous = NORMAL;   -- Balance durability/performance
   PRAGMA busy_timeout = 5000;    -- Wait for locks (milliseconds)
   PRAGMA foreign_keys = ON;      -- Enforce referential integrity
   ```

3. **Schema embedded via `go:embed`**
   ```go
   //go:embed schema.sql
   var schemaSQL string
   ```
   - Schema file at `internal/store/schema.sql`
   - Applied automatically on first connection

4. **Migrations are idempotent**
   - Safe to run `Open()` multiple times on same database
   - No errors when schema already exists
   - Uses `CREATE TABLE IF NOT EXISTS`

5. **Store struct provides clean interface**
   ```go
   type Store struct {
       db *sql.DB
   }

   func (s *Store) Close() error {
       return s.db.Close()
   }
   ```

6. **Error handling for common failure modes**
   - Invalid path (permission denied, etc.)
   - Corrupted database file
   - Pragma application failures

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **FR-5.1** | SQLite-backed append-only log |
| **CP-2** | Logical clocks, no timestamps |

## Tasks / Subtasks

- [ ] Task 1: Create store package structure (AC: #1)
  - [ ] 1.1 Create `internal/store/` directory
  - [ ] 1.2 Create `internal/store/doc.go` with package documentation
  - [ ] 1.3 Create `internal/store/store.go` with Store struct

- [ ] Task 2: Implement Store.Open() (AC: #1, #2, #6)
  - [ ] 2.1 Implement `Open(path string) (*Store, error)` function
  - [ ] 2.2 Open database with `database/sql` and `mattn/go-sqlite3` driver
  - [ ] 2.3 Apply all PRAGMA settings
  - [ ] 2.4 Return configured Store instance
  - [ ] 2.5 Add error handling for common failures

- [ ] Task 3: Create embedded schema (AC: #3, #4)
  - [ ] 3.1 Create `internal/store/schema.sql` file
  - [ ] 3.2 Add initial tables (invocations, completions, sync_firings, provenance_edges)
  - [ ] 3.3 Use `CREATE TABLE IF NOT EXISTS` for idempotency
  - [ ] 3.4 Embed schema via `//go:embed schema.sql`

- [ ] Task 4: Implement schema migration (AC: #4)
  - [ ] 4.1 Create `applySchema(db *sql.DB) error` function
  - [ ] 4.2 Execute embedded schema SQL
  - [ ] 4.3 Call from Open() after pragma setup

- [ ] Task 5: Implement Store.Close() (AC: #5)
  - [ ] 5.1 Add Close() method that closes underlying sql.DB

- [ ] Task 6: Write comprehensive tests (all AC)
  - [ ] 6.1 Test successful database creation
  - [ ] 6.2 Test idempotent Open() (multiple calls)
  - [ ] 6.3 Test pragma settings applied correctly
  - [ ] 6.4 Test schema tables exist after Open()
  - [ ] 6.5 Test error handling (bad path, permissions, etc.)
  - [ ] 6.6 Test Close() cleans up resources

## Dev Notes

### Store Implementation

```go
// internal/store/store.go
package store

import (
    "database/sql"
    _ "embed"
    "fmt"

    _ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// Store provides durable storage for NYSM event logs.
// Uses SQLite with WAL mode for concurrent read access.
type Store struct {
    db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
// Applies required pragmas and migrations automatically.
//
// The database is configured with:
// - WAL mode for concurrent reads during writes
// - NORMAL synchronous mode (balance durability/performance)
// - 5-second busy timeout for lock contention
// - Foreign key enforcement
//
// This function is idempotent - safe to call multiple times.
func Open(path string) (*Store, error) {
    // Open database (creates file if doesn't exist)
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Apply required pragmas
    if err := applyPragmas(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to apply pragmas: %w", err)
    }

    // Apply schema migrations
    if err := applySchema(db); err != nil {
        db.Close()
        return nil, fmt.Errorf("failed to apply schema: %w", err)
    }

    return &Store{db: db}, nil
}

// Close closes the database connection.
// Should be called when the store is no longer needed.
func (s *Store) Close() error {
    if s.db == nil {
        return nil
    }
    return s.db.Close()
}

// applyPragmas sets required SQLite configuration.
func applyPragmas(db *sql.DB) error {
    pragmas := []string{
        "PRAGMA journal_mode = WAL",
        "PRAGMA synchronous = NORMAL",
        "PRAGMA busy_timeout = 5000",
        "PRAGMA foreign_keys = ON",
    }

    for _, pragma := range pragmas {
        if _, err := db.Exec(pragma); err != nil {
            return fmt.Errorf("failed to execute %q: %w", pragma, err)
        }
    }

    return nil
}

// applySchema creates tables if they don't exist.
// This function is idempotent.
func applySchema(db *sql.DB) error {
    if _, err := db.Exec(schemaSQL); err != nil {
        return fmt.Errorf("failed to execute schema: %w", err)
    }
    return nil
}

// verifyPragma checks that a pragma is set to the expected value.
// Used for testing.
func (s *Store) verifyPragma(name, expected string) error {
    var value string
    query := fmt.Sprintf("PRAGMA %s", name)
    if err := s.db.QueryRow(query).Scan(&value); err != nil {
        return fmt.Errorf("failed to query %s: %w", name, err)
    }
    if value != expected {
        return fmt.Errorf("%s = %q, expected %q", name, value, expected)
    }
    return nil
}
```

### Initial Schema

```sql
-- internal/store/schema.sql
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
CREATE TABLE IF NOT EXISTS completions (
    id TEXT PRIMARY KEY,              -- Content-addressed hash via ir.CompletionID()
    invocation_id TEXT NOT NULL REFERENCES invocations(id),
    output_case TEXT NOT NULL,        -- OutputCase name ("Success", error variant)
    result TEXT NOT NULL,             -- Canonical JSON (IRObject)
    seq INTEGER NOT NULL,             -- Logical clock (per CP-2)
    security_context TEXT NOT NULL    -- JSON (SecurityContext, always present per CP-6)
);

CREATE INDEX IF NOT EXISTS idx_completions_invocation
    ON completions(invocation_id);
CREATE INDEX IF NOT EXISTS idx_completions_seq
    ON completions(seq);

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
```

### Package Documentation

```go
// internal/store/doc.go
// Package store provides SQLite-backed durable storage for NYSM event logs.
//
// The store implements an append-only log with:
// - Invocations: Action invocation records
// - Completions: Action completion records
// - Sync Firings: Sync rule firing records (with binding-level idempotency)
// - Provenance Edges: Causality links (completion → sync → invocation)
//
// CRITICAL PATTERNS:
//
// CP-1: Binding-Level Idempotency
// - UNIQUE(completion_id, sync_id, binding_hash) constraint
// - Prevents duplicate firings for same binding values
//
// CP-2: Logical Identity and Time
// - All ordering uses seq INTEGER (logical clock), NEVER timestamps
// - Enables deterministic replay regardless of wall time
//
// CP-4: Deterministic Query Results
// - All queries MUST include: ORDER BY seq ASC, id ASC COLLATE BINARY
// - Ensures identical results across replays
//
// CP-6: Security Context Always Present
// - security_context column NEVER NULL
// - Stored as JSON for audit trail
//
// Database Configuration:
// - WAL mode: Concurrent reads during writes
// - synchronous=NORMAL: Balance durability/performance
// - busy_timeout=5000: Wait for locks up to 5 seconds
// - foreign_keys=ON: Enforce referential integrity
//
// All content-addressed IDs are computed via functions in internal/ir/hash.go
// using RFC 8785 canonical JSON and SHA-256 with domain separation.
package store
```

### Test Examples

```go
// internal/store/store_test.go
package store

import (
    "os"
    "path/filepath"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestOpen_CreatesDatabase(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    // Verify file exists
    _, err = os.Stat(dbPath)
    assert.NoError(t, err, "database file should exist")
}

func TestOpen_AppliesPragmas(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    tests := []struct {
        name     string
        expected string
    }{
        {"journal_mode", "wal"},
        {"synchronous", "1"},      // NORMAL = 1
        {"foreign_keys", "1"},     // ON = 1
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := store.verifyPragma(tt.name, tt.expected)
            assert.NoError(t, err)
        })
    }
}

func TestOpen_CreatesSchema(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    // Verify tables exist
    tables := []string{
        "invocations",
        "completions",
        "sync_firings",
        "provenance_edges",
    }

    for _, table := range tables {
        t.Run(table, func(t *testing.T) {
            var name string
            query := `SELECT name FROM sqlite_master WHERE type='table' AND name=?`
            err := store.db.QueryRow(query, table).Scan(&name)
            require.NoError(t, err, "table %s should exist", table)
            assert.Equal(t, table, name)
        })
    }
}

func TestOpen_Idempotent(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    // First open
    store1, err := Open(dbPath)
    require.NoError(t, err)
    require.NoError(t, store1.Close())

    // Second open (should not error)
    store2, err := Open(dbPath)
    require.NoError(t, err)
    defer store2.Close()

    // Verify schema still intact
    var count int
    query := `SELECT COUNT(*) FROM sqlite_master WHERE type='table'`
    err = store2.db.QueryRow(query).Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 4, count, "should have 4 tables")
}

func TestOpen_ErrorHandling(t *testing.T) {
    t.Run("invalid_path", func(t *testing.T) {
        // Path with invalid directory
        dbPath := "/nonexistent/path/test.db"
        _, err := Open(dbPath)
        assert.Error(t, err)
    })

    t.Run("close_nil_db", func(t *testing.T) {
        store := &Store{db: nil}
        err := store.Close()
        assert.NoError(t, err, "closing nil db should not error")
    })
}

func TestSchema_InvocationsTable(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    // Verify columns exist
    query := `PRAGMA table_info(invocations)`
    rows, err := store.db.Query(query)
    require.NoError(t, err)
    defer rows.Close()

    expectedCols := map[string]bool{
        "id":               false,
        "flow_token":       false,
        "action_uri":       false,
        "args":             false,
        "seq":              false,
        "security_context": false,
        "spec_hash":        false,
        "engine_version":   false,
        "ir_version":       false,
    }

    for rows.Next() {
        var cid int
        var name, ctype string
        var notnull, pk int
        var dfltValue *string
        err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk)
        require.NoError(t, err)

        if _, ok := expectedCols[name]; ok {
            expectedCols[name] = true

            // Verify NOT NULL columns
            if name != "id" {
                assert.Equal(t, 1, notnull, "column %s should be NOT NULL", name)
            }
        }
    }

    // Verify all expected columns found
    for col, found := range expectedCols {
        assert.True(t, found, "column %s should exist", col)
    }
}

func TestSchema_SyncFiringsUniqueness(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    // Verify UNIQUE constraint exists on (completion_id, sync_id, binding_hash)
    query := `
        SELECT sql FROM sqlite_master
        WHERE type='table' AND name='sync_firings'
    `
    var sql string
    err = store.db.QueryRow(query).Scan(&sql)
    require.NoError(t, err)

    assert.Contains(t, sql, "UNIQUE(completion_id, sync_id, binding_hash)",
        "sync_firings should have UNIQUE constraint per CP-1")
}

func TestSchema_IndexesCreated(t *testing.T) {
    dir := t.TempDir()
    dbPath := filepath.Join(dir, "test.db")

    store, err := Open(dbPath)
    require.NoError(t, err)
    defer store.Close()

    expectedIndexes := []string{
        "idx_invocations_flow_token",
        "idx_invocations_seq",
        "idx_completions_invocation",
        "idx_completions_seq",
        "idx_sync_firings_completion",
        "idx_sync_firings_seq",
        "idx_provenance_invocation",
    }

    for _, idx := range expectedIndexes {
        t.Run(idx, func(t *testing.T) {
            var name string
            query := `SELECT name FROM sqlite_master WHERE type='index' AND name=?`
            err := store.db.QueryRow(query, idx).Scan(&name)
            require.NoError(t, err, "index %s should exist", idx)
        })
    }
}
```

### File List

Files to create:

1. `internal/store/doc.go` - Package documentation
2. `internal/store/store.go` - Store struct and Open/Close functions
3. `internal/store/schema.sql` - Initial schema definition
4. `internal/store/store_test.go` - Comprehensive tests

### Relationship to Other Stories

**Dependencies:**
- Story 1.1 (Project Initialization & IR Type Definitions) - Required for go.mod with sqlite3 dependency

**Enables:**
- Story 2.2 (Event Log Schema with Logical Clocks) - Builds on this schema foundation
- Story 2.3 (Write Invocations and Completions) - Uses Store.Open()
- Story 2.4 (Read Flow and Query Support) - Uses Store struct
- Story 2.5 (Sync Firings Table) - Already defined in schema here
- Story 2.6 (Provenance Edges Table) - Already defined in schema here
- Story 2.7 (Crash Recovery and Replay) - Uses Store for replay

**Note:** Stories 2.2 through 2.6 will ADD CRUD operations for the tables defined here. This story establishes the foundation.

### Story Completion Checklist

- [ ] `internal/store/` directory created
- [ ] `internal/store/doc.go` written with package documentation
- [ ] `internal/store/store.go` implements Open() and Close()
- [ ] `internal/store/schema.sql` defines all 4 tables with indexes
- [ ] All pragmas applied correctly (WAL, NORMAL, busy_timeout, foreign_keys)
- [ ] Schema embedded via `//go:embed`
- [ ] Open() is idempotent (multiple calls work)
- [ ] All tests pass (`go test ./internal/store/...`)
- [ ] `go build ./...` succeeds
- [ ] `go vet ./internal/store` passes
- [ ] Pragma verification tests pass
- [ ] Schema table existence tests pass
- [ ] Index existence tests pass
- [ ] UNIQUE constraint verified for sync_firings
- [ ] Error handling tested (invalid paths, etc.)

### References

- [Source: docs/architecture.md#SQLite Configuration] - Pragma settings
- [Source: docs/architecture.md#Project Structure] - File locations
- [Source: docs/architecture.md#Technology Stack] - SQLite version v1.14.32
- [Source: docs/epics.md#Story 2.1] - Acceptance criteria
- [Source: docs/epics.md#Story 2.2] - Schema details with logical clocks
- [Source: docs/epics.md#Story 2.5] - Sync firings UNIQUE constraint (CP-1)
- [Source: docs/architecture.md#CP-2] - Logical clocks instead of timestamps
- [Source: docs/architecture.md#CP-1] - Binding-level idempotency
- [Source: docs/prd.md#FR-5.1] - SQLite-backed append-only log requirement

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow

### Completion Notes

- Schema includes all 4 core tables (invocations, completions, sync_firings, provenance_edges)
- WAL mode enables concurrent reads during sync engine execution
- UNIQUE constraint on sync_firings implements CP-1 (binding-level idempotency)
- All seq columns use INTEGER for logical clocks (CP-2)
- Schema uses `CREATE TABLE IF NOT EXISTS` for idempotent migrations
- security_context columns are NOT NULL per CP-6
- All indexes created for query performance
- Store package has no complex logic - just initialization
- Next stories (2.2-2.6) will add CRUD operations using this foundation
