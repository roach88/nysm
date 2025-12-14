package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestOpen_CreatesNewDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Verify file was created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("database file was not created")
	}
}

func TestOpen_OpensExistingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// Create database
	s1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open() failed: %v", err)
	}
	s1.Close()

	// Reopen database
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open() failed: %v", err)
	}
	defer s2.Close()

	// Verify we can query it
	var count int
	err = s2.db.QueryRow("SELECT COUNT(*) FROM invocations").Scan(&count)
	if err != nil {
		t.Errorf("query failed: %v", err)
	}
}

func TestOpen_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// Open multiple times
	for i := 0; i < 3; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open() iteration %d failed: %v", i, err)
		}
		s.Close()
	}

	// Final open should work
	s, err := Open(path)
	if err != nil {
		t.Fatalf("final Open() failed: %v", err)
	}
	defer s.Close()

	// Verify schema is intact
	tables := []string{"invocations", "completions", "sync_firings", "provenance_edges"}
	for _, table := range tables {
		var name string
		err := s.db.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %q not found after idempotent opens: %v", table, err)
		}
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	// Try to open in non-existent directory
	path := "/nonexistent/dir/test.db"

	_, err := Open(path)
	if err == nil {
		t.Error("expected error for invalid path, got nil")
	}
}

func TestClose_NilDB(t *testing.T) {
	s := &Store{db: nil}
	err := s.Close()
	if err != nil {
		t.Errorf("Close() on nil db should not error: %v", err)
	}
}

func TestClose_MultipleCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}

	// First close should succeed
	if err := s.Close(); err != nil {
		t.Errorf("first Close() failed: %v", err)
	}

	// Second close should not panic (though may error)
	// We just verify it doesn't panic
	_ = s.Close()
}

func TestDB_ReturnsUnderlyingConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	db := s.DB()
	if db == nil {
		t.Error("DB() returned nil")
	}

	// Verify it's usable
	if err := db.Ping(); err != nil {
		t.Errorf("DB() connection not usable: %v", err)
	}
}

// Pragma tests

func TestPragma_JournalMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	if err := s.verifyPragma("journal_mode", "wal"); err != nil {
		t.Error(err)
	}
}

func TestPragma_Synchronous(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// NORMAL = 1
	if err := s.verifyPragma("synchronous", "1"); err != nil {
		t.Error(err)
	}
}

func TestPragma_BusyTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	if err := s.verifyPragma("busy_timeout", "5000"); err != nil {
		t.Error(err)
	}
}

func TestPragma_ForeignKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// ON = 1
	if err := s.verifyPragma("foreign_keys", "1"); err != nil {
		t.Error(err)
	}
}

// Schema table tests

func TestSchema_InvocationsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Verify table exists with expected columns
	columns := getTableColumns(t, s.db, "invocations")

	expected := []string{
		"id", "flow_token", "action_uri", "args", "seq",
		"security_context", "spec_hash", "engine_version", "ir_version",
	}

	for _, col := range expected {
		if !contains(columns, col) {
			t.Errorf("invocations table missing column %q", col)
		}
	}
}

func TestSchema_CompletionsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	columns := getTableColumns(t, s.db, "completions")

	expected := []string{
		"id", "invocation_id", "output_case", "result", "seq", "security_context",
	}

	for _, col := range expected {
		if !contains(columns, col) {
			t.Errorf("completions table missing column %q", col)
		}
	}
}

func TestSchema_SyncFiringsTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	columns := getTableColumns(t, s.db, "sync_firings")

	expected := []string{
		"id", "completion_id", "sync_id", "binding_hash", "seq",
	}

	for _, col := range expected {
		if !contains(columns, col) {
			t.Errorf("sync_firings table missing column %q", col)
		}
	}
}

func TestSchema_ProvenanceEdgesTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	columns := getTableColumns(t, s.db, "provenance_edges")

	expected := []string{
		"id", "sync_firing_id", "invocation_id",
	}

	for _, col := range expected {
		if !contains(columns, col) {
			t.Errorf("provenance_edges table missing column %q", col)
		}
	}
}

// Index tests

func TestSchema_InvocationsIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	indexes := getTableIndexes(t, s.db, "invocations")

	expected := []string{
		"idx_invocations_flow_token",
		"idx_invocations_seq",
	}

	for _, idx := range expected {
		if !contains(indexes, idx) {
			t.Errorf("invocations table missing index %q", idx)
		}
	}
}

func TestSchema_CompletionsIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	indexes := getTableIndexes(t, s.db, "completions")

	expected := []string{
		"idx_completions_invocation",
		"idx_completions_seq",
	}

	for _, idx := range expected {
		if !contains(indexes, idx) {
			t.Errorf("completions table missing index %q", idx)
		}
	}
}

func TestSchema_SyncFiringsIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	indexes := getTableIndexes(t, s.db, "sync_firings")

	expected := []string{
		"idx_sync_firings_completion",
		"idx_sync_firings_seq",
	}

	for _, idx := range expected {
		if !contains(indexes, idx) {
			t.Errorf("sync_firings table missing index %q", idx)
		}
	}
}

func TestSchema_ProvenanceEdgesIndexes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	indexes := getTableIndexes(t, s.db, "provenance_edges")

	if !contains(indexes, "idx_provenance_invocation") {
		t.Error("provenance_edges table missing index idx_provenance_invocation")
	}
}

// Constraint tests

func TestConstraint_SyncFiringsUniqueBinding(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Insert an invocation first (for FK)
	_, err = s.db.Exec(`
		INSERT INTO invocations (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES ('inv1', 'flow1', 'Test.action', '{}', 1, '{}', 'hash1', '1.0', '1.0')
	`)
	if err != nil {
		t.Fatalf("failed to insert invocation: %v", err)
	}

	// Insert a completion (for FK)
	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp1', 'inv1', 'Success', '{}', 2, '{}')
	`)
	if err != nil {
		t.Fatalf("failed to insert completion: %v", err)
	}

	// Insert first sync firing
	_, err = s.db.Exec(`
		INSERT INTO sync_firings (completion_id, sync_id, binding_hash, seq)
		VALUES ('comp1', 'sync1', 'binding1', 3)
	`)
	if err != nil {
		t.Fatalf("failed to insert first sync firing: %v", err)
	}

	// Try to insert duplicate (same completion_id, sync_id, binding_hash)
	_, err = s.db.Exec(`
		INSERT INTO sync_firings (completion_id, sync_id, binding_hash, seq)
		VALUES ('comp1', 'sync1', 'binding1', 4)
	`)
	if err == nil {
		t.Error("expected UNIQUE constraint violation, got nil")
	}
}

func TestConstraint_SyncFiringsAllowsDifferentBindings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Set up FK chain
	_, err = s.db.Exec(`
		INSERT INTO invocations (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES ('inv1', 'flow1', 'Test.action', '{}', 1, '{}', 'hash1', '1.0', '1.0')
	`)
	if err != nil {
		t.Fatalf("failed to insert invocation: %v", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp1', 'inv1', 'Success', '{}', 2, '{}')
	`)
	if err != nil {
		t.Fatalf("failed to insert completion: %v", err)
	}

	// Insert sync firings with different binding hashes - should succeed
	for i, hash := range []string{"binding1", "binding2", "binding3"} {
		_, err = s.db.Exec(`
			INSERT INTO sync_firings (completion_id, sync_id, binding_hash, seq)
			VALUES ('comp1', 'sync1', ?, ?)
		`, hash, i+3)
		if err != nil {
			t.Errorf("failed to insert sync firing with hash %q: %v", hash, err)
		}
	}
}

func TestConstraint_ProvenanceEdgeUniqueSyncFiring(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Set up FK chain
	_, err = s.db.Exec(`
		INSERT INTO invocations (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES
			('inv1', 'flow1', 'Test.action', '{}', 1, '{}', 'hash1', '1.0', '1.0'),
			('inv2', 'flow1', 'Test.action2', '{}', 4, '{}', 'hash1', '1.0', '1.0')
	`)
	if err != nil {
		t.Fatalf("failed to insert invocations: %v", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp1', 'inv1', 'Success', '{}', 2, '{}')
	`)
	if err != nil {
		t.Fatalf("failed to insert completion: %v", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO sync_firings (completion_id, sync_id, binding_hash, seq)
		VALUES ('comp1', 'sync1', 'binding1', 3)
	`)
	if err != nil {
		t.Fatalf("failed to insert sync firing: %v", err)
	}

	// Get the sync_firing_id
	var syncFiringID int64
	err = s.db.QueryRow("SELECT id FROM sync_firings WHERE completion_id = 'comp1'").Scan(&syncFiringID)
	if err != nil {
		t.Fatalf("failed to get sync firing ID: %v", err)
	}

	// Insert first provenance edge
	_, err = s.db.Exec(`
		INSERT INTO provenance_edges (sync_firing_id, invocation_id)
		VALUES (?, 'inv2')
	`, syncFiringID)
	if err != nil {
		t.Fatalf("failed to insert first provenance edge: %v", err)
	}

	// Try to insert another edge for same sync_firing_id
	_, err = s.db.Exec(`
		INSERT INTO provenance_edges (sync_firing_id, invocation_id)
		VALUES (?, 'inv2')
	`, syncFiringID)
	if err == nil {
		t.Error("expected UNIQUE constraint violation on sync_firing_id, got nil")
	}
}

func TestConstraint_ForeignKeyCompletionToInvocation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Try to insert completion with non-existent invocation_id
	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp1', 'nonexistent', 'Success', '{}', 2, '{}')
	`)
	if err == nil {
		t.Error("expected foreign key constraint violation, got nil")
	}
}

func TestConstraint_ForeignKeySyncFiringToCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Try to insert sync_firing with non-existent completion_id
	_, err = s.db.Exec(`
		INSERT INTO sync_firings (completion_id, sync_id, binding_hash, seq)
		VALUES ('nonexistent', 'sync1', 'binding1', 3)
	`)
	if err == nil {
		t.Error("expected foreign key constraint violation, got nil")
	}
}

func TestConstraint_CompletionUniqueInvocationID(t *testing.T) {
	// Each invocation can have exactly ONE completion (UNIQUE constraint)
	// Required for deterministic replay - multiple completions would break replay.
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Insert invocation
	_, err = s.db.Exec(`
		INSERT INTO invocations (id, flow_token, action_uri, args, seq, security_context, spec_hash, engine_version, ir_version)
		VALUES ('inv1', 'flow1', 'Test.action', '{}', 1, '{}', 'hash1', '1.0', '1.0')
	`)
	if err != nil {
		t.Fatalf("failed to insert invocation: %v", err)
	}

	// Insert first completion - should succeed
	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp1', 'inv1', 'Success', '{}', 2, '{}')
	`)
	if err != nil {
		t.Fatalf("failed to insert first completion: %v", err)
	}

	// Try to insert second completion for same invocation - should fail
	_, err = s.db.Exec(`
		INSERT INTO completions (id, invocation_id, output_case, result, seq, security_context)
		VALUES ('comp2', 'inv1', 'Error', '{}', 3, '{}')
	`)
	if err == nil {
		t.Error("expected UNIQUE constraint violation on invocation_id, got nil")
	}
}

// Migration tests

func TestMigration_SchemaVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Verify user_version is set to currentSchemaVersion
	var version int
	err = s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		t.Fatalf("failed to get user_version: %v", err)
	}

	if version != currentSchemaVersion {
		t.Errorf("user_version = %d, want %d", version, currentSchemaVersion)
	}
}

func TestMigration_V1UniqueIndexExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Check that the unique index on completions.invocation_id exists
	indexes := getTableIndexes(t, s.db, "completions")

	// Either the migration index or SQLite's auto-generated unique index should exist
	hasUniqueIndex := contains(indexes, "idx_completions_invocation_unique") ||
		contains(indexes, "sqlite_autoindex_completions_2") // SQLite creates this for UNIQUE columns
	if !hasUniqueIndex {
		t.Errorf("completions table missing unique index on invocation_id, indexes: %v", indexes)
	}
}

func TestMigration_IdempotentUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	// Open and close multiple times - migrations should be idempotent
	for i := 0; i < 3; i++ {
		s, err := Open(path)
		if err != nil {
			t.Fatalf("Open() iteration %d failed: %v", i, err)
		}

		// Verify version is correct each time
		var version int
		err = s.db.QueryRow("PRAGMA user_version").Scan(&version)
		if err != nil {
			t.Fatalf("failed to get user_version: %v", err)
		}

		if version != currentSchemaVersion {
			t.Errorf("iteration %d: user_version = %d, want %d", i, version, currentSchemaVersion)
		}

		s.Close()
	}
}

func TestMigration_UpgradeFromV0(t *testing.T) {
	// Simulate a pre-migration database (version 0)
	path := filepath.Join(t.TempDir(), "test.db")

	// Create database manually without migration
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	// Apply schema but NOT migrations (simulates pre-migration state)
	if _, err := db.Exec(schemaSQL); err != nil {
		t.Fatalf("failed to apply schema: %v", err)
	}

	// Set version to 0 explicitly (pre-migration)
	if _, err := db.Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatalf("failed to set user_version: %v", err)
	}
	db.Close()

	// Now open through our normal path - should trigger migration
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open() failed: %v", err)
	}
	defer s.Close()

	// Verify version was upgraded
	var version int
	err = s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if err != nil {
		t.Fatalf("failed to get user_version: %v", err)
	}

	if version != currentSchemaVersion {
		t.Errorf("user_version = %d, want %d after migration", version, currentSchemaVersion)
	}

	// Verify the unique index exists
	indexes := getTableIndexes(t, s.db, "completions")
	if !contains(indexes, "idx_completions_invocation_unique") {
		// The UNIQUE in schema.sql creates sqlite_autoindex_completions_2
		// but the migration also creates idx_completions_invocation_unique
		// At least one should exist
		hasUnique := false
		for _, idx := range indexes {
			if idx == "idx_completions_invocation_unique" || idx == "sqlite_autoindex_completions_2" {
				hasUnique = true
				break
			}
		}
		if !hasUnique {
			t.Errorf("expected unique index on completions.invocation_id after migration, got indexes: %v", indexes)
		}
	}
}

// Helper functions

func getTableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("failed to get table info for %q: %v", table, err)
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			t.Fatalf("failed to scan column info: %v", err)
		}
		columns = append(columns, name)
	}
	return columns
}

func getTableIndexes(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='index' AND tbl_name=?", table)
	if err != nil {
		t.Fatalf("failed to get indexes for %q: %v", table, err)
	}
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("failed to scan index name: %v", err)
		}
		indexes = append(indexes, name)
	}
	return indexes
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
