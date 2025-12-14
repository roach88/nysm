package store

import (
	"testing"

	"github.com/roach88/nysm/internal/ir"
)

func TestMarshalArgs_EmptyObject(t *testing.T) {
	args := ir.IRObject{}
	json, err := marshalArgs(args)
	if err != nil {
		t.Fatalf("marshalArgs() failed: %v", err)
	}
	if json != "{}" {
		t.Errorf("marshalArgs() = %q, want %q", json, "{}")
	}
}

func TestMarshalArgs_WithValues(t *testing.T) {
	args := ir.IRObject{
		"name":     ir.IRString("widget"),
		"quantity": ir.IRInt(42),
		"active":   ir.IRBool(true),
	}
	json, err := marshalArgs(args)
	if err != nil {
		t.Fatalf("marshalArgs() failed: %v", err)
	}

	// Canonical JSON has deterministic key ordering (alphabetical)
	expected := `{"active":true,"name":"widget","quantity":42}`
	if json != expected {
		t.Errorf("marshalArgs() = %q, want %q", json, expected)
	}
}

func TestMarshalArgs_NestedObject(t *testing.T) {
	args := ir.IRObject{
		"item": ir.IRObject{
			"id":    ir.IRString("abc"),
			"count": ir.IRInt(5),
		},
	}
	json, err := marshalArgs(args)
	if err != nil {
		t.Fatalf("marshalArgs() failed: %v", err)
	}

	expected := `{"item":{"count":5,"id":"abc"}}`
	if json != expected {
		t.Errorf("marshalArgs() = %q, want %q", json, expected)
	}
}

func TestMarshalArgs_WithArray(t *testing.T) {
	args := ir.IRObject{
		"items": ir.IRArray{ir.IRString("a"), ir.IRString("b"), ir.IRString("c")},
	}
	json, err := marshalArgs(args)
	if err != nil {
		t.Fatalf("marshalArgs() failed: %v", err)
	}

	expected := `{"items":["a","b","c"]}`
	if json != expected {
		t.Errorf("marshalArgs() = %q, want %q", json, expected)
	}
}

func TestMarshalResult_EmptyObject(t *testing.T) {
	result := ir.IRObject{}
	json, err := marshalResult(result)
	if err != nil {
		t.Fatalf("marshalResult() failed: %v", err)
	}
	if json != "{}" {
		t.Errorf("marshalResult() = %q, want %q", json, "{}")
	}
}

func TestMarshalResult_WithValues(t *testing.T) {
	result := ir.IRObject{
		"status": ir.IRString("completed"),
		"count":  ir.IRInt(10),
	}
	json, err := marshalResult(result)
	if err != nil {
		t.Fatalf("marshalResult() failed: %v", err)
	}

	expected := `{"count":10,"status":"completed"}`
	if json != expected {
		t.Errorf("marshalResult() = %q, want %q", json, expected)
	}
}

func TestMarshalSecurityContext_Empty(t *testing.T) {
	ctx := ir.SecurityContext{}
	json, err := marshalSecurityContext(ctx)
	if err != nil {
		t.Fatalf("marshalSecurityContext() failed: %v", err)
	}

	// Empty security context still serializes with null permissions
	expected := `{"permissions":null,"tenant_id":"","user_id":""}`
	if json != expected {
		t.Errorf("marshalSecurityContext() = %q, want %q", json, expected)
	}
}

func TestMarshalSecurityContext_WithValues(t *testing.T) {
	ctx := ir.SecurityContext{
		TenantID:    "tenant-123",
		UserID:      "user-456",
		Permissions: []string{"read", "write"},
	}
	json, err := marshalSecurityContext(ctx)
	if err != nil {
		t.Fatalf("marshalSecurityContext() failed: %v", err)
	}

	expected := `{"permissions":["read","write"],"tenant_id":"tenant-123","user_id":"user-456"}`
	if json != expected {
		t.Errorf("marshalSecurityContext() = %q, want %q", json, expected)
	}
}

func TestUnmarshalArgs_EmptyObject(t *testing.T) {
	args, err := unmarshalArgs("{}")
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}
	if len(args) != 0 {
		t.Errorf("unmarshalArgs() returned %d fields, want 0", len(args))
	}
}

func TestUnmarshalArgs_EmptyString(t *testing.T) {
	args, err := unmarshalArgs("")
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}
	if len(args) != 0 {
		t.Errorf("unmarshalArgs() returned %d fields, want 0", len(args))
	}
}

func TestUnmarshalArgs_WithValues(t *testing.T) {
	json := `{"active":true,"name":"widget","quantity":42}`
	args, err := unmarshalArgs(json)
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}

	if len(args) != 3 {
		t.Fatalf("unmarshalArgs() returned %d fields, want 3", len(args))
	}

	name, ok := args["name"].(ir.IRString)
	if !ok || name != "widget" {
		t.Errorf("args[name] = %v, want IRString(widget)", args["name"])
	}

	quantity, ok := args["quantity"].(ir.IRInt)
	if !ok || quantity != 42 {
		t.Errorf("args[quantity] = %v, want IRInt(42)", args["quantity"])
	}

	active, ok := args["active"].(ir.IRBool)
	if !ok || active != true {
		t.Errorf("args[active] = %v, want IRBool(true)", args["active"])
	}
}

func TestUnmarshalArgs_NestedObject(t *testing.T) {
	json := `{"item":{"count":5,"id":"abc"}}`
	args, err := unmarshalArgs(json)
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}

	item, ok := args["item"].(ir.IRObject)
	if !ok {
		t.Fatalf("args[item] is not IRObject: %T", args["item"])
	}

	id, ok := item["id"].(ir.IRString)
	if !ok || id != "abc" {
		t.Errorf("item[id] = %v, want IRString(abc)", item["id"])
	}

	count, ok := item["count"].(ir.IRInt)
	if !ok || count != 5 {
		t.Errorf("item[count] = %v, want IRInt(5)", item["count"])
	}
}

func TestUnmarshalArgs_WithArray(t *testing.T) {
	json := `{"items":["a","b","c"]}`
	args, err := unmarshalArgs(json)
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}

	items, ok := args["items"].(ir.IRArray)
	if !ok {
		t.Fatalf("args[items] is not IRArray: %T", args["items"])
	}

	if len(items) != 3 {
		t.Fatalf("items has %d elements, want 3", len(items))
	}

	for i, expected := range []string{"a", "b", "c"} {
		s, ok := items[i].(ir.IRString)
		if !ok || string(s) != expected {
			t.Errorf("items[%d] = %v, want IRString(%s)", i, items[i], expected)
		}
	}
}

func TestUnmarshalArgs_InvalidJSON(t *testing.T) {
	_, err := unmarshalArgs("not valid json")
	if err == nil {
		t.Error("unmarshalArgs() should fail on invalid JSON")
	}
}

func TestUnmarshalResult_EmptyObject(t *testing.T) {
	result, err := unmarshalResult("{}")
	if err != nil {
		t.Fatalf("unmarshalResult() failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("unmarshalResult() returned %d fields, want 0", len(result))
	}
}

func TestUnmarshalResult_WithValues(t *testing.T) {
	json := `{"count":10,"status":"completed"}`
	result, err := unmarshalResult(json)
	if err != nil {
		t.Fatalf("unmarshalResult() failed: %v", err)
	}

	status, ok := result["status"].(ir.IRString)
	if !ok || status != "completed" {
		t.Errorf("result[status] = %v, want IRString(completed)", result["status"])
	}

	count, ok := result["count"].(ir.IRInt)
	if !ok || count != 10 {
		t.Errorf("result[count] = %v, want IRInt(10)", result["count"])
	}
}

func TestUnmarshalSecurityContext_Empty(t *testing.T) {
	ctx, err := unmarshalSecurityContext("{}")
	if err != nil {
		t.Fatalf("unmarshalSecurityContext() failed: %v", err)
	}
	if ctx.TenantID != "" || ctx.UserID != "" {
		t.Errorf("unmarshalSecurityContext() = %+v, want empty", ctx)
	}
}

func TestUnmarshalSecurityContext_EmptyString(t *testing.T) {
	ctx, err := unmarshalSecurityContext("")
	if err != nil {
		t.Fatalf("unmarshalSecurityContext() failed: %v", err)
	}
	if ctx.TenantID != "" || ctx.UserID != "" {
		t.Errorf("unmarshalSecurityContext() = %+v, want empty", ctx)
	}
}

func TestUnmarshalSecurityContext_WithValues(t *testing.T) {
	json := `{"permissions":["read","write"],"tenant_id":"tenant-123","user_id":"user-456"}`
	ctx, err := unmarshalSecurityContext(json)
	if err != nil {
		t.Fatalf("unmarshalSecurityContext() failed: %v", err)
	}

	if ctx.TenantID != "tenant-123" {
		t.Errorf("ctx.TenantID = %q, want %q", ctx.TenantID, "tenant-123")
	}
	if ctx.UserID != "user-456" {
		t.Errorf("ctx.UserID = %q, want %q", ctx.UserID, "user-456")
	}
	if len(ctx.Permissions) != 2 {
		t.Fatalf("ctx.Permissions has %d elements, want 2", len(ctx.Permissions))
	}
	if ctx.Permissions[0] != "read" || ctx.Permissions[1] != "write" {
		t.Errorf("ctx.Permissions = %v, want [read, write]", ctx.Permissions)
	}
}

func TestUnmarshalSecurityContext_InvalidJSON(t *testing.T) {
	_, err := unmarshalSecurityContext("not valid json")
	if err == nil {
		t.Error("unmarshalSecurityContext() should fail on invalid JSON")
	}
}

func TestUnmarshalArgs_LargeInteger(t *testing.T) {
	// Test that large integers (>2^53) are preserved without precision loss
	// 2^53 = 9007199254740992, we test with 2^53 + 1 = 9007199254740993
	largeInt := int64(9007199254740993)
	json := `{"large":9007199254740993}`

	args, err := unmarshalArgs(json)
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}

	val, ok := args["large"].(ir.IRInt)
	if !ok {
		t.Fatalf("args[large] is not IRInt: %T", args["large"])
	}

	if int64(val) != largeInt {
		t.Errorf("args[large] = %d, want %d (precision loss!)", val, largeInt)
	}
}

func TestUnmarshalArgs_RejectsFloat(t *testing.T) {
	// Test that float values are rejected per CP-5
	json := `{"pi":3.14159}`

	_, err := unmarshalArgs(json)
	if err == nil {
		t.Error("unmarshalArgs() should reject float values (CP-5)")
	}
}

func TestMarshalUnmarshalRoundtrip(t *testing.T) {
	// Test round-trip serialization preserves data
	original := ir.IRObject{
		"string":  ir.IRString("hello"),
		"int":     ir.IRInt(42),
		"bool":    ir.IRBool(true),
		"array":   ir.IRArray{ir.IRInt(1), ir.IRInt(2), ir.IRInt(3)},
		"nested":  ir.IRObject{"inner": ir.IRString("value")},
	}

	json, err := marshalArgs(original)
	if err != nil {
		t.Fatalf("marshalArgs() failed: %v", err)
	}

	restored, err := unmarshalArgs(json)
	if err != nil {
		t.Fatalf("unmarshalArgs() failed: %v", err)
	}

	// Verify each field
	s, ok := restored["string"].(ir.IRString)
	if !ok || s != "hello" {
		t.Errorf("restored[string] = %v, want hello", restored["string"])
	}

	i, ok := restored["int"].(ir.IRInt)
	if !ok || i != 42 {
		t.Errorf("restored[int] = %v, want 42", restored["int"])
	}

	b, ok := restored["bool"].(ir.IRBool)
	if !ok || b != true {
		t.Errorf("restored[bool] = %v, want true", restored["bool"])
	}

	arr, ok := restored["array"].(ir.IRArray)
	if !ok || len(arr) != 3 {
		t.Errorf("restored[array] = %v, want array of 3", restored["array"])
	}

	nested, ok := restored["nested"].(ir.IRObject)
	if !ok {
		t.Errorf("restored[nested] = %T, want IRObject", restored["nested"])
	} else {
		inner, ok := nested["inner"].(ir.IRString)
		if !ok || inner != "value" {
			t.Errorf("nested[inner] = %v, want value", nested["inner"])
		}
	}
}
