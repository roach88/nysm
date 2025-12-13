# Story 1.5: Content-Addressed Identity with Domain Separation

Status: ready-for-dev

## Story

As a **developer building NYSM**,
I want **content-addressed IDs using SHA-256 with domain separation**,
So that **records have stable identity for replay and provenance**.

## Acceptance Criteria

1. **InvocationID function computes content-addressed ID**
   ```go
   func InvocationID(flowToken, actionURI string, args IRObject, seq int64) string
   ```
   - Returns hex-encoded SHA-256 hash
   - Uses domain prefix `nysm/invocation/v1`

2. **CompletionID function computes content-addressed ID**
   ```go
   func CompletionID(invocationID, outputCase string, result IRObject, seq int64) string
   ```
   - Returns hex-encoded SHA-256 hash
   - Uses domain prefix `nysm/completion/v1`

3. **BindingHash function computes hash for idempotency**
   ```go
   func BindingHash(bindings IRObject) (string, error)
   ```
   - Returns hex-encoded SHA-256 hash and error if marshaling fails
   - Uses domain prefix `nysm/binding/v1`
   - Use `MustBindingHash` in tests when inputs are known to be valid

4. **Domain separation prevents cross-type collisions**
   - Format: `{domain}\x00{canonical_json_bytes}`
   - Null byte (0x00) separator between domain and data

5. **Version prefix allows future algorithm migration**
   - Current version: `v1`
   - Version in domain string enables graceful upgrades

6. **All hashing uses MarshalCanonical from Story 1-4**
   - Deterministic serialization before hashing
   - UTF-16 key ordering, no HTML escaping, NFC normalization

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CP-4** | Content-addressed IDs with domain separation |
| **CP-3** | RFC 8785 UTF-16 key ordering in canonical JSON |

## Tasks / Subtasks

- [ ] Task 1: Implement hashWithDomain helper (AC: #4, #5)
  - [ ] 1.1 Create `internal/ir/hash.go`
  - [ ] 1.2 Implement domain separation with null byte
  - [ ] 1.3 Return hex-encoded SHA-256

- [ ] Task 2: Implement InvocationID (AC: #1)
  - [ ] 2.1 Build canonical object from parameters
  - [ ] 2.2 Hash with `nysm/invocation/v1` domain

- [ ] Task 3: Implement CompletionID (AC: #2)
  - [ ] 3.1 Build canonical object from parameters
  - [ ] 3.2 Hash with `nysm/completion/v1` domain

- [ ] Task 4: Implement BindingHash (AC: #3)
  - [ ] 4.1 Hash bindings with `nysm/binding/v1` domain

- [ ] Task 5: Write comprehensive tests
  - [ ] 5.1 Test determinism (same input → same output)
  - [ ] 5.2 Test domain separation (different domains → different hashes)
  - [ ] 5.3 Test version prefix handling
  - [ ] 5.4 Test cross-invocation with same content

## Dev Notes

### Hash Implementation

**IMPORTANT:** Hash functions return `(string, error)` instead of panicking.
This allows proper error propagation and makes fuzz/property testing more useful.

```go
// internal/ir/hash.go
package ir

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// Domain prefixes for content-addressed identity
const (
    DomainInvocation = "nysm/invocation/v1"
    DomainCompletion = "nysm/completion/v1"
    DomainBinding    = "nysm/binding/v1"
)

// hashWithDomain computes SHA-256 hash with domain separation.
// Format: SHA256(domain + 0x00 + data)
// The null byte prevents domain/data boundary ambiguity.
func hashWithDomain(domain string, data []byte) string {
    h := sha256.New()
    h.Write([]byte(domain))
    h.Write([]byte{0x00}) // Null separator - CRITICAL for security
    h.Write(data)
    return hex.EncodeToString(h.Sum(nil))
}

// InvocationID computes content-addressed ID for an invocation.
// The ID is stable across restarts and replays given the same inputs.
// Returns error if args cannot be canonically marshaled.
//
// DESIGN DECISION: SecurityContext is intentionally EXCLUDED from InvocationID.
// InvocationID represents "what happened" (logical identity), not "who did it".
// This enables:
//   - Replay flexibility: same action can be replayed under different auth context
//   - Stable provenance: Completion.InvocationID links remain valid across replays
//   - Decoupling from mutable data: Roles/Permissions can change without invalidating IDs
//
// For cryptographic "who did it" binding, use a separate AttributionHash (future story).
// SecurityContext is still stored on the Invocation record for audit purposes.
func InvocationID(flowToken, actionURI string, args IRObject, seq int64) (string, error) {
    // Build object for hashing using IRObject for type safety (CP-5)
    // NOTE: SecurityContext excluded - see design decision above
    obj := IRObject{
        "flow_token": IRString(flowToken),
        "action_uri": IRString(actionURI),
        "args":       args, // Already IRObject
        "seq":        IRInt(seq),
    }

    canonical, err := MarshalCanonical(obj)
    if err != nil {
        return "", fmt.Errorf("InvocationID: failed to marshal: %w", err)
    }

    return hashWithDomain(DomainInvocation, canonical), nil
}

// CompletionID computes content-addressed ID for a completion.
// Links to the invocation it completes via invocationID.
// Returns error if result cannot be canonically marshaled.
func CompletionID(invocationID, outputCase string, result IRObject, seq int64) (string, error) {
    obj := IRObject{
        "invocation_id": IRString(invocationID),
        "output_case":   IRString(outputCase),
        "result":        result, // Already IRObject
        "seq":           IRInt(seq),
    }

    canonical, err := MarshalCanonical(obj)
    if err != nil {
        return "", fmt.Errorf("CompletionID: failed to marshal: %w", err)
    }

    return hashWithDomain(DomainCompletion, canonical), nil
}

// BindingHash computes hash for idempotency checking.
// Used in sync_firings table: UNIQUE(completion_id, sync_id, binding_hash)
// Returns error if bindings cannot be canonically marshaled.
func BindingHash(bindings IRObject) (string, error) {
    canonical, err := MarshalCanonical(bindings)
    if err != nil {
        return "", fmt.Errorf("BindingHash: failed to marshal: %w", err)
    }

    return hashWithDomain(DomainBinding, canonical), nil
}

// MustInvocationID is like InvocationID but panics on error.
// Use only in tests or when inputs are known to be valid.
func MustInvocationID(flowToken, actionURI string, args IRObject, seq int64) string {
    id, err := InvocationID(flowToken, actionURI, args, seq)
    if err != nil {
        panic(err)
    }
    return id
}

// MustCompletionID is like CompletionID but panics on error.
// Use only in tests or when inputs are known to be valid.
func MustCompletionID(invocationID, outputCase string, result IRObject, seq int64) string {
    id, err := CompletionID(invocationID, outputCase, result, seq)
    if err != nil {
        panic(err)
    }
    return id
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
```

### Why SecurityContext is Excluded from InvocationID

**DESIGN DECISION (validated by Codex + Gemini 3 Pro consensus):**

`InvocationID` intentionally excludes `SecurityContext` because it represents **logical identity**
("what happened at position `seq` in flow"), not **attribution** ("who did it").

**Reasons for exclusion:**

1. **Replay Flexibility**: Replaying the same invocation under a different auth context
   (simulation, migration, debugging with service account) would change the ID and break
   referential integrity (`Completion.InvocationID`, provenance edges, idempotent inserts).

2. **Decoupling from Mutable Data**: `Roles`/`Permissions` are derived from policy evaluation
   and can change without changing "the invocation". Including them couples identity to
   policy evolution.

3. **Array Ordering Hazard**: `Roles []string` would require deterministic sorting before
   hashing (RFC 8785 doesn't reorder arrays), adding fragility to the primary identifier.

4. **Privacy/Correlation Risk**: Low-entropy `Principal`/`UserID` in hash inputs could enable
   offline guessing/correlation when other fields are known.

**How to get cryptographic "who did it" binding:**

If you need to cryptographically bind the actor to the invocation, add a **separate** hash:

```go
// Option: AttributionHash (future story)
// Binds WHO performed THIS invocation without changing InvocationID
const DomainAttribution = "nysm/invocation_attribution/v1"

func AttributionHash(invocationID string, secCtx SecurityContext) (string, error) {
    obj := IRObject{
        "invocation_id": IRString(invocationID),
        "principal":     IRString(secCtx.Principal),
        "roles":         sortedRolesArray(secCtx.Roles), // Must sort for determinism
        "session_id":    IRString(secCtx.SessionID),
    }
    canonical, err := MarshalCanonical(obj)
    if err != nil {
        return "", err
    }
    return hashWithDomain(DomainAttribution, canonical), nil
}
```

This preserves `InvocationID` stability while enabling cryptographic audit trails.

### Why Domain Separation?

Without domain separation, different record types could collide:

```go
// WITHOUT domain separation (BAD):
// Invocation with args {"id": "abc"} could hash to same value as
// Completion with result {"id": "abc"} if their other fields happen to align

// WITH domain separation (GOOD):
// "nysm/invocation/v1\x00{...}" will NEVER equal
// "nysm/completion/v1\x00{...}" even with identical JSON bodies
```

The null byte (0x00) separator prevents domain/data boundary confusion:
- Without it: domain "foo" + data "bar" = domain "foob" + data "ar"
- With it: "foo\x00bar" ≠ "foob\x00ar"

### Why SHA-256?

From Architecture: "SHA-256 is 'overkill' but eliminates collision as correctness concern."

- 256-bit output → 2^128 collision resistance
- Well-supported in Go stdlib
- Fast enough for event processing
- No known practical attacks

### Version Prefix

The `v1` in domain strings allows future algorithm migration:

```go
// Current
hashWithDomain("nysm/invocation/v1", data)

// Future (hypothetical)
hashWithDomain("nysm/invocation/v2", data) // Different algorithm or structure
```

This enables:
- Gradual migration to new hash algorithms
- Schema evolution without breaking existing IDs
- Mixed-version data stores during transition

### Test Examples

```go
func TestInvocationIDDeterminism(t *testing.T) {
    flowToken := "flow-123"
    actionURI := "Cart.addItem"
    args := IRObject{
        "item_id":  IRString("SKU-001"),
        "quantity": IRInt(2),
    }
    seq := int64(1)

    // Same inputs must produce same ID
    id1 := InvocationID(flowToken, actionURI, args, seq)
    id2 := InvocationID(flowToken, actionURI, args, seq)

    assert.Equal(t, id1, id2, "InvocationID must be deterministic")
    assert.Len(t, id1, 64, "SHA-256 hex is 64 characters")
}

func TestInvocationIDChangesWithInput(t *testing.T) {
    args := IRObject{"item_id": IRString("SKU-001")}

    id1 := InvocationID("flow-1", "Cart.addItem", args, 1)
    id2 := InvocationID("flow-2", "Cart.addItem", args, 1) // Different flow
    id3 := InvocationID("flow-1", "Cart.addItem", args, 2) // Different seq

    assert.NotEqual(t, id1, id2, "Different flow tokens should produce different IDs")
    assert.NotEqual(t, id1, id3, "Different seq should produce different IDs")
}

func TestCompletionIDLinksToInvocation(t *testing.T) {
    invID := InvocationID("flow-1", "Cart.addItem", IRObject{}, 1)

    result := IRObject{
        "new_quantity": IRInt(5),
    }

    compID := CompletionID(invID, "Success", result, 2)

    assert.Len(t, compID, 64, "CompletionID is SHA-256 hex")
    assert.NotEqual(t, invID, compID, "Completion ID differs from Invocation ID")
}

func TestDomainSeparationPreventsCrossTypeCollision(t *testing.T) {
    // Create identical JSON content for different record types
    sameContent := IRObject{
        "id":   IRString("test"),
        "data": IRInt(42),
    }

    // Hash as "invocation"
    inv, _ := MarshalCanonical(map[string]any{
        "flow_token": "same",
        "action_uri": "same",
        "args":       sameContent,
        "seq":        int64(1),
    })
    invHash := hashWithDomain(DomainInvocation, inv)

    // Hash as "binding"
    bindingHash := hashWithDomain(DomainBinding, inv) // Same bytes, different domain

    assert.NotEqual(t, invHash, bindingHash,
        "Domain separation must prevent cross-type collisions")
}

func TestBindingHashForIdempotency(t *testing.T) {
    bindings := IRObject{
        "cart_id": IRString("cart-123"),
        "item_id": IRString("SKU-001"),
    }

    hash1 := BindingHash(bindings)
    hash2 := BindingHash(bindings)

    assert.Equal(t, hash1, hash2, "Same bindings must produce same hash")

    // Different bindings produce different hash
    differentBindings := IRObject{
        "cart_id": IRString("cart-456"), // Different cart
        "item_id": IRString("SKU-001"),
    }
    hash3 := BindingHash(differentBindings)

    assert.NotEqual(t, hash1, hash3, "Different bindings must produce different hash")
}

func TestHashWithDomainNullSeparator(t *testing.T) {
    // Verify null separator prevents boundary confusion
    // "foo" + 0x00 + "bar" ≠ "foob" + 0x00 + "ar"

    hash1 := hashWithDomain("foo", []byte("bar"))
    hash2 := hashWithDomain("foob", []byte("ar"))

    assert.NotEqual(t, hash1, hash2, "Null separator must prevent boundary confusion")
}

func TestInvocationIDKeyOrdering(t *testing.T) {
    // Verify that key ordering is deterministic (UTF-16)
    args := IRObject{
        "zebra": IRInt(1),
        "alpha": IRInt(2),
    }

    id1 := InvocationID("flow", "action", args, 1)

    // Create args in different insertion order (Go maps don't guarantee order)
    args2 := IRObject{
        "alpha": IRInt(2),
        "zebra": IRInt(1),
    }

    id2 := InvocationID("flow", "action", args2, 1)

    assert.Equal(t, id1, id2, "Key ordering must be deterministic regardless of insertion order")
}
```

### File List

Files to create/modify:

1. `internal/ir/hash.go` - Hash functions with domain separation
2. `internal/ir/hash_test.go` - Comprehensive tests

### Relationship to Other Stories

- **Story 1-4:** Uses MarshalCanonical for deterministic serialization
- **Story 2-3:** Event store uses InvocationID/CompletionID for records
- **Story 5-1:** Binding hash used for idempotency enforcement

### Story Completion Checklist

- [ ] hashWithDomain implemented with null separator
- [ ] InvocationID implemented with `nysm/invocation/v1` domain
- [ ] CompletionID implemented with `nysm/completion/v1` domain
- [ ] BindingHash implemented with `nysm/binding/v1` domain
- [ ] All functions use MarshalCanonical
- [ ] SHA-256 with hex encoding
- [ ] Determinism tests pass
- [ ] Domain separation tests pass
- [ ] All tests pass
- [ ] `go vet ./internal/ir/...` passes

### References

- [Source: docs/architecture.md#CP-2] - Logical Identity pattern
- [Source: docs/architecture.md#Content-Addressed] - SHA-256 with domain separation
- [Source: docs/epics.md#Story 1.5] - Story definition

## Dev Agent Record

### Agent Model Used

Claude Opus 4.5 (claude-opus-4-5-20251101)

### Validation History

- Initial creation: create-story workflow
- 2025-12-12: codex review - changed hash functions to return (string, error) instead of panicking, added Must* variants for test convenience, fixed hash payload to use typed IRObject instead of map[string]any
- 2025-12-12: consensus review (Codex + Gemini 3 Pro) - confirmed SecurityContext should be EXCLUDED from InvocationID; documented rationale and added AttributionHash pattern for future "who did it" binding

### Completion Notes

- Content-addressed IDs enable replay and provenance tracking
- Domain separation is critical for security (prevents type confusion attacks)
- Version prefix enables graceful hash algorithm migration
- Depends on MarshalCanonical for deterministic serialization
- **SecurityContext intentionally excluded** from InvocationID (logical identity vs attribution)
