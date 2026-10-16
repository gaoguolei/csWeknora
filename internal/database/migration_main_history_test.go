package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSQLiteMigrationsPreserveBothMainAndDevHistories(t *testing.T) {
	for _, lineage := range []string{"main-v33", "dev-v39"} {
		t.Run(lineage, func(t *testing.T) {
			root := sqliteRepoRoot(t)
			version := 39
			if lineage == "main-v33" {
				version = 32
			}
			legacy := copySQLiteMigrationsThrough(t, root, version)
			if lineage == "main-v33" {
				// Actual pre-merge main schema: version 33 added the role,
				// rather than the workspace-test table used by dev.
				require.NoError(t, os.WriteFile(filepath.Join(legacy, "migrations/sqlite/000033_operations_admin.up.sql"), []byte(`
ALTER TABLE users ADD COLUMN is_operations_admin BOOLEAN NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_users_is_operations_admin ON users(is_operations_admin);
`), 0o600))
			}
			chdirAndRestore(t, legacy)
			path := filepath.Join(t.TempDir(), "upgrade.db")
			opts := MigrationOptions{SQLiteDBPath: path}
			require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
			db := openSQLiteDB(t, path)
			_, err := db.Exec(`INSERT INTO users(id, username, email, password_hash)
VALUES ('kept-user', 'kept-user', 'kept@example.test', 'unused');
INSERT INTO projects(id, tenant_id, name) VALUES ('kept-project', 1, 'existing project');`)
			require.NoError(t, err)
			if lineage == "main-v33" {
				_, err = db.Exec("UPDATE users SET is_operations_admin = 1 WHERE id = 'kept-user'")
				require.NoError(t, err)
				require.False(t, sqliteTableExists(t, db, "workspace_test_runs"))
			} else {
				_, err = db.Exec(`INSERT INTO workspace_test_runs(id, tenant_id, user_id, session_id, status, result)
VALUES ('kept-report', 1, 'kept-user', 'session', 'completed', '{"passed":true}')`)
				require.NoError(t, err)
			}
			chdirAndRestore(t, root)
			require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
			require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", opts))
			current, dirty := sqliteMigrationState(t, db)
			require.Equal(t, 40, current)
			require.False(t, dirty)
			var role bool
			require.NoError(t, db.QueryRow("SELECT is_operations_admin FROM users WHERE id = 'kept-user'").Scan(&role))
			require.Equal(t, lineage == "main-v33", role)
			var name string
			require.NoError(t, db.QueryRow("SELECT name FROM projects WHERE id = 'kept-project'").Scan(&name))
			require.Equal(t, "existing project", name)
			require.True(t, sqliteTableExists(t, db, "workspace_test_runs"))
			require.True(t, sqliteColumnExists(t, db, "workspace_test_runs", "runner_started"))
			if lineage == "dev-v39" {
				var report string
				require.NoError(t, db.QueryRow("SELECT result FROM workspace_test_runs WHERE id = 'kept-report'").Scan(&report))
				require.JSONEq(t, `{"passed":true}`, report)
			}
		})
	}
}
