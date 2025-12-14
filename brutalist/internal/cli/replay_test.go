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

func TestReplayMissingDatabaseFlag(t *testing.T) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{}) // Missing --db flag

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestReplayEmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create empty database
	st, err := store.Open(dbPath)
	require.NoError(t, err)
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath})

	err = cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No flows found")
}

func TestReplayWithFlow(t *testing.T) {
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
		ActionURI:     "Test.action",
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
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "test-flow-1")
	assert.Contains(t, output, "1 flow(s)")
	assert.Contains(t, output, "✓")
	assert.Contains(t, output, "All flows verified deterministic")
}

func TestReplayWithFlowJSON(t *testing.T) {
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
		ActionURI:     "Test.action",
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
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath})

	err = cmd.Execute()
	require.NoError(t, err)

	// Parse JSON response
	var response CLIResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &response))
	assert.Equal(t, "ok", response.Status)
}

func TestReplaySpecificFlow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	ctx := context.Background()

	// Create database with two flows
	st, err := store.Open(dbPath)
	require.NoError(t, err)

	// Flow 1
	inv1 := ir.Invocation{
		ID:            "inv-1",
		FlowToken:     "flow-1",
		ActionURI:     "Test.action",
		Args:          ir.IRObject{},
		Seq:           1,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv1))

	// Flow 2
	inv2 := ir.Invocation{
		ID:            "inv-2",
		FlowToken:     "flow-2",
		ActionURI:     "Test.action",
		Args:          ir.IRObject{},
		Seq:           2,
		SpecHash:      "test-hash",
		EngineVersion: "test",
		IRVersion:     ir.IRVersion,
	}
	require.NoError(t, st.WriteInvocation(ctx, inv2))
	st.Close()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", dbPath, "--flow", "flow-1"})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "flow-1")
	assert.NotContains(t, output, "flow-2")
}

func TestReplayNonExistentDatabase(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--db", "/nonexistent/path/test.db"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open database")
}

func TestReplayHelpText(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewReplayCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Replay")
	assert.Contains(t, output, "--db")
	assert.Contains(t, output, "--flow")
	assert.Contains(t, output, "determinism")
}

func TestCompareEventSequences(t *testing.T) {
	// Test equal sequences
	events1 := []store.FlowEvent{
		{Type: store.EventInvocation, Seq: 1, ID: "inv-1"},
		{Type: store.EventCompletion, Seq: 2, ID: "comp-1"},
	}
	events2 := []store.FlowEvent{
		{Type: store.EventInvocation, Seq: 1, ID: "inv-1"},
		{Type: store.EventCompletion, Seq: 2, ID: "comp-1"},
	}
	assert.True(t, compareEventSequences(events1, events2))

	// Test different lengths
	events3 := []store.FlowEvent{
		{Type: store.EventInvocation, Seq: 1, ID: "inv-1"},
	}
	assert.False(t, compareEventSequences(events1, events3))

	// Test different content
	events4 := []store.FlowEvent{
		{Type: store.EventInvocation, Seq: 1, ID: "inv-1"},
		{Type: store.EventCompletion, Seq: 2, ID: "comp-2"}, // Different ID
	}
	assert.False(t, compareEventSequences(events1, events4))
}

func TestEventsEqual(t *testing.T) {
	// Test basic equality
	a := store.FlowEvent{Type: store.EventInvocation, Seq: 1, ID: "inv-1"}
	b := store.FlowEvent{Type: store.EventInvocation, Seq: 1, ID: "inv-1"}
	assert.True(t, eventsEqual(a, b))

	// Test different types
	c := store.FlowEvent{Type: store.EventCompletion, Seq: 1, ID: "inv-1"}
	assert.False(t, eventsEqual(a, c))

	// Test different seq
	d := store.FlowEvent{Type: store.EventInvocation, Seq: 2, ID: "inv-1"}
	assert.False(t, eventsEqual(a, d))

	// Test different ID
	e := store.FlowEvent{Type: store.EventInvocation, Seq: 1, ID: "inv-2"}
	assert.False(t, eventsEqual(a, e))
}
