---
story_id: "7.1"
epic_id: "7"
title: "CLI Framework Setup"
status: "done"
created: "2025-12-12"
sprint: "TBD"
points: "3"
---

# Story 7.1: CLI Framework Setup

## Story Statement

As a **developer using NYSM**,
I want **a well-structured CLI with standard commands**,
So that **I can interact with NYSM from the terminal**.

---

## Acceptance Criteria

**Given** the CLI entry point
**When** I run `nysm --help`
**Then** I see:
```
NYSM - Now You See Me

A framework for building legible software with the WYSIWYG pattern.

Usage:
  nysm [command]

Available Commands:
  compile     Compile CUE specs to canonical IR
  validate    Validate specs without full compilation
  run         Start engine with compiled specs
  replay      Replay event log from scratch
  test        Run conformance harness
  trace       Query provenance for a flow

Flags:
  -h, --help      help for nysm
  -v, --verbose   verbose output
  --format        output format (json|text)

Use "nysm [command] --help" for more information about a command.
```

**And** the CLI uses Cobra v1.10.2
**And** all commands follow kebab-case flag convention

### Go Code Signatures

```go
// cmd/nysm/main.go
package main

import (
    "github.com/spf13/cobra"
    "github.com/tyler/nysm/internal/cli"
)

func main() {
    rootCmd := cli.NewRootCommand()
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

```go
// internal/cli/root.go
package cli

import "github.com/spf13/cobra"

type RootOptions struct {
    Verbose bool
    Format  string // "json" | "text"
}

func NewRootCommand() *cobra.Command {
    opts := &RootOptions{}

    cmd := &cobra.Command{
        Use:   "nysm",
        Short: "NYSM - Now You See Me",
        Long:  "A framework for building legible software with the WYSIWYG pattern.",
    }

    // Global flags
    cmd.PersistentFlags().BoolVarP(&opts.Verbose, "verbose", "v", false, "verbose output")
    cmd.PersistentFlags().StringVar(&opts.Format, "format", "text", "output format (json|text)")

    // Add subcommands
    cmd.AddCommand(NewCompileCommand(opts))
    cmd.AddCommand(NewValidateCommand(opts))
    cmd.AddCommand(NewRunCommand(opts))
    cmd.AddCommand(NewReplayCommand(opts))
    cmd.AddCommand(NewTestCommand(opts))
    cmd.AddCommand(NewTraceCommand(opts))

    return cmd
}
```

```go
// internal/cli/compile.go
package cli

import "github.com/spf13/cobra"

type CompileOptions struct {
    *RootOptions
    Output string // output file path
}

func NewCompileCommand(rootOpts *RootOptions) *cobra.Command {
    opts := &CompileOptions{RootOptions: rootOpts}

    cmd := &cobra.Command{
        Use:   "compile <specs-dir>",
        Short: "Compile CUE specs to canonical IR",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runCompile(opts, args[0])
        },
    }

    cmd.Flags().StringVarP(&opts.Output, "output", "o", "", "output file path")

    return cmd
}

func runCompile(opts *CompileOptions, specsDir string) error {
    // Implementation in Story 7.2
    return nil
}
```

```go
// internal/cli/validate.go
package cli

import "github.com/spf13/cobra"

func NewValidateCommand(rootOpts *RootOptions) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "validate <specs-dir>",
        Short: "Validate specs without full compilation",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runValidate(rootOpts, args[0])
        },
    }

    return cmd
}

func runValidate(opts *RootOptions, specsDir string) error {
    // Implementation in Story 7.3
    return nil
}
```

```go
// internal/cli/run.go
package cli

import "github.com/spf13/cobra"

type RunOptions struct {
    *RootOptions
    Database string
}

func NewRunCommand(rootOpts *RootOptions) *cobra.Command {
    opts := &RunOptions{RootOptions: rootOpts}

    cmd := &cobra.Command{
        Use:   "run <specs-dir>",
        Short: "Start engine with compiled specs",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runEngine(opts, args[0])
        },
    }

    cmd.Flags().StringVar(&opts.Database, "db", "./nysm.db", "database path")

    return cmd
}

func runEngine(opts *RunOptions, specsDir string) error {
    // Implementation in Story 7.4
    return nil
}
```

```go
// internal/cli/replay.go
package cli

import "github.com/spf13/cobra"

type ReplayOptions struct {
    *RootOptions
    Database  string
    FlowToken string // optional - specific flow only
}

func NewReplayCommand(rootOpts *RootOptions) *cobra.Command {
    opts := &ReplayOptions{RootOptions: rootOpts}

    cmd := &cobra.Command{
        Use:   "replay",
        Short: "Replay event log from scratch",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runReplay(opts)
        },
    }

    cmd.Flags().StringVar(&opts.Database, "db", "./nysm.db", "database path")
    cmd.Flags().StringVar(&opts.FlowToken, "flow", "", "replay specific flow only")

    return cmd
}

func runReplay(opts *ReplayOptions) error {
    // Implementation in Story 7.5
    return nil
}
```

```go
// internal/cli/test.go
package cli

import "github.com/spf13/cobra"

type TestOptions struct {
    *RootOptions
    Update bool   // regenerate golden files
    Filter string // scenario filter
}

func NewTestCommand(rootOpts *RootOptions) *cobra.Command {
    opts := &TestOptions{RootOptions: rootOpts}

    cmd := &cobra.Command{
        Use:   "test <specs-dir> <scenarios-dir>",
        Short: "Run conformance harness",
        Args:  cobra.ExactArgs(2),
        RunE: func(cmd *cobra.Command, args []string) error {
            return runTests(opts, args[0], args[1])
        },
    }

    cmd.Flags().BoolVar(&opts.Update, "update", false, "regenerate golden files")
    cmd.Flags().StringVar(&opts.Filter, "filter", "", "run subset of scenarios")

    return cmd
}

func runTests(opts *TestOptions, specsDir, scenariosDir string) error {
    // Implementation in Story 7.6
    return nil
}
```

```go
// internal/cli/trace.go
package cli

import "github.com/spf13/cobra"

type TraceOptions struct {
    *RootOptions
    Database string
    FlowToken string
    Action    string // optional - filter to specific action
}

func NewTraceCommand(rootOpts *RootOptions) *cobra.Command {
    opts := &TraceOptions{RootOptions: rootOpts}

    cmd := &cobra.Command{
        Use:   "trace",
        Short: "Query provenance for a flow",
        RunE: func(cmd *cobra.Command, args []string) error {
            return runTrace(opts)
        },
    }

    cmd.Flags().StringVar(&opts.Database, "db", "./nysm.db", "database path")
    cmd.Flags().StringVar(&opts.FlowToken, "flow", "", "flow token to trace")
    cmd.Flags().StringVar(&opts.Action, "action", "", "filter to specific action")
    cmd.MarkFlagRequired("flow")

    return cmd
}

func runTrace(opts *TraceOptions) error {
    // Implementation in Story 7.7
    return nil
}
```

```go
// internal/cli/output.go
package cli

import (
    "encoding/json"
    "fmt"
    "io"
)

// OutputFormatter handles JSON vs text output
type OutputFormatter struct {
    Format  string
    Writer  io.Writer
    Verbose bool
}

// CLIResponse is the standard JSON response format
type CLIResponse struct {
    Status  string      `json:"status"`   // "ok" or "error"
    Data    interface{} `json:"data,omitempty"`
    Error   *CLIError   `json:"error,omitempty"`
    TraceID string      `json:"trace_id,omitempty"`
}

// CLIError is the error structure
type CLIError struct {
    Code    string      `json:"code"`    // "E001", "E002", etc.
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

func (f *OutputFormatter) Success(data interface{}) error {
    if f.Format == "json" {
        return json.NewEncoder(f.Writer).Encode(CLIResponse{
            Status: "ok",
            Data:   data,
        })
    }

    // Human-readable text output
    fmt.Fprintln(f.Writer, data)
    return nil
}

func (f *OutputFormatter) Error(code, message string, details interface{}) error {
    if f.Format == "json" {
        return json.NewEncoder(f.Writer).Encode(CLIResponse{
            Status: "error",
            Error: &CLIError{
                Code:    code,
                Message: message,
                Details: details,
            },
        })
    }

    // Human-readable error
    fmt.Fprintf(f.Writer, "Error [%s]: %s\n", code, message)
    if f.Verbose && details != nil {
        fmt.Fprintf(f.Writer, "Details: %v\n", details)
    }
    return nil
}
```

---

## Quick Reference

### Relevant Patterns from Architecture

| Pattern | Location | Description |
|---------|----------|-------------|
| CLI Framework | `cmd/nysm/main.go` | Cobra entry point |
| Command Structure | `internal/cli/*.go` | One file per command |
| Flag Naming | Architecture naming conventions | kebab-case flags |
| Output Formatting | `internal/cli/output.go` | JSON vs text output |
| Error Codes | Architecture error codes | E001-E399 categories |

### Package Dependencies

```
cmd/nysm/main.go
    └── internal/cli/*.go
            ├── github.com/spf13/cobra v1.10.2
            └── (future: internal/compiler, engine, store, harness)
```

---

## Tasks / Subtasks

### Task 1: Project Initialization
- [ ] Create project directory structure
- [ ] Initialize go.mod with module path
- [ ] Add Cobra dependency
- [ ] Create .gitignore

### Task 2: Root Command Setup
- [ ] Create `cmd/nysm/main.go` entry point
- [ ] Create `internal/cli/root.go` with root command
- [ ] Add global flags (--verbose, --format)
- [ ] Verify `nysm --help` output

### Task 3: Command Stubs
- [ ] Create `internal/cli/compile.go` stub
- [ ] Create `internal/cli/validate.go` stub
- [ ] Create `internal/cli/run.go` stub
- [ ] Create `internal/cli/replay.go` stub
- [ ] Create `internal/cli/test.go` stub
- [ ] Create `internal/cli/trace.go` stub

### Task 4: Output Formatting
- [ ] Create `internal/cli/output.go`
- [ ] Implement OutputFormatter with JSON/text modes
- [ ] Define CLIResponse and CLIError types
- [ ] Add Success() and Error() methods

### Task 5: Build & Smoke Test
- [ ] Build binary: `go build -o nysm ./cmd/nysm`
- [ ] Test `nysm --help`
- [ ] Test `nysm compile --help`
- [ ] Test all command help texts
- [ ] Verify flag parsing

---

## Dev Notes

### Implementation Details

#### Entry Point
```bash
# cmd/nysm/main.go
```
- Minimal main.go - just calls cli.NewRootCommand().Execute()
- All logic in internal/cli/ package

#### Package Structure
```
internal/cli/
├── root.go          # Root command + global flags
├── compile.go       # nysm compile
├── validate.go      # nysm validate
├── run.go           # nysm run
├── replay.go        # nysm replay
├── test.go          # nysm test
├── trace.go         # nysm trace
└── output.go        # JSON/text formatting
```

#### Cobra v1.10.2 Framework
- Use cobra.Command structs
- RunE for error returns
- cobra.ExactArgs(N) for arg validation
- Flags added via cmd.Flags() and cmd.PersistentFlags()

#### Root Command Flags
```go
--help, -h       // Built-in by Cobra
--verbose, -v    // Global verbose flag
--format         // "json" or "text" output
```

#### Available Commands
Each command stub includes:
- Use, Short, Long help text
- Args validation (cobra.ExactArgs, etc.)
- Flags specific to that command
- RunE function (returns error)
- Placeholder implementation returning nil

#### Command-Specific Flags
- **compile**: `--output, -o` (file path)
- **run**: `--db` (database path, default: ./nysm.db)
- **replay**: `--db`, `--flow` (optional flow token)
- **test**: `--update` (regen golden files), `--filter` (scenario subset)
- **trace**: `--db`, `--flow` (required), `--action` (optional filter)

#### Output Formatting
Two modes:
1. **Text (default)**: Human-readable, colored output
2. **JSON**: Machine-readable structured format

JSON format follows standard:
```json
{
  "status": "ok",
  "data": {...}
}

{
  "status": "error",
  "error": {
    "code": "E001",
    "message": "...",
    "details": {...}
  }
}
```

#### Error Code Categories (from Architecture)
- E001-E099: Compilation errors
- E100-E199: Validation errors
- E200-E299: Engine errors
- E300-E399: Store errors

---

## Test Examples

### Manual Testing

```bash
# Build
go build -o nysm ./cmd/nysm

# Test root help
./nysm --help

# Test command help
./nysm compile --help
./nysm validate --help
./nysm run --help
./nysm replay --help
./nysm test --help
./nysm trace --help

# Test JSON output
./nysm --format json compile ./specs

# Test verbose flag
./nysm -v compile ./specs
```

### Unit Tests

```go
// internal/cli/root_test.go
package cli

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestRootCommand(t *testing.T) {
    cmd := NewRootCommand()
    assert.NotNil(t, cmd)
    assert.Equal(t, "nysm", cmd.Use)
    assert.Contains(t, cmd.Long, "WYSIWYG")
}

func TestCommandPresence(t *testing.T) {
    cmd := NewRootCommand()
    commands := []string{"compile", "validate", "run", "replay", "test", "trace"}

    for _, cmdName := range commands {
        subCmd, _, err := cmd.Find([]string{cmdName})
        assert.NoError(t, err, "Command %s should exist", cmdName)
        assert.NotNil(t, subCmd)
    }
}

func TestGlobalFlags(t *testing.T) {
    cmd := NewRootCommand()

    verboseFlag := cmd.PersistentFlags().Lookup("verbose")
    assert.NotNil(t, verboseFlag)
    assert.Equal(t, "v", verboseFlag.Shorthand)

    formatFlag := cmd.PersistentFlags().Lookup("format")
    assert.NotNil(t, formatFlag)
    assert.Equal(t, "text", formatFlag.DefValue)
}
```

```go
// internal/cli/output_test.go
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
```

---

## File List

### New Files
- `cmd/nysm/main.go` - CLI entry point
- `internal/cli/root.go` - Root command + global flags
- `internal/cli/compile.go` - Compile command stub
- `internal/cli/validate.go` - Validate command stub
- `internal/cli/run.go` - Run command stub
- `internal/cli/replay.go` - Replay command stub
- `internal/cli/test.go` - Test command stub
- `internal/cli/trace.go` - Trace command stub
- `internal/cli/output.go` - Output formatting
- `internal/cli/root_test.go` - Root command tests
- `internal/cli/output_test.go` - Output formatter tests

### Modified Files
- `go.mod` - Add Cobra v1.10.2 dependency
- `go.sum` - Dependency checksums (auto-generated)

---

## Relationship to Other Stories

### Depends On
- None (first story in Epic 7)

### Enables
- **Story 7.2: Compile Command** - Uses CLI framework structure
- **Story 7.3: Validate Command** - Uses CLI framework structure
- **Story 7.4: Run Command** - Uses CLI framework structure
- **Story 7.5: Replay Command** - Uses CLI framework structure
- **Story 7.6: Test Command** - Uses CLI framework structure
- **Story 7.7: Trace Command** - Uses CLI framework structure

### Related Stories
- **Epic 1 (Foundation)** - Commands will use IR types
- **Epic 2 (Store)** - run/replay/trace use store
- **Epic 6 (Harness)** - test command uses harness

---

## Story Completion Checklist

### Definition of Done
- [ ] All acceptance criteria met
- [ ] Root command displays correct help text
- [ ] All 6 command stubs created and accessible
- [ ] Global flags (--verbose, --format) work correctly
- [ ] OutputFormatter supports JSON and text modes
- [ ] Unit tests pass with >80% coverage
- [ ] Manual smoke tests successful
- [ ] Binary builds without errors
- [ ] Code follows Architecture naming conventions
- [ ] No golangci-lint warnings

### Pre-Merge Checklist
- [ ] `go build -o nysm ./cmd/nysm` succeeds
- [ ] `./nysm --help` displays correct output
- [ ] All command help texts verified
- [ ] Unit tests pass: `go test ./internal/cli`
- [ ] Code formatted: `go fmt ./...`
- [ ] Vet passes: `go vet ./...`
- [ ] No compiler warnings

---

## References

### Architecture Document
- **Technology Stack** - Go 1.25, Cobra v1.10.2
- **CLI Commands** - Section "CLI Commands"
- **Package Structure** - `internal/cli/` organization
- **Naming Patterns** - CLI flags (kebab-case)
- **Error Codes** - E001-E399 categories
- **Output Formatting** - JSON vs text patterns

### PRD
- **Section 7: Technical Architecture** - CLI mentioned
- **Section 8: MVP Phasing** - CLI is final Epic

### Epic 7
- **Story 7.1 (this)** - CLI Framework Setup
- **Story 7.2-7.7** - Individual command implementations

---

## Dev Agent Record

### Implementation Notes
_Dev agent to record implementation decisions, blockers, and learnings here._

**Status**: Not Started

**Implementation Log**:
```
[Date] [Agent] Action/Decision/Note
```

**Blockers**:
- None currently

**Learnings**:
- _Record any deviations from architecture or interesting findings_
