package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputFormatter_JSONSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format: "json",
		Writer: buf,
	}

	data := map[string]string{"result": "success"}
	err := formatter.Success(data)
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Status)
	assert.NotNil(t, resp.Data)
}

func TestOutputFormatter_JSONError(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format: "json",
		Writer: buf,
	}

	err := formatter.Error("E001", "compilation failed", nil)
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.NotNil(t, resp.Error)
	assert.Equal(t, "E001", resp.Error.Code)
	assert.Equal(t, "compilation failed", resp.Error.Message)
}

func TestOutputFormatter_JSONErrorWithDetails(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format: "json",
		Writer: buf,
	}

	details := map[string]string{"file": "test.cue", "line": "42"}
	err := formatter.Error("E002", "syntax error", details)
	require.NoError(t, err)

	var resp CLIResponse
	err = json.Unmarshal(buf.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp.Status)
	assert.NotNil(t, resp.Error)
	assert.NotNil(t, resp.Error.Details)
}

func TestOutputFormatter_TextSuccess(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format: "text",
		Writer: buf,
	}

	err := formatter.Success("All specs valid")
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "All specs valid")
}

func TestOutputFormatter_TextError(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format:  "text",
		Writer:  buf,
		Verbose: false,
	}

	err := formatter.Error("E001", "compilation failed", nil)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Error [E001]")
	assert.Contains(t, buf.String(), "compilation failed")
}

func TestOutputFormatter_TextErrorVerbose(t *testing.T) {
	buf := &bytes.Buffer{}
	formatter := &OutputFormatter{
		Format:  "text",
		Writer:  buf,
		Verbose: true,
	}

	details := map[string]string{"file": "test.cue"}
	err := formatter.Error("E001", "compilation failed", details)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "Error [E001]")
	assert.Contains(t, buf.String(), "Details:")
}

func TestOutputFormatter_VerboseLog(t *testing.T) {
	tests := []struct {
		name     string
		verbose  bool
		wantLog  bool
	}{
		{"verbose_enabled", true, true},
		{"verbose_disabled", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			formatter := &OutputFormatter{
				Format:  "text",
				Writer:  buf,
				Verbose: tt.verbose,
			}

			formatter.VerboseLog("Processing %s", "test.cue")

			if tt.wantLog {
				assert.Contains(t, buf.String(), "Processing test.cue")
			} else {
				assert.Empty(t, buf.String())
			}
		})
	}
}

func TestCLIResponse_JSON(t *testing.T) {
	resp := CLIResponse{
		Status: "ok",
		Data:   map[string]int{"count": 42},
	}

	data, err := json.Marshal(resp)
	require.NoError(t, err)

	var decoded CLIResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "ok", decoded.Status)
}

func TestCLIError_JSON(t *testing.T) {
	cliErr := CLIError{
		Code:    "E100",
		Message: "validation failed",
		Details: []string{"missing field: name"},
	}

	data, err := json.Marshal(cliErr)
	require.NoError(t, err)

	var decoded CLIError
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)
	assert.Equal(t, "E100", decoded.Code)
	assert.Equal(t, "validation failed", decoded.Message)
}
