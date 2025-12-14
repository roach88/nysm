package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd)
	assert.Equal(t, "nysm", cmd.Use)
	assert.Contains(t, cmd.Long, "WYSIWYG")
}

func TestCommandPresence(t *testing.T) {
	cmd := NewRootCommand()
	commands := []string{"compile", "validate", "run", "invoke", "replay", "test", "trace"}

	for _, cmdName := range commands {
		t.Run(cmdName, func(t *testing.T) {
			subCmd, _, err := cmd.Find([]string{cmdName})
			require.NoError(t, err, "Command %s should exist", cmdName)
			require.NotNil(t, subCmd)
			assert.Equal(t, cmdName, subCmd.Name())
		})
	}
}

func TestGlobalFlags(t *testing.T) {
	cmd := NewRootCommand()

	verboseFlag := cmd.PersistentFlags().Lookup("verbose")
	require.NotNil(t, verboseFlag)
	assert.Equal(t, "v", verboseFlag.Shorthand)
	assert.Equal(t, "false", verboseFlag.DefValue)

	formatFlag := cmd.PersistentFlags().Lookup("format")
	require.NotNil(t, formatFlag)
	assert.Equal(t, "text", formatFlag.DefValue)
}

func TestCompileCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	compileCmd, _, err := cmd.Find([]string{"compile"})
	require.NoError(t, err)

	outputFlag := compileCmd.Flags().Lookup("output")
	require.NotNil(t, outputFlag)
	assert.Equal(t, "o", outputFlag.Shorthand)
}

func TestRunCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	runCmd, _, err := cmd.Find([]string{"run"})
	require.NoError(t, err)

	dbFlag := runCmd.Flags().Lookup("db")
	require.NotNil(t, dbFlag)
	// --db is required, so default is empty
	assert.Equal(t, "", dbFlag.DefValue)
}

func TestInvokeCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	invokeCmd, _, err := cmd.Find([]string{"invoke"})
	require.NoError(t, err)

	argsFlag := invokeCmd.Flags().Lookup("args")
	require.NotNil(t, argsFlag)
	assert.Equal(t, "{}", argsFlag.DefValue)
}

func TestReplayCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	replayCmd, _, err := cmd.Find([]string{"replay"})
	require.NoError(t, err)

	dbFlag := replayCmd.Flags().Lookup("db")
	require.NotNil(t, dbFlag)

	flowFlag := replayCmd.Flags().Lookup("flow")
	require.NotNil(t, flowFlag)
}

func TestTestCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	testCmd, _, err := cmd.Find([]string{"test"})
	require.NoError(t, err)

	updateFlag := testCmd.Flags().Lookup("update")
	require.NotNil(t, updateFlag)
	assert.Equal(t, "false", updateFlag.DefValue)

	filterFlag := testCmd.Flags().Lookup("filter")
	require.NotNil(t, filterFlag)
}

func TestTraceCommandFlags(t *testing.T) {
	cmd := NewRootCommand()
	traceCmd, _, err := cmd.Find([]string{"trace"})
	require.NoError(t, err)

	dbFlag := traceCmd.Flags().Lookup("db")
	require.NotNil(t, dbFlag)

	flowFlag := traceCmd.Flags().Lookup("flow")
	require.NotNil(t, flowFlag)

	actionFlag := traceCmd.Flags().Lookup("action")
	require.NotNil(t, actionFlag)
}

func TestCommandHelp(t *testing.T) {
	cmd := NewRootCommand()

	// Verify help text contains key elements
	assert.Contains(t, cmd.Short, "NYSM")
	assert.Contains(t, cmd.Long, "legible software")
}

func TestFormatValidation(t *testing.T) {
	// Test valid formats
	assert.True(t, isValidFormat("text"))
	assert.True(t, isValidFormat("json"))

	// Test invalid formats
	assert.False(t, isValidFormat("xml"))
	assert.False(t, isValidFormat(""))
	assert.False(t, isValidFormat("TEXT"))
}

func TestFormatValidationIntegration(t *testing.T) {
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"--format", "invalid", "compile", "."})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid format")
}
