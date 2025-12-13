# Story 2.5: Sync Firings Table with Binding Hash

Status: ready-for-dev

## Story

As a **developer building idempotency**,
I want **a sync_firings table that tracks each binding separately**,
So that **multi-binding syncs work correctly**.

## Acceptance Criteria

1. **sync_firings table schema implements CP-1**
   - Table created with correct columns: id, completion_id, sync_id, binding_hash, seq
   - PRIMARY KEY on id (auto-increment for FK references)
   - FOREIGN KEY to completions(id) with ON DELETE CASCADE
   - UNIQUE(completion_id, sync_id, binding_hash) - NOT just UNIQUE(completion_id, sync_id)
   - seq column is INTEGER NOT NULL (logical clock, not timestamp)

2. **Index on completion_id for efficient lookups**
   ```sql
   CREATE INDEX idx_sync_firings_completion ON sync_firings(completion_id);
   ```

3. **WriteSyncFiring function in internal/store/store.go**
   ```go
   func (s *Store) WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) error {
       // INSERT OR IGNORE for graceful idempotency handling
       // Returns no error if duplicate exists (idempotent write)
   }
   ```

4. **HasFiring function for idempotency checks**
   ```go
   func (s *Store) HasFiring(ctx context.Context, completionID, syncID, bindingHash string) (bool, error) {
       // Queries: SELECT COUNT(*) FROM sync_firings WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
       // Returns true if count > 0, false otherwise
   }
   ```

5. **binding_hash computed via ir.BindingHash()**
   - Hash computed using canonical JSON encoding per CP-3
   - Domain separation: "nysm/binding/v1"
   - Hash format: sha256(domain + "\x00" + canonical_json(bindings))
   - Function signature: `func BindingHash(bindings IRObject) (string, error)`
   - Use `MustBindingHash` in tests when inputs are known to be valid

6. **Migration adds table to schema**
   - Add CREATE TABLE statement to migration file
   - Add CREATE INDEX statement
   - Migration runs successfully on new and existing databases

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-1** | Binding-level idempotency: `UNIQUE(completion_id, sync_id, binding_hash)` |
| **CP-2** | Logical clocks (`seq`), NO wall-clock timestamps ever |
| **CP-3** | RFC 8785 UTF-16 code unit key ordering (not UTF-8 bytes) |
| **CP-4** | Content-addressed hash with domain separation |

## Tasks / Subtasks

- [ ] Task 1: Add binding_hash to IR types (AC: #5)
  - [ ] 1.1 Add BindingHash function to `internal/ir/hash.go`
  - [ ] 1.2 Implement canonical JSON encoding with domain separation
  - [ ] 1.3 Add tests for BindingHash determinism
  - [ ] 1.4 Verify empty bindings produce valid hash

- [ ] Task 2: Create sync_firings table migration (AC: #1, #2, #6)
  - [ ] 2.1 Add CREATE TABLE statement with all columns
  - [ ] 2.2 Add FOREIGN KEY constraint to completions(id)
  - [ ] 2.3 Add UNIQUE constraint on (completion_id, sync_id, binding_hash)
  - [ ] 2.4 Add INDEX on completion_id
  - [ ] 2.5 Verify migration runs on clean database
  - [ ] 2.6 Verify migration runs on database with existing tables

- [ ] Task 3: Implement WriteSyncFiring function (AC: #3)
  - [ ] 3.1 Add WriteSyncFiring to Store interface
  - [ ] 3.2 Implement INSERT OR IGNORE for idempotent writes
  - [ ] 3.3 Return id of inserted or existing firing
  - [ ] 3.4 Add error handling for constraint violations (should never occur with OR IGNORE)
  - [ ] 3.5 Add unit tests for successful write
  - [ ] 3.6 Add unit tests for duplicate write (verify idempotency)

- [ ] Task 4: Implement HasFiring function (AC: #4)
  - [ ] 4.1 Add HasFiring to Store interface
  - [ ] 4.2 Implement SELECT COUNT(*) query
  - [ ] 4.3 Use parameterized query (no string interpolation)
  - [ ] 4.4 Add unit tests for existing firing
  - [ ] 4.5 Add unit tests for non-existing firing
  - [ ] 4.6 Add unit tests for partial matches (different binding_hash)

- [ ] Task 5: Verify and test (AC: all)
  - [ ] 5.1 Integration test: write same firing twice (verify idempotency)
  - [ ] 5.2 Integration test: write multiple bindings for same (completion, sync)
  - [ ] 5.3 Integration test: verify UNIQUE constraint prevents duplicates
  - [ ] 5.4 Integration test: verify foreign key cascade on completion delete
  - [ ] 5.5 Performance test: index improves lookup speed
  - [ ] 5.6 Verify all tests pass with `go test ./internal/store/...`

## Dev Notes

### Critical Pattern Details

**CP-1: Binding-Level Idempotency**

The binding_hash is **CRITICAL** to correct sync rule semantics. Without it, NYSM breaks for any sync rule that produces multiple bindings.

**Why Binding-Level Granularity is Required:**

Consider this sync rule:
```cue
sync "reserve-each-item" {
  when: Cart.checkout.completed {
    bind: { cart_id: result.cart_id }
  }
  where: CartItems {
    filter: "cart_id = bound.cart_id"
    bind: { item_id: item_id, quantity: quantity }
  }
  then: Inventory.reserve(item: bound.item_id, qty: bound.quantity)
}
```

**Scenario:** Cart contains 3 items.

**Expected Behavior:** The `where` clause returns 3 bindings, one for each item. The sync should fire 3 times, producing 3 `Inventory.reserve` invocations.

**Without binding_hash (sync-level idempotency only):**
```sql
-- BROKEN SCHEMA:
UNIQUE(completion_id, sync_id)  -- ❌ WRONG
```

With this schema:
1. First binding fires → sync_firing inserted ✓
2. Second binding fires → INSERT fails (duplicate completion_id, sync_id) ❌
3. Third binding fires → INSERT fails ❌
4. **Result:** Only 1 of 3 items reserved. Cart checkout succeeds but inventory is corrupted.

**With binding_hash (binding-level idempotency):**
```sql
-- CORRECT SCHEMA:
UNIQUE(completion_id, sync_id, binding_hash)  -- ✓ CORRECT
```

With this schema:
1. First binding fires → (completion, sync, hash_1) inserted ✓
2. Second binding fires → (completion, sync, hash_2) inserted ✓
3. Third binding fires → (completion, sync, hash_3) inserted ✓
4. **Result:** All 3 items reserved correctly. Cart checkout and inventory both consistent.

**Replay Scenario:**

After a crash, the engine replays the Cart.checkout completion:
1. Where-clause re-executes → produces same 3 bindings (deterministic query ordering)
2. Binding hashes re-computed → produce same hash values (canonical JSON)
3. HasFiring checks → all return true (already fired)
4. All 3 bindings skipped (idempotent replay)
5. **Result:** Replay produces identical state. No duplicate reservations.

**Key Insight:** The (completion_id, sync_id, binding_hash) triple uniquely identifies a "should we fire?" decision. Without binding_hash, multiple bindings from the same where-clause collapse into a single firing, breaking the fundamental `when→where→then` semantics.

### Binding Hash Implementation

**Function Signature:**
```go
// internal/ir/hash.go

// BindingHash computes a content-addressed hash of binding values.
// Uses canonical JSON encoding per CP-3 and domain separation per CP-4.
// Deterministic: identical bindings always produce identical hashes.
// Returns error if bindings cannot be canonically marshaled.
func BindingHash(bindings IRObject) (string, error) {
    canonical, err := MarshalCanonical(bindings)
    if err != nil {
        return "", fmt.Errorf("BindingHash: failed to marshal: %w", err)
    }
    return hashWithDomain("nysm/binding/v1", canonical), nil
}

// MustBindingHash is like BindingHash but panics on error.
// Use only in tests or when inputs are known to be valid.
func MustBindingHash(bindings IRObject) string {
    hash, err := BindingHash(bindings)
    if err != nil {
        panic(err)
    }
    return hash
}

// hashWithDomain computes SHA-256 hash with domain separation.
// Format: sha256(domain + "\x00" + data)
func hashWithDomain(domain string, data []byte) string {
    h := sha256.New()
    h.Write([]byte(domain))
    h.Write([]byte{0x00}) // Null separator
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}
```

**Canonical JSON Encoding (CP-3):**
```go
// MarshalCanonical produces RFC 8785 canonical JSON.
// Key requirements:
// 1. Keys sorted by UTF-16 code units (NOT UTF-8 bytes)
// 2. No whitespace
// 3. Minimal encoding (no escaped characters unless required)
func MarshalCanonical(v IRValue) ([]byte, error) {
    // Implementation in Story 1.4 (RFC 8785 Canonical JSON Marshaling)
    // CRITICAL: Use UTF-16 comparison, not Go's UTF-8 sort.Strings
}
```

**Domain Separation:**
Domain prefixes prevent hash collisions across different value types:
- `"nysm/invocation/v1"` - Invocation IDs
- `"nysm/completion/v1"` - Completion IDs
- `"nysm/binding/v1"` - Binding hashes

Without domain separation, these could collide:
```go
// Invocation with args: {"x": 1}
inv_hash = sha256(canonical({"x": 1}))

// Binding with values: {"x": 1}
binding_hash = sha256(canonical({"x": 1}))

// inv_hash == binding_hash  // ❌ Collision!
```

With domain separation:
```go
inv_hash = sha256("nysm/invocation/v1\x00" + canonical({"x": 1}))
binding_hash = sha256("nysm/binding/v1\x00" + canonical({"x": 1}))
// inv_hash != binding_hash  // ✓ No collision
```

### Schema Definition

**Complete Table Creation:**
```sql
-- Migration file: internal/store/migrations/003_sync_firings.sql

CREATE TABLE sync_firings (
    id INTEGER PRIMARY KEY,
    completion_id TEXT NOT NULL REFERENCES completions(id) ON DELETE CASCADE,
    sync_id TEXT NOT NULL,
    binding_hash TEXT NOT NULL,
    seq INTEGER NOT NULL,
    UNIQUE(completion_id, sync_id, binding_hash)
);

CREATE INDEX idx_sync_firings_completion ON sync_firings(completion_id);
```

**Column Details:**

| Column | Type | Constraints | Purpose |
|--------|------|-------------|---------|
| id | INTEGER | PRIMARY KEY | Auto-increment for FK references in provenance_edges |
| completion_id | TEXT | NOT NULL, FK to completions(id) | Which completion triggered this firing |
| sync_id | TEXT | NOT NULL | Which sync rule fired (stable identifier) |
| binding_hash | TEXT | NOT NULL | Hash of binding values from where-clause |
| seq | INTEGER | NOT NULL | Logical clock value (CP-2) |

**Why ON DELETE CASCADE:**
If a completion is deleted (e.g., during flow cleanup), all associated sync firings should also be deleted. This maintains referential integrity without requiring manual cleanup.

**Why AUTO-INCREMENT id:**
The `provenance_edges` table needs to reference sync_firings. Using an auto-increment integer is simpler and more efficient than composite FKs on (completion_id, sync_id, binding_hash).

Note: This is an exception to CP-2 (content-addressed identity). Auto-increment IDs are acceptable for store-internal FK relationships, but NEVER for logical identity or replay.

### Store Interface

**WriteSyncFiring Function:**
```go
// internal/store/store.go

// WriteSyncFiring records a sync rule firing.
// Uses INSERT OR IGNORE for idempotent writes.
// Returns the firing ID (either newly inserted or existing).
func (s *Store) WriteSyncFiring(ctx context.Context, firing ir.SyncFiring) (int64, error) {
    result, err := s.db.ExecContext(ctx, `
        INSERT OR IGNORE INTO sync_firings
            (completion_id, sync_id, binding_hash, seq)
        VALUES (?, ?, ?, ?)
    `, firing.CompletionID, firing.SyncID, firing.BindingHash, firing.Seq)

    if err != nil {
        return 0, fmt.Errorf("write sync firing: %w", err)
    }

    // If INSERT OR IGNORE skipped (duplicate), query for existing ID
    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        var id int64
        err := s.db.QueryRowContext(ctx, `
            SELECT id FROM sync_firings
            WHERE completion_id = ? AND sync_id = ? AND binding_hash = ?
        `, firing.CompletionID, firing.SyncID, firing.BindingHash).Scan(&id)

        if err != nil {
            return 0, fmt.Errorf("query existing firing: %w", err)
        }
        return id, nil
    }

    // New insert succeeded
    id, err := result.LastInsertId()
    if err != nil {
        return 0, fmt.Errorf("get last insert id: %w", err)
    }
    return id, nil
}
```

**HasFiring Function:**
```go
// HasFiring checks if a sync firing already exists.
// Used by engine to implement idempotent firing.
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
```

**Why INSERT OR IGNORE:**
- Normal path: First time firing → INSERT succeeds
- Idempotent path: Duplicate firing (replay) → INSERT silently skips (no error)
- Result: Both paths succeed, no error handling needed for expected duplicates

**Alternative (less elegant):**
```go
// Check first, then insert - introduces race condition
exists, _ := s.HasFiring(ctx, ...)
if !exists {
    s.db.Exec("INSERT INTO ...") // ❌ Can still fail if concurrent insert
}
```

INSERT OR IGNORE is simpler and race-free.

### Testing Strategy

**Unit Tests (internal/store/store_test.go):**

```go
func TestWriteSyncFiring_FirstWrite(t *testing.T) {
    // GIVEN: Empty database
    // WHEN: WriteSyncFiring called
    // THEN: Firing inserted, ID returned
}

func TestWriteSyncFiring_Idempotent(t *testing.T) {
    // GIVEN: Existing firing
    // WHEN: WriteSyncFiring called with same (completion, sync, binding)
    // THEN: No error, existing ID returned, count unchanged
}

func TestWriteSyncFiring_DifferentBinding(t *testing.T) {
    // GIVEN: Existing firing with binding_hash_1
    // WHEN: WriteSyncFiring called with same (completion, sync) but binding_hash_2
    // THEN: Second firing inserted (different binding)
}

func TestHasFiring_Exists(t *testing.T) {
    // GIVEN: Existing firing
    // WHEN: HasFiring called
    // THEN: Returns true
}

func TestHasFiring_NotExists(t *testing.T) {
    // GIVEN: No matching firing
    // WHEN: HasFiring called
    // THEN: Returns false
}

func TestHasFiring_PartialMatch(t *testing.T) {
    // GIVEN: Firing with (comp1, sync1, hash1)
    // WHEN: HasFiring called with (comp1, sync1, hash2)
    // THEN: Returns false (binding_hash doesn't match)
}
```

**Integration Tests (internal/store/integration_test.go):**

```go
func TestMultiBindingSync_AllBindingsFire(t *testing.T) {
    // GIVEN: Completion that triggers sync with 3 bindings
    // WHEN: Sync evaluated
    // THEN: 3 separate firings inserted (different binding_hash values)
    // AND: 3 separate invocations generated
}

func TestSyncFiring_UniqueConstraint(t *testing.T) {
    // GIVEN: Existing firing
    // WHEN: Manual INSERT with duplicate (completion, sync, binding)
    // THEN: UNIQUE constraint violation (caught by INSERT OR IGNORE in real code)
}

func TestSyncFiring_ForeignKey(t *testing.T) {
    // GIVEN: Firing referencing completion
    // WHEN: Completion deleted
    // THEN: Firing also deleted (ON DELETE CASCADE)
}

func TestSyncFiring_IndexPerformance(t *testing.T) {
    // GIVEN: 10000 firings for 1000 completions
    // WHEN: Query firings by completion_id
    // THEN: Uses index, sub-millisecond response
}
```

**Determinism Tests (internal/store/determinism_test.go):**

```go
func TestBindingHash_Deterministic(t *testing.T) {
    bindings := ir.IRObject{
        "item_id": ir.IRString("item-123"),
        "quantity": ir.IRInt(5),
    }

    hash1 := ir.BindingHash(bindings)
    hash2 := ir.BindingHash(bindings)

    assert.Equal(t, hash1, hash2, "Same bindings must produce same hash")
}

func TestBindingHash_KeyOrderIndependent(t *testing.T) {
    // Bindings created in different key order
    bindings1 := ir.IRObject{"a": ir.IRInt(1), "b": ir.IRInt(2)}
    bindings2 := ir.IRObject{"b": ir.IRInt(2), "a": ir.IRInt(1)}

    hash1 := ir.BindingHash(bindings1)
    hash2 := ir.BindingHash(bindings2)

    assert.Equal(t, hash1, hash2, "Key order must not affect hash (canonical JSON)")
}

func TestBindingHash_ValueSensitive(t *testing.T) {
    bindings1 := ir.IRObject{"x": ir.IRInt(1)}
    bindings2 := ir.IRObject{"x": ir.IRInt(2)}

    hash1 := ir.BindingHash(bindings1)
    hash2 := ir.BindingHash(bindings2)

    assert.NotEqual(t, hash1, hash2, "Different values must produce different hashes")
}

func TestBindingHash_EmptyBindings(t *testing.T) {
    bindings := ir.IRObject{}
    hash := ir.BindingHash(bindings)

    assert.NotEmpty(t, hash, "Empty bindings must produce valid hash")
    assert.Len(t, hash, 64, "SHA-256 hash is 64 hex characters")
}
```

**Replay Tests (internal/engine/replay_test.go):**

```go
func TestReplay_IdempotentFiring(t *testing.T) {
    // GIVEN: Flow with sync that produced 3 bindings
    // AND: All 3 firings and invocations written
    // WHEN: Engine replays the flow (e.g., after crash)
    // THEN: Where-clause re-executes → same 3 bindings
    // AND: Binding hashes re-computed → same hash values
    // AND: HasFiring returns true for all 3
    // AND: No duplicate invocations generated
    // AND: Final state identical to initial complete run
}

func TestReplay_PartialCompletion(t *testing.T) {
    // GIVEN: Flow with sync that should produce 3 bindings
    // AND: Only 2 firings written (crash mid-sync)
    // WHEN: Engine replays the flow
    // THEN: Where-clause re-executes → 3 bindings
    // AND: First 2 bindings skipped (HasFiring = true)
    // AND: Third binding fires (HasFiring = false)
    // AND: Final state has all 3 firings
}
```

### File List

Files to create/modify:

1. `internal/ir/hash.go` - BindingHash function (new file)
2. `internal/ir/hash_test.go` - Binding hash tests (new file)
3. `internal/store/migrations/003_sync_firings.sql` - Schema migration (new file)
4. `internal/store/store.go` - WriteSyncFiring, HasFiring functions (modify)
5. `internal/store/store_test.go` - Unit tests (modify)
6. `internal/store/integration_test.go` - Integration tests (new/modify)
7. `internal/store/determinism_test.go` - Determinism tests (new file)

### Story Completion Checklist

- [ ] BindingHash function implemented and tested
- [ ] sync_firings table created in migration
- [ ] WriteSyncFiring function implemented with INSERT OR IGNORE
- [ ] HasFiring function implemented
- [ ] UNIQUE(completion_id, sync_id, binding_hash) enforced
- [ ] Foreign key to completions(id) with ON DELETE CASCADE
- [ ] Index on completion_id created
- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] Determinism tests verify hash stability
- [ ] Replay tests verify idempotency
- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` passes
- [ ] `go test ./internal/store/...` passes

### References

- [Source: docs/architecture.md#CRITICAL-1] - Binding-level idempotency requirement
- [Source: docs/architecture.md#CP-1] - Binding-Level Idempotency pattern
- [Source: docs/architecture.md#CP-2] - Logical clocks, not timestamps
- [Source: docs/architecture.md#CP-3] - RFC 8785 canonical JSON
- [Source: docs/architecture.md#CP-4] - Domain separation for hashes
- [Source: docs/architecture.md#SQLite Configuration] - Schema conventions
- [Source: docs/prd.md#FR-4.2] - Idempotency via sync edges
- [Source: docs/epics.md#Story 2.5] - Story definition

## Dev Agent Record

### Agent Model Used

_To be filled during implementation_

### Validation History

_To be filled during implementation_

### Completion Notes

_To be filled during implementation_
