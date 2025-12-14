package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/harness"
)

func TestTestCommandMissingArgs(t *testing.T) {
	buf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{}) // Missing both directories

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 2 arg")
}

func TestTestCommandNonExistentSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	scenariosDir := filepath.Join(tmpDir, "scenarios")
	require.NoError(t, os.MkdirAll(scenariosDir, 0755))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"/nonexistent/specs", scenariosDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specs directory not found")
}

func TestTestCommandNonExistentScenariosDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0755))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir, "/nonexistent/scenarios"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scenarios directory not found")
}

func TestTestCommandEmptyScenariosDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	scenariosDir := filepath.Join(tmpDir, "scenarios")
	require.NoError(t, os.MkdirAll(specsDir, 0755))
	require.NoError(t, os.MkdirAll(scenariosDir, 0755))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir, scenariosDir})

	err := cmd.Execute()
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "No scenarios found")
}

func TestTestCommandEmptyScenariosDirJSON(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	scenariosDir := filepath.Join(tmpDir, "scenarios")
	require.NoError(t, os.MkdirAll(specsDir, 0755))
	require.NoError(t, os.MkdirAll(scenariosDir, 0755))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir, scenariosDir})

	err := cmd.Execute()
	require.NoError(t, err)

	var response CLIResponse
	require.NoError(t, json.Unmarshal(buf.Bytes(), &response))
	assert.Equal(t, "ok", response.Status)
}

func TestTestHelpText(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewTestCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "conformance")
	assert.Contains(t, output, "--update")
	assert.Contains(t, output, "--filter")
	assert.Contains(t, output, "specs-dir")
	assert.Contains(t, output, "scenarios-dir")
}

func TestFindScenarioFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create scenario files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test1.yaml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test2.yml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "ignore.txt"), []byte(""), 0644))

	files, err := findScenarioFiles(tmpDir, "")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestFindScenarioFilesWithFilter(t *testing.T) {
	tmpDir := t.TempDir()

	// Create scenario files
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cart-test.yaml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "cart-add.yaml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "inventory-test.yaml"), []byte(""), 0644))

	files, err := findScenarioFiles(tmpDir, "cart-*")
	require.NoError(t, err)
	assert.Len(t, files, 2)

	// All found files should start with cart-
	for _, f := range files {
		base := filepath.Base(f)
		assert.True(t, len(base) >= 5 && base[:5] == "cart-", "Expected file to start with 'cart-': %s", f)
	}
}

func TestFindScenarioFilesSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create scenario files in root and subdir
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.yaml"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(""), 0644))

	files, err := findScenarioFiles(tmpDir, "")
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestGoldenFilePath(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/path/to/scenario.yaml", "/path/to/golden/scenario.golden"},
		{"/path/to/scenario.yml", "/path/to/golden/scenario.golden"},
		{"scenarios/test.yaml", "scenarios/golden/test.golden"},
	}

	for _, tc := range testCases {
		result := goldenFilePath(tc.input)
		assert.Equal(t, tc.expected, result)
	}
}

func TestConvertTraceToCanonical(t *testing.T) {
	trace := []harness.TraceEvent{
		{
			Type:      "invocation",
			ActionURI: "Cart.addItem",
			Args:      map[string]interface{}{"item": "widget"},
			Seq:       1,
		},
		{
			Type:       "completion",
			OutputCase: "Success",
			Result:     map[string]interface{}{"count": 1},
			Seq:        2,
		},
	}

	result := convertTraceToCanonical(trace)
	assert.Len(t, result, 2)

	// Check first event (invocation)
	inv := result[0].(map[string]any)
	assert.Equal(t, "invocation", inv["type"])
	assert.Equal(t, "Cart.addItem", inv["action_uri"])
	assert.Equal(t, int64(1), inv["seq"])

	// Check second event (completion)
	comp := result[1].(map[string]any)
	assert.Equal(t, "completion", comp["type"])
	assert.Equal(t, "Success", comp["output_case"])
	assert.Equal(t, int64(2), comp["seq"])
}
