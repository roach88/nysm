package ir

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Domain prefixes for content-addressed identity (CP-4).
// Version suffix enables future algorithm migration.
const (
	DomainInvocation = "nysm/invocation/v1"
	DomainCompletion = "nysm/completion/v1"
	DomainBinding    = "nysm/binding/v1"
)

// hashWithDomain computes SHA-256 hash with domain separation.
// Format: SHA256(domain + 0x00 + data)
// The null byte (0x00) separator prevents domain/data boundary ambiguity.
//
// Example: hashWithDomain("nysm/invocation/v1", jsonBytes)
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
		"args":       args,
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
		"result":        result,
		"seq":           IRInt(seq),
	}

	canonical, err := MarshalCanonical(obj)
	if err != nil {
		return "", fmt.Errorf("CompletionID: failed to marshal: %w", err)
	}

	return hashWithDomain(DomainCompletion, canonical), nil
}

// BindingHash computes hash for idempotency checking (CP-1).
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
