package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/roach88/nysm/internal/ir"
)

func TestCompileValidSpecs(t *testing.T) {
	// Use testdata/specs directory
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	// Skip if testdata doesn't exist
	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ Compiled")
	assert.Contains(t, output, "concept(s)")
}

func TestCompileValidSpecsJSON(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir})

	err := cmd.Execute()
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.NotNil(t, resp.Data)
}

func TestCompileOutputToFile(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "compiled.json")

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir, "--output", outputFile})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was written
	_, err = os.Stat(outputFile)
	require.NoError(t, err)

	// Verify content is valid JSON
	data, err := os.ReadFile(outputFile)
	require.NoError(t, err)

	var result CompilationResult
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)
	assert.True(t, len(result.Concepts) > 0 || len(result.Syncs) > 0)
}

func TestCompileNonExistentDirectory(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"/nonexistent/directory/path"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E005") // ErrCodeNotFound
	assert.Contains(t, buf.String(), "not found")
}

func TestCompileEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E003")
	assert.Contains(t, buf.String(), "no CUE files found")
}

func TestCompileInvalidSpec(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a CUE file with a concept missing purpose
	invalidSpec := `
package test

concept: Bad: {
	action: foo: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "bad.cue"), []byte(invalidSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "compilation failed")
	assert.Contains(t, buf.String(), "Compilation failed")
	assert.Contains(t, buf.String(), "purpose")
}

func TestCompileInvalidSpecJSON(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a CUE file with a concept missing purpose
	invalidSpec := `
package test

concept: Bad: {
	action: foo: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "bad.cue"), []byte(invalidSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)

	var resp CLIResponse
	jsonErr := json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, jsonErr)
	assert.Equal(t, "error", resp.Status)
	assert.NotNil(t, resp.Error)
	assert.Contains(t, resp.Error.Message, "purpose")
}

func TestCompileSingleConcept(t *testing.T) {
	tmpDir := t.TempDir()

	conceptSpec := `
package test

concept: Calculator: {
	purpose: "Stateless calculations"

	action: add: {
		args: { a: int, b: int }
		outputs: [{ case: "Success", fields: { result: int } }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "calc.cue"), []byte(conceptSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ Compiled 1 concept(s)")
	assert.Contains(t, output, "Calculator")
	assert.Contains(t, output, "1 action(s)")
}

func TestCompileSyncRule(t *testing.T) {
	tmpDir := t.TempDir()

	syncSpec := `
package test

sync: "test-sync": {
	scope: "flow"

	when: {
		action: "Concept.action"
		event:  "completed"
	}

	then: {
		action: "Other.handle"
		args: {}
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "sync.cue"), []byte(syncSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ Compiled 0 concept(s), 1 sync(s)")
	assert.Contains(t, output, "test-sync")
	assert.Contains(t, output, "Concept.action")
	assert.Contains(t, output, "Other.handle")
}

func TestCompileConceptAndSync(t *testing.T) {
	tmpDir := t.TempDir()

	// Create concept file
	conceptSpec := `
package test

concept: Service: {
	purpose: "Test service"

	action: process: {
		args: { id: string }
		outputs: [{ case: "Success", fields: { result: string } }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "service.cue"), []byte(conceptSpec), 0644)
	require.NoError(t, err)

	// Create sync file
	syncSpec := `
package test

sync: "service-sync": {
	scope: "flow"

	when: {
		action: "Service.process"
		event:  "completed"
		case:   "Success"
	}

	then: {
		action: "Logger.log"
		args: {
			message: "bound.result"
		}
	}
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "sync.cue"), []byte(syncSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)

	// Data should contain concepts and syncs
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok)
	concepts, ok := dataMap["concepts"].([]interface{})
	require.True(t, ok)
	assert.Len(t, concepts, 1)
	syncs, ok := dataMap["syncs"].([]interface{})
	require.True(t, ok)
	assert.Len(t, syncs, 1)
}

func TestCompileVerboseOutput(t *testing.T) {
	tmpDir := t.TempDir()

	conceptSpec := `
package test

concept: Demo: {
	purpose: "Demo concept"

	action: run: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "demo.cue"), []byte(conceptSpec), 0644)
	require.NoError(t, err)

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text", Verbose: true}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(stderrBuf) // Verbose output goes to stderr
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verbose logs go to stderr to avoid corrupting JSON output
	verboseOutput := stderrBuf.String()
	assert.Contains(t, verboseOutput, "Found")
	assert.Contains(t, verboseOutput, "CUE file(s)")
	assert.Contains(t, verboseOutput, "Compiling concept: Demo")
}

func TestCompileFloatRejection(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a CUE file with forbidden float type
	floatSpec := `
package test

concept: Bad: {
	purpose: "Has float"

	state: Price: {
		value: float
	}

	action: buy: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "float.cue"), []byte(floatSpec), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewCompileCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "float")
	assert.Contains(t, buf.String(), "forbidden")
}

func TestFindCUEFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directories with CUE files
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.MkdirAll(subDir, 0755)
	require.NoError(t, err)

	// Create files
	err = os.WriteFile(filepath.Join(tmpDir, "root.cue"), []byte("package test"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "notcue.txt"), []byte("not a cue file"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(subDir, "nested.cue"), []byte("package test"), 0644)
	require.NoError(t, err)

	files, err := FindCUEFiles(tmpDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestMapFieldToErrorCode(t *testing.T) {
	tests := []struct {
		field    string
		expected string
	}{
		{"purpose", ErrCodeConceptPurpose},   // E101
		{"action", ErrCodeConceptActions},    // E102
		{"type", ErrCodeInvalidType},         // E104
		{"scope", ErrCodeInvalidScope},       // E111
		{"when", ErrCodeInvalidWhen},         // E110
		{"then", ErrCodeInvalidThen},         // E113
		{"where", ErrCodeInvalidWhere},       // E112
		{"outputs", ErrCodeActionOutputs},    // E103
		{"unknown", ErrCodeGeneric},          // E001
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			code := MapFieldToErrorCode(tt.field)
			assert.Equal(t, tt.expected, code)
		})
	}
}

func TestCalculateStats(t *testing.T) {
	result := &CompilationResult{
		Concepts: []ir.ConceptSpec{
			{
				Name:    "A",
				Actions: []ir.ActionSig{{Name: "a1"}, {Name: "a2"}},
				OperationalPrinciples: []ir.OperationalPrinciple{{Description: "op1"}},
			},
			{
				Name:    "B",
				Actions: []ir.ActionSig{{Name: "b1"}},
			},
		},
		Syncs: []ir.SyncRule{
			{ID: "s1"},
			{ID: "s2"},
		},
	}

	stats := calculateStats(result)

	assert.Equal(t, 2, stats.ConceptCount)
	assert.Equal(t, 2, stats.SyncCount)
	assert.Equal(t, 3, stats.TotalActions)
	assert.Equal(t, 1, stats.TotalOperationalPrinciples)
}
