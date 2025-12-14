# Story 7.4: Run Command

Status: done

## Story

As a **developer running NYSM apps**,
I want **a run command that starts the engine**,
So that **I can execute flows against my specs**.

## Acceptance Criteria

1. **Command signature in `internal/cli/run.go`**
   ```go
   func NewRunCommand() *cobra.Command {
       cmd := &cobra.Command{
           Use:   "run --db <database-path> <specs-dir>",
           Short: "Start the NYSM engine with compiled specs",
           Long: `Start the NYSM engine with compiled specs.

   The engine loads concept specs and sync rules from the specified directory,
   initializes a SQLite database (creating it if it doesn't exist), and starts
   the single-writer event loop. The engine listens for invocations via the
   'nysm invoke' subcommand.

   Example:
     nysm run --db ./nysm.db ./specs
     nysm run --db /tmp/test.db ./demo-specs --verbose
   `,
           Args: cobra.ExactArgs(1),
           RunE: runEngine,
       }

       cmd.Flags().String("db", "", "Path to SQLite database (required)")
       cmd.MarkFlagRequired("db")
       cmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging")

       return cmd
   }
   ```
   - `--db` flag specifies database path (required)
   - `--verbose` flag enables debug logging
   - Single positional argument for specs directory
   - Clear help text with examples

2. **Engine startup in runEngine()**
   ```go
   func runEngine(cmd *cobra.Command, args []string) error {
       specsDir := args[0]
       dbPath, _ := cmd.Flags().GetString("db")
       verbose, _ := cmd.Flags().GetBool("verbose")

       // Configure logging
       logLevel := slog.LevelInfo
       if verbose {
           logLevel = slog.LevelDebug
       }
       handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
           Level: logLevel,
       })
       slog.SetDefault(slog.New(handler))

       // Compile specs
       slog.Info("compiling specs", "dir", specsDir)
       specs, syncs, err := compileSpecs(specsDir)
       if err != nil {
           return fmt.Errorf("failed to compile specs: %w", err)
       }
       slog.Info("specs compiled", "concepts", len(specs), "syncs", len(syncs))

       // Open database (create if not exists)
       slog.Info("opening database", "path", dbPath)
       st, err := store.Open(dbPath)
       if err != nil {
           return fmt.Errorf("failed to open database: %w", err)
       }
       defer st.Close()
       slog.Info("database ready")

       // Create engine
       eng := engine.New(st, specs, syncs)

       // Setup signal handling for graceful shutdown
       ctx, cancel := context.WithCancel(context.Background())
       defer cancel()

       sigChan := make(chan os.Signal, 1)
       signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

       go func() {
           sig := <-sigChan
           slog.Info("received signal, shutting down", "signal", sig)
           cancel()
       }()

       // Start engine
       slog.Info("engine starting", "db", dbPath, "specs_dir", specsDir)
       fmt.Println("Engine started. Listening for invocations...")
       fmt.Println("Press Ctrl-C to stop.")

       if err := eng.Run(ctx); err != nil && err != context.Canceled {
           return fmt.Errorf("engine error: %w", err)
       }

       slog.Info("engine stopped gracefully")
       return nil
   }
   ```
   - Compiles specs from directory
   - Creates database if doesn't exist
   - Initializes engine with compiled specs
   - Sets up signal handling for Ctrl-C
   - Starts engine event loop
   - Graceful shutdown on signal

3. **Spec compilation helper**
   ```go
   func compileSpecs(dir string) ([]ir.ConceptSpec, []ir.SyncRule, error) {
       // Load all .cue files from directory
       conceptFiles, err := filepath.Glob(filepath.Join(dir, "*.concept.cue"))
       if err != nil {
           return nil, nil, fmt.Errorf("failed to list concept files: %w", err)
       }

       syncFiles, err := filepath.Glob(filepath.Join(dir, "*.sync.cue"))
       if err != nil {
           return nil, nil, fmt.Errorf("failed to list sync files: %w", err)
       }

       if len(conceptFiles) == 0 {
           return nil, nil, fmt.Errorf("no concept files found in %s", dir)
       }

       // Compile concepts
       var specs []ir.ConceptSpec
       for _, file := range conceptFiles {
           spec, err := compiler.CompileConcept(file)
           if err != nil {
               return nil, nil, fmt.Errorf("failed to compile %s: %w", file, err)
           }
           specs = append(specs, spec)
       }

       // Compile syncs
       var syncs []ir.SyncRule
       for _, file := range syncFiles {
           sync, err := compiler.CompileSync(file)
           if err != nil {
               return nil, nil, fmt.Errorf("failed to compile %s: %w", file, err)
           }
           syncs = append(syncs, sync)
       }

       return specs, syncs, nil
   }
   ```
   - Discovers .concept.cue and .sync.cue files
   - Compiles each file via compiler package
   - Returns compiled IR types
   - Clear error messages with file context

4. **Database creation if not exists**
   - `store.Open()` creates database file automatically
   - Runs schema migrations on first open
   - Subsequent opens use existing schema
   - No special flags needed

5. **Invoke subcommand stub in `internal/cli/invoke.go`**
   ```go
   func NewInvokeCommand() *cobra.Command {
       cmd := &cobra.Command{
           Use:   "invoke <action-uri> --args <json>",
           Short: "Invoke an action on the running engine",
           Long: `Invoke an action on the running engine.

   For MVP, this sends an invocation to the engine via a simple mechanism
   (file-based or in-memory for same process). Future versions will use HTTP.

   Example:
     nysm invoke Cart.addItem --args '{"item_id":"widget","quantity":3}'
   `,
           Args: cobra.ExactArgs(1),
           RunE: invokeAction,
       }

       cmd.Flags().String("args", "{}", "Action arguments as JSON")

       return cmd
   }

   func invokeAction(cmd *cobra.Command, args []string) error {
       actionURI := args[0]
       argsJSON, _ := cmd.Flags().GetString("args")

       // TODO: For MVP, this is a stub that prints instructions
       // Future: Send invocation to running engine via IPC/HTTP

       fmt.Printf("Invocation request:\n")
       fmt.Printf("  Action: %s\n", actionURI)
       fmt.Printf("  Args: %s\n", argsJSON)
       fmt.Println("\nNote: For MVP, the engine must be run in the same process.")
       fmt.Println("Use the test harness (nysm test) to execute flows.")

       return fmt.Errorf("invoke subcommand not yet implemented - use 'nysm test' for MVP")
   }
   ```
   - Placeholder for invocation API
   - Clear message directing to test harness
   - Documents future HTTP adapter plan

6. **Signal handling tests verify graceful shutdown**
   - Engine stops cleanly on SIGINT/SIGTERM
   - Database closed properly
   - No goroutine leaks
   - Context canceled propagates to engine

7. **User output formatting**
   - Human-friendly status messages
   - Structured logging with slog for debugging
   - Clear startup confirmation
   - Shutdown message on signal

## Quick Reference

| Pattern | Key Rule |
|---------|----------|
| **CLI Framework** | Cobra for command structure, slog for logging |
| **Database** | SQLite created on first open, schema.sql migrations |
| **Signal Handling** | os.Signal + context.Context for graceful shutdown |
| **Spec Discovery** | filepath.Glob for .concept.cue and .sync.cue files |

## Tasks / Subtasks

- [ ] Task 1: Create run command structure (AC: #1)
  - [ ] 1.1 Create `internal/cli/run.go`
  - [ ] 1.2 Implement NewRunCommand() with flags
  - [ ] 1.3 Add help text and examples
  - [ ] 1.4 Validate required --db flag

- [ ] Task 2: Implement compileSpecs() helper (AC: #3)
  - [ ] 2.1 Add filepath.Glob for .concept.cue files
  - [ ] 2.2 Add filepath.Glob for .sync.cue files
  - [ ] 2.3 Call compiler.CompileConcept() for each concept file
  - [ ] 2.4 Call compiler.CompileSync() for each sync file
  - [ ] 2.5 Return compiled specs and syncs
  - [ ] 2.6 Write tests for spec discovery and compilation

- [ ] Task 3: Implement runEngine() (AC: #2)
  - [ ] 3.1 Extract flags (db, verbose, specs-dir)
  - [ ] 3.2 Configure slog based on verbose flag
  - [ ] 3.3 Call compileSpecs() to load specs
  - [ ] 3.4 Call store.Open() to create/open database
  - [ ] 3.5 Create engine with engine.New()
  - [ ] 3.6 Setup signal handling (os.Interrupt, SIGTERM)
  - [ ] 3.7 Start engine.Run() with context
  - [ ] 3.8 Handle graceful shutdown on signal
  - [ ] 3.9 Write tests for runEngine flow

- [ ] Task 4: Implement signal handling (AC: #6)
  - [ ] 4.1 Create signal channel with os.Signal
  - [ ] 4.2 Register SIGINT and SIGTERM handlers
  - [ ] 4.3 Cancel context on signal received
  - [ ] 4.4 Log shutdown message
  - [ ] 4.5 Write tests for signal handling (with fake signals)

- [ ] Task 5: Create invoke command stub (AC: #5)
  - [ ] 5.1 Create `internal/cli/invoke.go`
  - [ ] 5.2 Implement NewInvokeCommand() with flags
  - [ ] 5.3 Add placeholder invokeAction() function
  - [ ] 5.4 Print helpful message directing to test harness
  - [ ] 5.5 Document future HTTP adapter plan

- [ ] Task 6: Add commands to root CLI (AC: #7)
  - [ ] 6.1 Import run and invoke commands in `cmd/nysm/main.go`
  - [ ] 6.2 Add run command to root
  - [ ] 6.3 Add invoke command to root
  - [ ] 6.4 Verify `nysm --help` shows both commands

- [ ] Task 7: Write integration tests
  - [ ] 7.1 Test run command with valid specs
  - [ ] 7.2 Test run command with missing database path (error)
  - [ ] 7.3 Test run command with invalid specs (compilation error)
  - [ ] 7.4 Test graceful shutdown on signal
  - [ ] 7.5 Test invoke command stub (prints helpful message)
  - [ ] 7.6 Verify no goroutine leaks with goleak

## Dev Notes

### Run Command Flow

```
┌──────────────────┐
│  nysm run        │
│  --db ./nysm.db  │
│  ./specs         │
└────────┬─────────┘
         │
         ▼
┌─────────────────────────────┐
│ 1. Compile Specs            │
│    - Discover .concept.cue  │
│    - Discover .sync.cue     │
│    - Compile to IR          │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 2. Open Database            │
│    - Create if not exists   │
│    - Run schema migrations  │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 3. Create Engine            │
│    - engine.New(store, ...) │
│    - Specs + Syncs loaded   │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 4. Setup Signal Handling    │
│    - SIGINT (Ctrl-C)        │
│    - SIGTERM                │
│    - Cancel context on sig  │
└────────┬────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ 5. Start Engine Loop        │
│    - engine.Run(ctx)        │
│    - Blocks until canceled  │
│    - Processes invocations  │
└────────┬────────────────────┘
         │
         ▼ (on signal)
┌─────────────────────────────┐
│ 6. Graceful Shutdown        │
│    - Context canceled       │
│    - Engine stops cleanly   │
│    - Database closed        │
└─────────────────────────────┘
```

### Database Creation

The `store.Open(path)` function handles database creation automatically:

```go
// internal/store/store.go (from Story 2.1)
func Open(path string) (*Store, error) {
    // Create parent directory if needed
    dir := filepath.Dir(path)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create db directory: %w", err)
    }

    // Open SQLite (creates file if not exists)
    db, err := sql.Open("sqlite3", path)
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %w", err)
    }

    // Apply WAL mode and migrations
    if err := applyPragmas(db); err != nil {
        return nil, err
    }

    if err := runMigrations(db); err != nil {
        return nil, err
    }

    return &Store{db: db}, nil
}
```

**Key Points:**
- Creates parent directories if needed
- Creates database file on first open
- Applies WAL mode and pragmas
- Runs schema migrations idempotently
- Safe to call multiple times (idempotent)

### Invoke Subcommand (MVP Stub)

For MVP, the `nysm invoke` command is a placeholder. The actual invocation mechanism is:

**MVP Approach:**
- Use the conformance harness (`nysm test`) to execute flows
- Scenarios defined in YAML files
- Deterministic flow tokens for golden testing

**Future (Phase 6):**
- HTTP adapter for external invocations
- Engine listens on port (e.g., :8080)
- `nysm invoke` sends HTTP POST
- Real-time invocation/completion streaming

**Why stub for MVP:**
- HTTP adapter complexity out of MVP scope (per PRD)
- Test harness sufficient for demo validation
- Focus on core engine correctness first

### Signal Handling Implementation

```go
// Setup signal handling for graceful shutdown
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

go func() {
    sig := <-sigChan
    slog.Info("received signal, shutting down", "signal", sig)
    cancel()
}()

// Start engine (blocks until context canceled)
if err := eng.Run(ctx); err != nil && err != context.Canceled {
    return fmt.Errorf("engine error: %w", err)
}
```

**Critical Points:**
- Buffered channel (size 1) prevents signal loss
- Notify on SIGINT (Ctrl-C) and SIGTERM (docker stop)
- Goroutine waits for signal, then cancels context
- Engine.Run() checks ctx.Done() in event loop
- Clean return on context.Canceled (not an error)

### Logging Configuration

```go
// Configure logging based on --verbose flag
logLevel := slog.LevelInfo
if verbose {
    logLevel = slog.LevelDebug
}

handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
    Level: logLevel,
})
slog.SetDefault(slog.New(handler))
```

**Log Levels:**
- `Info` (default): Startup, shutdown, major events
- `Debug` (--verbose): Spec compilation details, event processing, sync matching

**Example Output:**
```
$ nysm run --db ./nysm.db ./specs
2025-12-12T10:30:00.000Z INFO compiling specs dir=./specs
2025-12-12T10:30:00.100Z INFO specs compiled concepts=3 syncs=2
2025-12-12T10:30:00.150Z INFO opening database path=./nysm.db
2025-12-12T10:30:00.200Z INFO database ready
2025-12-12T10:30:00.250Z INFO engine starting db=./nysm.db specs_dir=./specs
Engine started. Listening for invocations...
Press Ctrl-C to stop.
^C
2025-12-12T10:35:00.000Z INFO received signal, shutting down signal=interrupt
2025-12-12T10:35:00.050Z INFO engine stopped gracefully
```

### Error Handling

**Compilation Errors:**
```bash
$ nysm run --db ./nysm.db ./bad-specs
2025-12-12T10:30:00.000Z INFO compiling specs dir=./bad-specs
Error: failed to compile specs: failed to compile cart.concept.cue: missing required field: purpose
```

**Database Errors:**
```bash
$ nysm run --db /read-only/nysm.db ./specs
2025-12-12T10:30:00.000Z INFO compiling specs dir=./specs
2025-12-12T10:30:00.100Z INFO specs compiled concepts=3 syncs=2
2025-12-12T10:30:00.150Z INFO opening database path=/read-only/nysm.db
Error: failed to open database: failed to create db directory: mkdir /read-only: permission denied
```

**Missing Specs:**
```bash
$ nysm run --db ./nysm.db ./empty-dir
2025-12-12T10:30:00.000Z INFO compiling specs dir=./empty-dir
Error: failed to compile specs: no concept files found in ./empty-dir
```

## Test Examples

### Test: Run command with valid specs

```go
func TestRunCommand_ValidSpecs(t *testing.T) {
    defer goleak.VerifyNone(t)

    // Setup
    tempDir := t.TempDir()
    dbPath := filepath.Join(tempDir, "test.db")
    specsDir := filepath.Join(tempDir, "specs")

    // Create test specs
    os.MkdirAll(specsDir, 0755)
    os.WriteFile(
        filepath.Join(specsDir, "cart.concept.cue"),
        []byte(`concept Cart { purpose: "test" }`),
        0644,
    )

    // Create command
    cmd := NewRunCommand()
    cmd.SetArgs([]string{"--db", dbPath, specsDir})

    // Run in goroutine with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
    defer cancel()

    errChan := make(chan error, 1)
    go func() {
        errChan <- cmd.ExecuteContext(ctx)
    }()

    // Wait for timeout or completion
    select {
    case err := <-errChan:
        assert.ErrorIs(t, err, context.DeadlineExceeded)
    case <-time.After(1 * time.Second):
        t.Fatal("command did not respect context timeout")
    }

    // Verify database created
    _, err := os.Stat(dbPath)
    assert.NoError(t, err, "database should be created")
}
```

### Test: Run command with missing database path

```go
func TestRunCommand_MissingDatabasePath(t *testing.T) {
    tempDir := t.TempDir()
    specsDir := filepath.Join(tempDir, "specs")
    os.MkdirAll(specsDir, 0755)

    cmd := NewRunCommand()
    cmd.SetArgs([]string{specsDir}) // Missing --db flag

    err := cmd.Execute()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "required flag(s) \"db\" not set")
}
```

### Test: Run command with invalid specs

```go
func TestRunCommand_InvalidSpecs(t *testing.T) {
    tempDir := t.TempDir()
    dbPath := filepath.Join(tempDir, "test.db")
    specsDir := filepath.Join(tempDir, "specs")

    // Create invalid spec (missing purpose)
    os.MkdirAll(specsDir, 0755)
    os.WriteFile(
        filepath.Join(specsDir, "cart.concept.cue"),
        []byte(`concept Cart { state: {} }`), // Missing purpose
        0644,
    )

    cmd := NewRunCommand()
    cmd.SetArgs([]string{"--db", dbPath, specsDir})

    err := cmd.Execute()
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "failed to compile specs")
}
```

### Test: Graceful shutdown on signal

```go
func TestRunCommand_GracefulShutdown(t *testing.T) {
    defer goleak.VerifyNone(t)

    tempDir := t.TempDir()
    dbPath := filepath.Join(tempDir, "test.db")
    specsDir := filepath.Join(tempDir, "specs")

    // Create test specs
    os.MkdirAll(specsDir, 0755)
    os.WriteFile(
        filepath.Join(specsDir, "cart.concept.cue"),
        []byte(`concept Cart { purpose: "test" }`),
        0644,
    )

    cmd := NewRunCommand()
    cmd.SetArgs([]string{"--db", dbPath, specsDir})

    // Start command in goroutine
    errChan := make(chan error, 1)
    go func() {
        errChan <- cmd.Execute()
    }()

    // Give engine time to start
    time.Sleep(100 * time.Millisecond)

    // Send interrupt signal
    proc, _ := os.FindProcess(os.Getpid())
    proc.Signal(os.Interrupt)

    // Wait for graceful shutdown
    err := <-errChan
    assert.NoError(t, err, "should shutdown gracefully on signal")
}
```

### Test: Invoke command stub

```go
func TestInvokeCommand_Stub(t *testing.T) {
    cmd := NewInvokeCommand()
    cmd.SetArgs([]string{"Cart.addItem", "--args", `{"item_id":"widget"}`})

    // Capture stdout
    oldStdout := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w

    err := cmd.Execute()

    w.Close()
    os.Stdout = oldStdout

    var buf bytes.Buffer
    io.Copy(&buf, r)
    output := buf.String()

    // Should error (not implemented)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "not yet implemented")

    // Should print helpful message
    assert.Contains(t, output, "Invocation request")
    assert.Contains(t, output, "Cart.addItem")
    assert.Contains(t, output, "use 'nysm test' for MVP")
}
```

## File List

Files to create or modify:

1. `internal/cli/run.go` - Run command implementation
2. `internal/cli/invoke.go` - Invoke command stub
3. `internal/cli/run_test.go` - Run command tests
4. `internal/cli/invoke_test.go` - Invoke command tests
5. `cmd/nysm/main.go` - Add run and invoke commands to root

## Relationship to Other Stories

**Dependencies:**
- Story 1.6 (CUE Concept Spec Parser) - Required for compileSpecs()
- Story 1.7 (CUE Sync Rule Parser) - Required for compileSpecs()
- Story 2.1 (SQLite Store Initialization) - Required for store.Open()
- Story 3.1 (Single-Writer Event Loop) - Required for engine.New() and engine.Run()
- Story 7.1 (CLI Framework Setup) - Required for Cobra command structure

**Enables:**
- Story 7.5 (Replay Command) - Uses same database and specs loading
- Story 7.6 (Test Command) - Similar spec compilation flow
- Story 7.7 (Trace Command) - Traces flows from running engine database

**Blocks:**
- Story 7.10 (Demo Scenarios and Golden Traces) - Needs running engine for validation

**Note:** This story provides the primary CLI entry point for running NYSM applications. The invoke subcommand is a stub for MVP; actual invocation happens via the test harness until Phase 6 HTTP adapter.

## Story Completion Checklist

- [ ] `internal/cli/run.go` created with NewRunCommand()
- [ ] runEngine() implements full startup flow
- [ ] compileSpecs() discovers and compiles .cue files
- [ ] Signal handling setup for SIGINT and SIGTERM
- [ ] Graceful shutdown on context cancellation
- [ ] Database created if not exists
- [ ] Engine started with compiled specs
- [ ] Verbose flag controls log level (slog)
- [ ] Clear user-facing output messages
- [ ] `internal/cli/invoke.go` created with stub implementation
- [ ] NewInvokeCommand() defines command structure
- [ ] invokeAction() prints helpful message for MVP
- [ ] Both commands added to root CLI in `cmd/nysm/main.go`
- [ ] All tests pass (`go test ./internal/cli/...`)
- [ ] `nysm run --help` shows correct usage
- [ ] `nysm invoke --help` shows correct usage
- [ ] `go build ./cmd/nysm` produces working binary
- [ ] Integration test: run with valid specs (starts and stops)
- [ ] Integration test: run with missing --db flag (error)
- [ ] Integration test: run with invalid specs (compilation error)
- [ ] Integration test: graceful shutdown on SIGINT
- [ ] Integration test: invoke stub prints message
- [ ] No goroutine leaks verified with goleak

## References

- [Source: docs/epics.md#Story 7.4] - Story definition and acceptance criteria
- [Source: docs/architecture.md#CLI Commands] - Command structure and conventions
- [Source: docs/architecture.md#Concurrency Model] - Single-writer event loop
- [Source: docs/prd.md#FR-5.1] - Durable engine requirements
- [Source: docs/prd.md#Phase 3] - Durable engine and crash recovery
- [Source: docs/prd.md#Out of Scope] - HTTP adapter deferred to Phase 6

## Dev Agent Record

### Agent Model Used

Claude Sonnet 4.5 (claude-sonnet-4-5-20250929)

### Validation History

- Initial creation: Story creation from epics.md Story 7.4 definition

### Completion Notes

- Run command is the primary entry point for NYSM applications
- Database creation is automatic via store.Open() from Story 2.1
- Spec compilation uses compiler package from Stories 1.6 and 1.7
- Engine startup uses engine.New() and engine.Run() from Story 3.1
- Signal handling provides graceful Ctrl-C shutdown
- Invoke subcommand is a stub for MVP (HTTP adapter in Phase 6)
- Test harness (nysm test) is the MVP invocation mechanism
- Structured logging with slog for debugging (--verbose flag)
- Clear user-facing messages separate from debug logs
- All database operations handled by store package (no direct SQLite in CLI)
- Context propagation enables graceful shutdown throughout engine
