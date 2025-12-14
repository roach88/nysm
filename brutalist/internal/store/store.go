package store

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed schema.sql
var schemaSQL string

// Schema version tracking:
// 0 - Initial schema (pre-migration)
// 1 - Added UNIQUE index on completions.invocation_id
const currentSchemaVersion = 1

// Store provides durable storage for NYSM event logs.
// Uses SQLite with WAL mode for concurrent read access.
type Store struct {
	db *sql.DB
}

// Open creates or opens a SQLite database at the given path.
// Applies required pragmas and migrations automatically.
//
// The database is configured with:
//   - WAL mode for concurrent reads during writes
//   - NORMAL synchronous mode (balance durability/performance)
//   - 5-second busy timeout for lock contention
//   - Foreign key enforcement
//
// This function is idempotent - safe to call multiple times.
func Open(path string) (*Store, error) {
	// Open database (creates file if doesn't exist)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Verify connection works
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool for SQLite
	// SQLite only supports one writer at a time, so limit connections
	db.SetMaxOpenConns(1) // Single writer to avoid SQLITE_BUSY errors
	db.SetMaxIdleConns(1) // Keep one connection ready

	// Apply required pragmas
	if err := applyPragmas(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply pragmas: %w", err)
	}

	// Apply schema migrations
	if err := applySchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to apply schema: %w", err)
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
// Should be called when the store is no longer needed.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// DB returns the underlying sql.DB for direct queries.
// Use with caution - prefer using Store methods when available.
func (s *Store) DB() *sql.DB {
	return s.db
}

// Query executes a query and returns the resulting rows.
// This is a convenience wrapper around db.QueryContext for use by the engine.
// Callers are responsible for closing the returned rows.
func (s *Store) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.db.QueryContext(ctx, query, args...)
}

// applyPragmas sets required SQLite configuration.
func applyPragmas(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return fmt.Errorf("failed to execute %q: %w", pragma, err)
		}
	}

	return nil
}

// applySchema creates tables if they don't exist and runs migrations.
// This function is idempotent.
func applySchema(db *sql.DB) error {
	if _, err := db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}

	// Run migrations
	if err := runMigrations(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// runMigrations applies incremental schema migrations based on user_version.
func runMigrations(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("get user_version: %w", err)
	}

	// Apply migrations sequentially
	if version < 1 {
		if err := migrateToV1(db); err != nil {
			return err
		}
		version = 1
	}

	// Set version after all migrations
	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", currentSchemaVersion)); err != nil {
		return fmt.Errorf("set user_version: %w", err)
	}

	return nil
}

// migrateToV1 adds UNIQUE index on completions.invocation_id for existing databases.
// New databases get this from the schema.sql UNIQUE constraint, but existing DBs
// created before v1 need this index added explicitly.
func migrateToV1(db *sql.DB) error {
	// CREATE UNIQUE INDEX IF NOT EXISTS is safe - no-op if index exists
	_, err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_completions_invocation_unique
		ON completions(invocation_id)
	`)
	if err != nil {
		return fmt.Errorf("migrate to v1: %w", err)
	}
	return nil
}

// verifyPragma checks that a pragma is set to the expected value.
// Used for testing.
func (s *Store) verifyPragma(name, expected string) error {
	var value string
	query := fmt.Sprintf("PRAGMA %s", name)
	if err := s.db.QueryRow(query).Scan(&value); err != nil {
		return fmt.Errorf("failed to query %s: %w", name, err)
	}
	if value != expected {
		return fmt.Errorf("%s = %q, expected %q", name, value, expected)
	}
	return nil
}
