package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvokeCommandStub(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewInvokeCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"Cart.addItem", "--args", `{"item_id":"widget","quantity":3}`})

	err := cmd.Execute()

	// Should error (not implemented)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")

	// Should print helpful message
	output := buf.String()
	assert.Contains(t, output, "Invocation request")
	assert.Contains(t, output, "Cart.addItem")
	assert.Contains(t, output, `{"item_id":"widget","quantity":3}`)
	assert.Contains(t, output, "nysm test")
}

func TestInvokeCommandDefaultArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewInvokeCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"Demo.run"}) // No --args flag

	err := cmd.Execute()

	require.Error(t, err)
	output := buf.String()
	assert.Contains(t, output, "Demo.run")
	assert.Contains(t, output, "{}") // Default args
}

func TestInvokeCommandMissingAction(t *testing.T) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewInvokeCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{}) // Missing action URI

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg")
}

func TestInvokeHelpText(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewInvokeCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Invoke an action")
	assert.Contains(t, output, "--args")
	assert.Contains(t, output, "action-uri")
}

func TestInvokeCommandInvalidJSON(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewInvokeCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"Cart.addItem", "--args", `{invalid json}`})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --args JSON")
}
