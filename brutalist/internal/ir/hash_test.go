package ir

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationIDDeterminism(t *testing.T) {
	flowToken := "flow-123"
	actionURI := "Cart.addItem"
	args := IRObject{
		"item_id":  IRString("SKU-001"),
		"quantity": IRInt(2),
	}
	seq := int64(1)

	// Same inputs must produce same ID
	id1, err := InvocationID(flowToken, actionURI, args, seq)
	require.NoError(t, err)

	id2, err := InvocationID(flowToken, actionURI, args, seq)
	require.NoError(t, err)

	assert.Equal(t, id1, id2, "InvocationID must be deterministic")
	assert.Len(t, id1, 64, "SHA-256 hex is 64 characters")
}

func TestInvocationIDChangesWithInput(t *testing.T) {
	args := IRObject{"item_id": IRString("SKU-001")}

	id1 := MustInvocationID("flow-1", "Cart.addItem", args, 1)
	id2 := MustInvocationID("flow-2", "Cart.addItem", args, 1) // Different flow
	id3 := MustInvocationID("flow-1", "Cart.addItem", args, 2) // Different seq
	id4 := MustInvocationID("flow-1", "Cart.removeItem", args, 1) // Different action

	assert.NotEqual(t, id1, id2, "Different flow tokens should produce different IDs")
	assert.NotEqual(t, id1, id3, "Different seq should produce different IDs")
	assert.NotEqual(t, id1, id4, "Different action should produce different IDs")
}

func TestInvocationIDChangesWithArgs(t *testing.T) {
	args1 := IRObject{"item_id": IRString("SKU-001")}
	args2 := IRObject{"item_id": IRString("SKU-002")}

	id1 := MustInvocationID("flow-1", "Cart.addItem", args1, 1)
	id2 := MustInvocationID("flow-1", "Cart.addItem", args2, 1)

	assert.NotEqual(t, id1, id2, "Different args should produce different IDs")
}

func TestCompletionIDDeterminism(t *testing.T) {
	invID := MustInvocationID("flow-1", "Cart.addItem", IRObject{}, 1)
	result := IRObject{"new_quantity": IRInt(5)}

	id1, err := CompletionID(invID, "Success", result, 2)
	require.NoError(t, err)

	id2, err := CompletionID(invID, "Success", result, 2)
	require.NoError(t, err)

	assert.Equal(t, id1, id2, "CompletionID must be deterministic")
	assert.Len(t, id1, 64, "SHA-256 hex is 64 characters")
}

func TestCompletionIDLinksToInvocation(t *testing.T) {
	invID := MustInvocationID("flow-1", "Cart.addItem", IRObject{}, 1)
	result := IRObject{"new_quantity": IRInt(5)}

	compID := MustCompletionID(invID, "Success", result, 2)

	assert.Len(t, compID, 64, "CompletionID is SHA-256 hex")
	assert.NotEqual(t, invID, compID, "Completion ID differs from Invocation ID")
}

func TestCompletionIDChangesWithOutput(t *testing.T) {
	invID := "inv-123"
	result := IRObject{"value": IRInt(1)}

	id1 := MustCompletionID(invID, "Success", result, 1)
	id2 := MustCompletionID(invID, "Error", result, 1)   // Different output case
	id3 := MustCompletionID(invID, "Success", result, 2) // Different seq

	assert.NotEqual(t, id1, id2, "Different output case should produce different IDs")
	assert.NotEqual(t, id1, id3, "Different seq should produce different IDs")
}

func TestBindingHashDeterminism(t *testing.T) {
	bindings := IRObject{
		"cart_id": IRString("cart-123"),
		"item_id": IRString("SKU-001"),
	}

	hash1 := MustBindingHash(bindings)
	hash2 := MustBindingHash(bindings)

	assert.Equal(t, hash1, hash2, "Same bindings must produce same hash")
	assert.Len(t, hash1, 64, "SHA-256 hex is 64 characters")
}

func TestBindingHashChangesWithContent(t *testing.T) {
	bindings1 := IRObject{
		"cart_id": IRString("cart-123"),
		"item_id": IRString("SKU-001"),
	}
	bindings2 := IRObject{
		"cart_id": IRString("cart-456"), // Different cart
		"item_id": IRString("SKU-001"),
	}

	hash1 := MustBindingHash(bindings1)
	hash2 := MustBindingHash(bindings2)

	assert.NotEqual(t, hash1, hash2, "Different bindings must produce different hash")
}

func TestDomainSeparationPreventsCrossTypeCollision(t *testing.T) {
	// Same data hashed with different domains must produce different hashes
	data := []byte(`{"id":"test","data":42}`)

	invHash := hashWithDomain(DomainInvocation, data)
	compHash := hashWithDomain(DomainCompletion, data)
	bindHash := hashWithDomain(DomainBinding, data)

	assert.NotEqual(t, invHash, compHash, "Different domains must produce different hashes")
	assert.NotEqual(t, invHash, bindHash, "Different domains must produce different hashes")
	assert.NotEqual(t, compHash, bindHash, "Different domains must produce different hashes")
}

func TestHashWithDomainNullSeparator(t *testing.T) {
	// Verify null separator prevents boundary confusion
	// "foo" + 0x00 + "bar" ≠ "foob" + 0x00 + "ar"

	hash1 := hashWithDomain("foo", []byte("bar"))
	hash2 := hashWithDomain("foob", []byte("ar"))

	assert.NotEqual(t, hash1, hash2, "Null separator must prevent boundary confusion")
}

func TestInvocationIDKeyOrdering(t *testing.T) {
	// Verify that key ordering is deterministic (UTF-16 via canonical marshaling)
	args := IRObject{
		"zebra": IRInt(1),
		"alpha": IRInt(2),
	}

	id1 := MustInvocationID("flow", "action", args, 1)

	// Create args in different insertion order (Go maps don't guarantee order)
	args2 := IRObject{
		"alpha": IRInt(2),
		"zebra": IRInt(1),
	}

	id2 := MustInvocationID("flow", "action", args2, 1)

	assert.Equal(t, id1, id2, "Key ordering must be deterministic regardless of insertion order")
}

func TestEmptyArgsAndResult(t *testing.T) {
	// Empty objects should still produce valid hashes
	invID := MustInvocationID("flow", "action", IRObject{}, 1)
	compID := MustCompletionID(invID, "Success", IRObject{}, 2)
	bindHash := MustBindingHash(IRObject{})

	assert.Len(t, invID, 64)
	assert.Len(t, compID, 64)
	assert.Len(t, bindHash, 64)
}

func TestDomainConstants(t *testing.T) {
	// Verify domain constants are what we expect
	assert.Equal(t, "nysm/invocation/v1", DomainInvocation)
	assert.Equal(t, "nysm/completion/v1", DomainCompletion)
	assert.Equal(t, "nysm/binding/v1", DomainBinding)
}

func TestNestedArgsHash(t *testing.T) {
	// Complex nested args should hash correctly
	args := IRObject{
		"nested": IRObject{
			"deep": IRArray{
				IRInt(1),
				IRString("two"),
				IRObject{"value": IRBool(true)},
			},
		},
		"simple": IRString("test"),
	}

	id1 := MustInvocationID("flow", "action", args, 1)
	id2 := MustInvocationID("flow", "action", args, 1)

	assert.Equal(t, id1, id2, "Nested args must hash deterministically")
}

func TestInvocationIDErrorHandling(t *testing.T) {
	// The function should return error if marshaling fails
	// Since all our IRValue types are valid, we just verify the error path exists
	// by checking the function signature allows error returns

	id, err := InvocationID("flow", "action", IRObject{}, 1)
	require.NoError(t, err)
	assert.Len(t, id, 64)
}

func TestCompletionIDErrorHandling(t *testing.T) {
	id, err := CompletionID("inv-123", "Success", IRObject{}, 1)
	require.NoError(t, err)
	assert.Len(t, id, 64)
}

func TestBindingHashErrorHandling(t *testing.T) {
	hash, err := BindingHash(IRObject{})
	require.NoError(t, err)
	assert.Len(t, hash, 64)
}

func TestMustFunctionsPanic(t *testing.T) {
	// The Must* functions should not panic with valid input
	assert.NotPanics(t, func() {
		MustInvocationID("flow", "action", IRObject{}, 1)
	})
	assert.NotPanics(t, func() {
		MustCompletionID("inv", "Success", IRObject{}, 1)
	})
	assert.NotPanics(t, func() {
		MustBindingHash(IRObject{})
	})
}

func TestHashHexEncoding(t *testing.T) {
	// Verify output is valid hex (only 0-9a-f characters)
	id := MustInvocationID("flow", "action", IRObject{}, 1)

	for _, c := range id {
		valid := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
		assert.True(t, valid, "Hash should only contain hex characters, got: %c", c)
	}
}
