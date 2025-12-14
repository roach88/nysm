package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateValidSpecs(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ All specs valid")
}

func TestValidateValidSpecsJSON(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "json"}
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{specsDir})

	err := cmd.Execute()
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
}

func TestValidateNonExistentDirectory(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"/nonexistent/directory/path"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E005") // ErrCodeNotFound
	assert.Contains(t, buf.String(), "not found")
}

func TestValidateEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "E003")
	assert.Contains(t, buf.String(), "no CUE files found")
}

func TestValidateInvalidSpec(t *testing.T) {
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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, buf.String(), "Validation failed")
	assert.Contains(t, buf.String(), "purpose")
}

func TestValidateInvalidSpecJSON(t *testing.T) {
	tmpDir := t.TempDir()

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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)

	var resp CLIResponse
	jsonErr := json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, jsonErr)
	assert.Equal(t, "error", resp.Status)
	assert.NotNil(t, resp.Error)
}

func TestValidateSingleValidConcept(t *testing.T) {
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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ All specs valid")
}

func TestValidateSingleValidSync(t *testing.T) {
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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "✓ All specs valid")
}

func TestValidateVerboseOutput(t *testing.T) {
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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(stdoutBuf)
	cmd.SetErr(stderrBuf) // Verbose output goes to stderr
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verbose logs go to stderr to avoid corrupting JSON output
	verboseOutput := stderrBuf.String()
	assert.Contains(t, verboseOutput, "Found")
	assert.Contains(t, verboseOutput, "CUE file(s)")
	assert.Contains(t, verboseOutput, "Validating concept: Demo")
}

func TestValidateMultipleErrors(t *testing.T) {
	tmpDir := t.TempDir()

	// Concept missing purpose
	spec1 := `
package test

concept: Bad1: {
	action: foo: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "bad1.cue"), []byte(spec1), 0644)
	require.NoError(t, err)

	// Another concept also missing purpose
	spec2 := `
package test

concept: Bad2: {
	action: bar: {
		outputs: []
	}
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "bad2.cue"), []byte(spec2), 0644)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)

	output := buf.String()
	assert.Contains(t, output, "Validation failed")
	// Should contain multiple errors (collected, not fail-fast)
	assert.Contains(t, output, "purpose")
}

func TestValidateFloatRejection(t *testing.T) {
	tmpDir := t.TempDir()

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
	cmd := NewValidateCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{tmpDir})

	err = cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, buf.String(), "float")
	assert.Contains(t, buf.String(), "forbidden")
}

func TestValidateSpecsDir(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	errors, err := ValidateSpecsDir(specsDir)
	require.NoError(t, err)
	assert.Empty(t, errors, "testdata/specs should validate without errors")
}

func TestValidateSpecsDirInvalid(t *testing.T) {
	tmpDir := t.TempDir()

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

	errors, err := ValidateSpecsDir(tmpDir)
	require.NoError(t, err) // Function returns errors in slice, not as error
	assert.NotEmpty(t, errors, "should have validation errors")
}

func TestValidateSpecsDirNonExistent(t *testing.T) {
	_, err := ValidateSpecsDir("/nonexistent/directory")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestMapCompileErrorToCode(t *testing.T) {
	tests := []struct {
		field    string
		expected string
	}{
		{"purpose", "E101"},
		{"action", "E102"},
		{"outputs", "E103"},
		{"type", "E104"},
		{"scope", "E111"},
		{"when", "E110"},
		{"then", "E113"},
		{"where", "E112"},
		{"unknown", "E001"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			code := mapCompileErrorToCode(tt.field)
			assert.Equal(t, tt.expected, code)
		})
	}
}
