package database

import (
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/stretchr/testify/require"
)

func TestForkMigrationSourcesHaveUniqueVersions(t *testing.T) {
	root := sqliteRepoRoot(t)
	for _, engine := range []string{"versioned", "sqlite"} {
		t.Run(engine, func(t *testing.T) {
			driver, err := source.Open("file://" + filepath.Join(root, "migrations", engine))
			require.NoError(t, err)
			require.NoError(t, driver.Close())
		})
	}
}

func TestSQLiteMigrationsUpgradeForkV33PreservesCodingData(t *testing.T) {
	root := sqliteRepoRoot(t)
	legacyRoot := copySQLiteMigrationsThrough(t, root, 33)
	chdirAndRestore(t, legacyRoot)
	dbPath := filepath.Join(t.TempDir(), "fork-upgrade.db")
	opts := MigrationOptions{SQLiteDBPath: dbPath}
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
	db := openSQLiteDB(t, dbPath)
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 33, version)
	require.False(t, dirty)
	_, err := db.Exec(`
		INSERT INTO projects (id, tenant_id, owner_id, name, root_path)
		VALUES ('kept-project', 1, 'owner', 'existing project', '/workspace/kept');
		INSERT INTO project_folders (id, project_id, tenant_id, name, local_path)
		VALUES ('kept-folder', 'kept-project', 1, 'existing folder', '/workspace/kept/src');
		INSERT INTO workspace_test_runs (id, tenant_id, user_id, session_id, status, result, runner_started, cancel_requested)
		VALUES ('kept-run', 1, 'owner', 'session', 'completed', '{"outcome":"passed"}', true, true);
	`)
	require.NoError(t, err)
	require.False(t, sqliteTableExists(t, db, "model_catalog_configs"))
	require.False(t, sqliteTableExists(t, db, "chunk_images"))

	chdirAndRestore(t, root)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
	// A second startup must be a no-op, not replay a historical custom migration.
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, 40, version)
	require.False(t, dirty)
	for _, table := range []string{"model_catalog_configs", "chunk_images", "mcp_metadata"} {
		require.True(t, sqliteTableExists(t, db, table), table)
	}
	require.True(t, sqliteColumnExists(t, db, "chunks", "source_locators"))
	require.True(t, sqliteIndexExists(t, db, "idx_messages_live_attachments"))
	var name, path, result string
	var started, cancelled bool
	require.NoError(t, db.QueryRow("SELECT name, root_path FROM projects WHERE id = 'kept-project'").Scan(&name, &path))
	require.Equal(t, "existing project", name)
	require.Equal(t, "/workspace/kept", path)
	require.NoError(t, db.QueryRow("SELECT local_path FROM project_folders WHERE id = 'kept-folder'").Scan(&path))
	require.Equal(t, "/workspace/kept/src", path)
	require.NoError(t, db.QueryRow("SELECT result, runner_started, cancel_requested FROM workspace_test_runs WHERE id = 'kept-run'").Scan(&result, &started, &cancelled))
	require.JSONEq(t, `{"outcome":"passed"}`, result)
	require.True(t, started)
	require.True(t, cancelled)
}
