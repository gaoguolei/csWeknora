package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Temporary tables keep this regression isolated from application records.
// The migration must repair an already-applied 114, and also be safe when
// 114 already created the final schema with both control fields.
func TestPostgresWorkspaceTestRunControlFlagsUpgrade(t *testing.T) {
	dsn := os.Getenv("WEKNORA_MIGRATION_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set WEKNORA_MIGRATION_TEST_POSTGRES_DSN to run PostgreSQL migration tests")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, `CREATE TEMP TABLE workspace_test_runs (
		id text PRIMARY KEY, status text NOT NULL, result text NOT NULL
	) ON COMMIT DROP;
	INSERT INTO workspace_test_runs VALUES ('existing-report', 'completed', '{"outcome":"passed"}');`)
	require.NoError(t, err)
	root := sqliteRepoRoot(t)
	up, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000115_workspace_test_run_control_flags.up.sql"))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(up))
	require.NoError(t, err)
	var started, cancelled bool
	var result string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT runner_started, cancel_requested, result FROM workspace_test_runs WHERE id = 'existing-report'`).Scan(&started, &cancelled, &result))
	require.False(t, started)
	require.False(t, cancelled)
	require.JSONEq(t, `{"outcome":"passed"}`, result)
	_, err = tx.ExecContext(ctx, `UPDATE workspace_test_runs SET runner_started = true, cancel_requested = true`)
	require.NoError(t, err)
	// An up-to-date 114 must preserve its existing flags.
	_, err = tx.ExecContext(ctx, string(up))
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(root, "migrations/versioned/000115_workspace_test_run_control_flags.down.sql"))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(down))
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT runner_started, cancel_requested FROM workspace_test_runs WHERE id = 'existing-report'`).Scan(&started, &cancelled))
	require.True(t, started)
	require.True(t, cancelled)
}
