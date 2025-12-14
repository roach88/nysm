package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

func TestTraceMissingDatabaseFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--flow", "test-flow"}) // Missing --db flag

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestTraceMissingFlowFlag(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create empty database
	st, err := store.Open(dbPath)
	require.NoError(t, err)
	st.Close()

	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--db", dbPath}) // Missing --flow flag

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestTraceNonExistentDatabase(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "/nonexistent/path/test.db", "--flow", "test-flow"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open database")
}

func TestTraceEmptyFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create empty database
	st, err := store.Open(dbPath)
	require.NoError(t, err)
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--flow", "nonexistent-flow"})

	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No events found")
}

func TestTraceWithFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Create database with some data
	st, err := store.Open(dbPath)
	require.NoError(t, err)

	// Write an invocation and completion
	inv := ir.Invocation{
		ID:            "inv-1",
		FlowToken:     "test-flow-1",
		ActionURI:     "Cart.addItem",
		Args:          ir.IRObject{"item": ir.IRString("widget")},
		Seq:           1,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{"count": ir.IRInt(1)},
		Seq:          2,
	}
	require.NoError(t, st.WriteCompletion(ctx, comp))
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--flow", "test-flow-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-flow-1")
	assert.Contains(t, output, "Timeline")
	assert.Contains(t, output, "Cart.addItem")
	assert.Contains(t, output, "Success")
	assert.Contains(t, output, "Stats")
}

func TestTraceWithFlowJSON(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Create database with some data
	st, err := store.Open(dbPath)
	require.NoError(t, err)

	// Write an invocation and completion
	inv := ir.Invocation{
		ID:            "inv-1",
		FlowToken:     "test-flow-1",
		ActionURI:     "Cart.addItem",
		Args:          ir.IRObject{},
		Seq:           1,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv))

	comp := ir.Completion{
		ID:           "comp-1",
		InvocationID: "inv-1",
		OutputCase:   "Success",
		Result:       ir.IRObject{},
		Seq:          2,
	}
	require.NoError(t, st.WriteCompletion(ctx, comp))
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--flow", "test-flow-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	var response CLIResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &response))
	assert.Equal(t, "ok", response.Status)
}

func TestTraceWithActionFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Create database with multiple actions
	st, err := store.Open(dbPath)
	require.NoError(t, err)

	// Write two invocations with different actions
	inv1 := ir.Invocation{
		ID:            "inv-1",
		FlowToken:     "test-flow-1",
		ActionURI:     "Cart.addItem",
		Args:          ir.IRObject{},
		Seq:           1,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv1))

	inv2 := ir.Invocation{
		ID:            "inv-2",
		FlowToken:     "test-flow-1",
		ActionURI:     "Inventory.check",
		Args:          ir.IRObject{},
		Seq:           2,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv2))
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text", Verbose: true}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--flow", "test-flow-1", "--action", "Cart.addItem"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Cart.addItem")
	assert.NotContains(t, output, "Inventory.check")
}

func TestTraceHelpText(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTraceCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "provenance")
	assert.Contains(t, output, "--db")
	assert.Contains(t, output, "--flow")
	assert.Contains(t, output, "--action")
}

func TestIRObjectToMap(t *testing.T) {
	obj := ir.IRObject{
		"string": ir.IRString("hello"),
		"int":    ir.IRInt(42),
		"bool":   ir.IRBool(true),
		"array":  ir.IRArray{ir.IRInt(1), ir.IRInt(2)},
		"nested": ir.IRObject{"inner": ir.IRString("value")},
	}

	result := irObjectToMap(obj)

	assert.Equal(t, "hello", result["string"])
	assert.Equal(t, int64(42), result["int"])
	assert.Equal(t, true, result["bool"])
	assert.Equal(t, []interface{}{int64(1), int64(2)}, result["array"])
	assert.Equal(t, map[string]interface{}{"inner": "value"}, result["nested"])
}

func TestIRObjectToMapNil(t *testing.T) {
	result := irObjectToMap(nil)
	assert.Nil(t, result)
}

func TestTruncateID(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"short", "short"},
		{"exactly16chars!!", "exactly16chars!!"},
		{"this-is-a-very-long-invocation-id-that-should-be-truncated", "this-is-...runcated"},
	}

	for _, tc := range testCases {
		result := truncateID(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}

func TestCompleteStatus(t *testing.T) {
	assert.Equal(t, "Complete", completeStatus(true))
	assert.Contains(t, completeStatus(false), "Incomplete")
}

func TestFormatArgs(t *testing.T) {
	// Empty args
	assert.Equal(t, "{}", formatArgs(nil))
	assert.Equal(t, "{}", formatArgs(map[string]interface{}{}))

	// Single arg
	result := formatArgs(map[string]interface{}{"key": "value"})
	assert.Contains(t, result, "key=value")

	// Multiple args - should be in sorted order (deterministic)
	result = formatArgs(map[string]interface{}{"b": 2, "a": 1, "c": 3})
	assert.Equal(t, "{a=1, b=2, c=3}", result)
}

func TestFormatArgsNested(t *testing.T) {
	// Nested map - should be formatted deterministically
	nested := map[string]interface{}{
		"outer": map[string]interface{}{
			"z": 3,
			"a": 1,
		},
		"simple": "value",
	}
	result := formatArgs(nested)
	// Keys should be sorted at each level
	assert.Equal(t, "{outer={a=1, z=3}, simple=value}", result)
}

func TestFormatArgsArray(t *testing.T) {
	// Array values
	args := map[string]interface{}{
		"items": []interface{}{1, 2, 3},
	}
	result := formatArgs(args)
	assert.Equal(t, "{items=[1, 2, 3]}", result)
}

func TestFormatValue(t *testing.T) {
	// Test various value types
	assert.Equal(t, "hello", formatValue("hello"))
	assert.Equal(t, "42", formatValue(42))
	assert.Equal(t, "true", formatValue(true))
	assert.Equal(t, "{a=1}", formatValue(map[string]interface{}{"a": 1}))
	assert.Equal(t, "[1, 2]", formatValue([]interface{}{1, 2}))
}
