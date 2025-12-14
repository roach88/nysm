package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/roach88/nysm/internal/ir"
)

// marshalArgs converts IRObject to canonical JSON TEXT for storage.
// Uses RFC 8785 canonical JSON for deterministic serialization.
func marshalArgs(args ir.IRObject) (string, error) {
	data, err := ir.MarshalCanonical(args)
	if err != nil {
		return "", fmt.Errorf("marshal args: %w", err)
	}
	return string(data), nil
}

// marshalResult converts IRObject to canonical JSON TEXT for storage.
// Uses RFC 8785 canonical JSON for deterministic serialization.
func marshalResult(result ir.IRObject) (string, error) {
	data, err := ir.MarshalCanonical(result)
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(data), nil
}

// marshalSecurityContext converts SecurityContext to JSON TEXT.
// Uses json.Encoder with HTML escaping disabled for RFC 8785 compliance.
// Note: SecurityContext is a struct (not IRValue), so we use json.Encoder
// with sorted keys to ensure consistent output for golden traces.
func marshalSecurityContext(ctx ir.SecurityContext) (string, error) {
	// Build a map with sorted keys for deterministic output
	// Go's json.Marshal sorts map keys alphabetically since Go 1.12
	m := map[string]any{
		"permissions": ctx.Permissions,
		"tenant_id":   ctx.TenantID,
		"user_id":     ctx.UserID,
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // CRITICAL: Disable HTML escaping for RFC 8785 compliance
	if err := enc.Encode(m); err != nil {
		return "", fmt.Errorf("marshal security context: %w", err)
	}
	// Encoder adds a trailing newline, remove it
	return strings.TrimSpace(buf.String()), nil
}

// unmarshalArgs parses canonical JSON TEXT to IRObject.
// Uses ir.IRObject.UnmarshalJSON which properly handles large integers via json.Number
// to avoid float64 precision loss for values > 2^53.
func unmarshalArgs(data string) (ir.IRObject, error) {
	if data == "" || data == "{}" {
		return ir.IRObject{}, nil
	}
	var obj ir.IRObject
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, fmt.Errorf("unmarshal args: %w", err)
	}
	return obj, nil
}

// unmarshalResult parses canonical JSON TEXT to IRObject.
// Uses ir.IRObject.UnmarshalJSON which properly handles large integers via json.Number
// to avoid float64 precision loss for values > 2^53.
func unmarshalResult(data string) (ir.IRObject, error) {
	if data == "" || data == "{}" {
		return ir.IRObject{}, nil
	}
	var obj ir.IRObject
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return obj, nil
}

// unmarshalSecurityContext parses JSON TEXT to SecurityContext.
func unmarshalSecurityContext(data string) (ir.SecurityContext, error) {
	var ctx ir.SecurityContext
	if data == "" || data == "{}" {
		return ctx, nil
	}
	if err := json.Unmarshal([]byte(data), &ctx); err != nil {
		return ir.SecurityContext{}, fmt.Errorf("unmarshal security context: %w", err)
	}
	return ctx, nil
}

