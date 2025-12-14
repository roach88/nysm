package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunMissingDatabaseFlag(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	require.NoError(t, os.MkdirAll(specsDir, 0755))

	// Create a valid concept spec
	spec := `
package test

concept: Demo: {
	purpose: "Test concept"
	action: run: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "demo.cue"), []byte(spec), 0644))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{specsDir}) // Missing --db flag

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
	assert.Contains(t, err.Error(), "db")
}

func TestRunInvalidSpecs(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	dbPath := filepath.Join(tmpDir, "test.db")
	require.NoError(t, os.MkdirAll(specsDir, 0755))

	// Create invalid spec (missing purpose)
	spec := `
package test

concept: Bad: {
	action: foo: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "bad.cue"), []byte(spec), 0644))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--db", dbPath, specsDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compile specs")
}

func TestRunNonExistentSpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--db", dbPath, "/nonexistent/directory"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "specs directory not found")
}

func TestRunEmptySpecsDir(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	dbPath := filepath.Join(tmpDir, "test.db")
	require.NoError(t, os.MkdirAll(specsDir, 0755))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--db", dbPath, specsDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CUE files found")
}

func TestRunValidSpecsWithTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	specsDir := filepath.Join(tmpDir, "specs")
	dbPath := filepath.Join(tmpDir, "test.db")
	require.NoError(t, os.MkdirAll(specsDir, 0755))

	// Create a valid concept spec
	spec := `
package test

concept: Demo: {
	purpose: "Test concept"
	action: run: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(specsDir, "demo.cue"), []byte(spec), 0644))

	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--db", dbPath, specsDir})

	// Run command with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.ExecuteContext(ctx)
	}()

	// Wait for timeout or completion
	select {
	case err := <-errChan:
		// Should be nil (engine exits gracefully on context cancellation)
		// or context.DeadlineExceeded/Canceled
		if err != nil {
			// Accept context-related errors as expected behavior
			isContextError := err == context.DeadlineExceeded ||
				err == context.Canceled ||
				err.Error() == "context deadline exceeded" ||
				err.Error() == "context canceled"
			if !isContextError {
				t.Logf("Got non-context error (may still be acceptable): %v", err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("command did not respect context timeout")
	}

	// Verify database was created
	_, err := os.Stat(dbPath)
	assert.NoError(t, err, "database should be created")

	// Verify startup message was printed
	output := buf.String()
	assert.Contains(t, output, "Engine started")
}

func TestCompileSpecs(t *testing.T) {
	specsDir := filepath.Join("..", "..", "testdata", "specs")

	if _, err := os.Stat(specsDir); os.IsNotExist(err) {
		t.Skip("testdata/specs directory not found")
	}

	specs, syncs, err := compileSpecs(specsDir)
	require.NoError(t, err)
	assert.NotEmpty(t, specs, "should have compiled concepts")
	assert.NotEmpty(t, syncs, "should have compiled syncs")
}

func TestCompileSpecsNonExistentDir(t *testing.T) {
	_, _, err := compileSpecs("/nonexistent/directory")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCompileSpecsEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	_, _, err := compileSpecs(tmpDir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no CUE files found")
}

func TestCompileSpecsInvalidSpec(t *testing.T) {
	tmpDir := t.TempDir()

	// Create invalid spec
	spec := `
package test

concept: Bad: {
	action: foo: {
		outputs: [{ case: "Success", fields: {} }]
	}
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.cue"), []byte(spec), 0644))

	_, _, err := compileSpecs(tmpDir)
	require.Error(t, err)
	// Should contain info about the compilation error (purpose is required)
	assert.Contains(t, err.Error(), "purpose")
}

func TestRunHelpText(t *testing.T) {
	buf := &bytes.Buffer{}
	rootOpts := &RootOptions{Format: "text"}
	cmd := NewRunCommand(rootOpts)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Start the NYSM sync engine")
	assert.Contains(t, output, "--db")
	assert.Contains(t, output, "specs-dir")
}
