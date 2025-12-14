package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/roach88/nysm/internal/engine"
	"github.com/roach88/nysm/internal/ir"
	"github.com/roach88/nysm/internal/store"
)

// RunOptions holds flags for the run command.
type RunOptions struct {
	*RootOptions
	Database string

	// FlowGenerator allows overriding the flow token generator (for testing).
	// If nil, defaults to UUIDv7Generator.
	FlowGenerator engine.FlowTokenGenerator
}

// NewRunCommand creates the run command.
func NewRunCommand(rootOpts *RootOptions) *cobra.Command {
	opts := &RunOptions{RootOptions: rootOpts}

	cmd := &cobra.Command{
		Use:   "run <specs-dir>",
		Short: "Start engine with compiled specs",
		Long: `Start the NYSM sync engine with compiled concept specs.

The engine loads concept specs and sync rules from the specified directory,
initializes a SQLite database (creating it if it doesn't exist), and starts
the single-writer event loop.

Example:
  nysm run --db ./nysm.db ./specs
  nysm run --db /tmp/test.db ./demo-specs --verbose`,
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEngine(opts, args[0], cmd)
		},
	}

	cmd.Flags().StringVar(&opts.Database, "db", "", "path to SQLite database (required)")
	_ = cmd.MarkFlagRequired("db")

	return cmd
}

func runEngine(opts *RunOptions, specsDir string, cmd *cobra.Command) error {
	// Configure logging based on verbose flag
	logLevel := slog.LevelInfo
	if opts.Verbose {
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
		return WrapExitError(ExitCommandError, "failed to compile specs", err)
	}
	slog.Info("specs compiled", "concepts", len(specs), "syncs", len(syncs))

	// Open database (create if not exists)
	slog.Info("opening database", "path", opts.Database)
	st, err := store.Open(opts.Database)
	if err != nil {
		return WrapExitError(ExitCommandError, "failed to open database", err)
	}
	defer func() {
		if closeErr := st.Close(); closeErr != nil {
			slog.Error("error closing database", "error", closeErr)
		}
	}()
	slog.Info("database ready")

	// Create engine with flow generator (default to UUIDv7)
	flowGen := opts.FlowGenerator
	if flowGen == nil {
		flowGen = engine.UUIDv7Generator{}
	}
	eng := engine.New(st, specs, syncs, flowGen)

	// Setup signal handling for graceful shutdown
	// Use command's context if available (for testing), otherwise create one
	parentCtx := cmd.Context()
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigChan) // Prevent signal handler leak

	go func() {
		select {
		case sig := <-sigChan:
			slog.Info("received signal, shutting down", "signal", sig)
			cancel()
		case <-ctx.Done():
			// Parent context cancelled (e.g., from test)
		}
	}()

	// Start engine
	slog.Info("engine starting", "db", opts.Database, "specs_dir", specsDir)
	fmt.Fprintln(cmd.OutOrStdout(), "Engine started. Listening for invocations...")
	fmt.Fprintln(cmd.OutOrStdout(), "Press Ctrl-C to stop.")

	if err := eng.Run(ctx); err != nil && err != context.Canceled && err != context.DeadlineExceeded {
		return WrapExitError(ExitFailure, "engine error", err)
	}

	slog.Info("engine stopped gracefully")
	return nil
}

// compileSpecs loads and compiles all CUE specs from a directory.
// Returns compiled concept specs and sync rules.
func compileSpecs(dir string) ([]ir.ConceptSpec, []ir.SyncRule, error) {
	// Use shared loader with fail-fast mode
	loadResult, loadErrors := LoadSpecs(dir, LoadModeFailFast)
	if loadResult == nil && len(loadErrors) > 0 {
		return nil, nil, loadErrors[0]
	}
	if len(loadErrors) > 0 {
		return nil, nil, loadErrors[0]
	}

	return loadResult.Concepts, loadResult.Syncs, nil
}
