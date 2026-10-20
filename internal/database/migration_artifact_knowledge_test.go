package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArtifactKnowledgeLinkSQLiteMigrationPreservesExistingFiles(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE message_artifacts (id TEXT PRIMARY KEY, file_name TEXT NOT NULL);
		INSERT INTO message_artifacts VALUES ('artifact', 'cases.xlsx')`)
	require.NoError(t, err)
	root := sqliteRepoRoot(t)
	up, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000041_artifact_knowledge_link.up.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(up))
	require.NoError(t, err)
	var knowledge, kb, name string
	require.NoError(t, db.QueryRow("SELECT knowledge_id, knowledge_base_id, file_name FROM message_artifacts").Scan(&knowledge, &kb, &name))
	require.Empty(t, knowledge)
	require.Empty(t, kb)
	require.Equal(t, "cases.xlsx", name)
	_, err = db.Exec("UPDATE message_artifacts SET knowledge_id = 'knowledge', knowledge_base_id = 'kb'")
	require.NoError(t, err)
	down, err := os.ReadFile(filepath.Join(root, "migrations/sqlite/000041_artifact_knowledge_link.down.sql"))
	require.NoError(t, err)
	_, err = db.Exec(string(down))
	require.NoError(t, err)
	require.NoError(t, db.QueryRow("SELECT file_name FROM message_artifacts").Scan(&name))
	require.Equal(t, "cases.xlsx", name)
}
