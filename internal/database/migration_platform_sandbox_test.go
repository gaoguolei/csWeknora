package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// The platform default sandbox is one existing workspace config promoted in
// place, so the migration decides which workspace loses its row. These tests
// pin the candidate rule (most referenced live config, ties to the earliest
// created), the rows that must follow it, and the no-op guarantees that make
// the migration safe to re-run.

const platformSandboxMigrationDir = "migrations/sqlite"

func openPlatformSandboxFixture(t *testing.T) *sql.DB {
	t.Helper()
	db := openSQLiteDB(t, filepath.Join(t.TempDir(), "platform-sandbox.db"))
	_, err := db.Exec(`
		CREATE TABLE tenant_sandbox_configs (
			id TEXT PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			deleted_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		);
		CREATE TABLE custom_agents (
			id TEXT PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			config TEXT,
			deleted_at DATETIME
		);
		CREATE TABLE tenant_skills (
			id TEXT PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			sandbox_config_id TEXT,
			updated_at DATETIME
		);
		CREATE TABLE tenant_skill_snapshots (
			id TEXT PRIMARY KEY,
			tenant_id INTEGER NOT NULL,
			sandbox_config_id TEXT,
			updated_at DATETIME
		);`)
	require.NoError(t, err)
	return db
}

func runPlatformSandboxMigration(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	path := filepath.Join(
		sqliteRepoRoot(t), platformSandboxMigrationDir,
		"000043_platform_sandbox."+direction+".sql")
	script, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = db.Exec(string(script))
	require.NoError(t, err)
}

func runPlatformSkillInstallTenantMigration(t *testing.T, db *sql.DB, direction string) {
	t.Helper()
	path := filepath.Join(
		sqliteRepoRoot(t), platformSandboxMigrationDir,
		"000044_platform_skill_install_tenant."+direction+".sql")
	script, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = db.Exec(string(script))
	require.NoError(t, err)
}

func insertSandboxConfig(
	t *testing.T, db *sql.DB, id string, tenantID int64, name, description string,
	createdAt string, deleted bool,
) {
	t.Helper()
	var deletedAt any
	if deleted {
		deletedAt = "2026-02-01 00:00:00"
	}
	_, err := db.Exec(`
		INSERT INTO tenant_sandbox_configs
			(id, tenant_id, name, description, deleted_at, created_at, updated_at)
		VALUES (?, ?, ?, NULLIF(?, ''), ?, ?, ?)`,
		id, tenantID, name, description, deletedAt, createdAt, createdAt)
	require.NoError(t, err)
}

func insertAgentReference(
	t *testing.T, db *sql.DB, id string, tenantID int64, configID string,
) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO custom_agents (id, tenant_id, config) VALUES (?, ?, ?)`,
		id, tenantID, `{"sandbox_config_id":"`+configID+`"}`)
	require.NoError(t, err)
}

func insertSkillRow(t *testing.T, db *sql.DB, id string, tenantID int64, configID string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO tenant_skills (id, tenant_id, sandbox_config_id) VALUES (?, ?, ?)`,
		id, tenantID, configID)
	require.NoError(t, err)
	_, err = db.Exec(`
		INSERT INTO tenant_skill_snapshots (id, tenant_id, sandbox_config_id) VALUES (?, ?, ?)`,
		"snap-"+id, tenantID, configID)
	require.NoError(t, err)
}

func sandboxConfigRow(t *testing.T, db *sql.DB, id string) (tenantID int64, name string, description sql.NullString) {
	t.Helper()
	require.NoError(t, db.QueryRow(`
		SELECT tenant_id, name, description FROM tenant_sandbox_configs WHERE id = ?`,
		id).Scan(&tenantID, &name, &description))
	return tenantID, name, description
}

func skillOwnerTenant(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	var tenantID int64
	require.NoError(t, db.QueryRow(`SELECT tenant_id FROM tenant_skills WHERE id = ?`, id).Scan(&tenantID))
	return tenantID
}

func snapshotOwnerTenant(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	var tenantID int64
	require.NoError(t, db.QueryRow(`SELECT tenant_id FROM tenant_skill_snapshots WHERE id = ?`, id).Scan(&tenantID))
	return tenantID
}

// The most referenced live config becomes the platform default, its display
// name moves into description, and its install rows and snapshot ledger follow
// it to the platform tenant. The workspace policy row, soft-deleted configs and
// other workspaces keep everything they had.
func TestSQLitePlatformSandboxPromotesTheMostReferencedConfig(t *testing.T) {
	db := openPlatformSandboxFixture(t)

	insertSandboxConfig(t, db, "cfg-policy", 7, "__workspace_scripts_policy__", "", "2025-12-01 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-old", 7, "Old box", "", "2026-01-01 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-busy", 8, "Team B", "E2B in FRA", "2026-01-02 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-gone", 9, "Gone", "", "2025-11-01 00:00:00", true)
	// cfg-gone is deleted (and has the most references), cfg-old is older but
	// less used: neither may win over cfg-busy.
	insertAgentReference(t, db, "a1", 8, "cfg-busy")
	insertAgentReference(t, db, "a2", 8, "cfg-busy")
	insertAgentReference(t, db, "a3", 9, "cfg-busy")
	insertAgentReference(t, db, "a4", 7, "cfg-old")
	insertAgentReference(t, db, "a5", 9, "cfg-gone")
	insertAgentReference(t, db, "a6", 9, "cfg-gone")
	insertAgentReference(t, db, "a7", 9, "cfg-gone")

	insertSkillRow(t, db, "skill-b1", 8, "cfg-busy")
	insertSkillRow(t, db, "skill-b2", 8, "cfg-busy")
	insertSkillRow(t, db, "skill-old", 7, "cfg-old")
	// A row that already belongs to another workspace must not be re-homed.
	insertSkillRow(t, db, "skill-foreign", 5, "cfg-busy")

	runPlatformSandboxMigration(t, db, "up")

	tenantID, name, description := sandboxConfigRow(t, db, "cfg-busy")
	require.EqualValues(t, 10000, tenantID)
	require.Equal(t, "__platform_sandbox__", name)
	require.True(t, description.Valid)
	require.Equal(t, "Team B — E2B in FRA", description.String)

	tenantID, name, _ = sandboxConfigRow(t, db, "cfg-old")
	require.EqualValues(t, 7, tenantID)
	require.Equal(t, "Old box", name)

	tenantID, name, _ = sandboxConfigRow(t, db, "cfg-policy")
	require.EqualValues(t, 7, tenantID)
	require.Equal(t, "__workspace_scripts_policy__", name)

	tenantID, name, _ = sandboxConfigRow(t, db, "cfg-gone")
	require.EqualValues(t, 9, tenantID)
	require.Equal(t, "Gone", name)

	require.EqualValues(t, 10000, skillOwnerTenant(t, db, "skill-b1"))
	require.EqualValues(t, 10000, skillOwnerTenant(t, db, "skill-b2"))
	require.EqualValues(t, 7, skillOwnerTenant(t, db, "skill-old"))
	require.EqualValues(t, 5, skillOwnerTenant(t, db, "skill-foreign"))

	require.EqualValues(t, 10000, snapshotOwnerTenant(t, db, "snap-skill-b1"))
	require.EqualValues(t, 7, snapshotOwnerTenant(t, db, "snap-skill-old"))
}

// An existing platform row makes the migration a no-op, which is what protects
// a database that already promoted a config from promoting a second one.
func TestSQLitePlatformSandboxPromotionIsIdempotent(t *testing.T) {
	db := openPlatformSandboxFixture(t)

	insertSandboxConfig(t, db, "cfg-busy", 8, "Team B", "", "2026-01-02 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-old", 7, "Old box", "", "2026-01-01 00:00:00", false)
	insertAgentReference(t, db, "a1", 8, "cfg-busy")
	insertAgentReference(t, db, "a2", 8, "cfg-busy")
	insertAgentReference(t, db, "a3", 7, "cfg-old")
	insertSkillRow(t, db, "skill-b", 8, "cfg-busy")
	insertSkillRow(t, db, "skill-old", 7, "cfg-old")

	runPlatformSandboxMigration(t, db, "up")
	runPlatformSandboxMigration(t, db, "up")

	var platformRows int
	require.NoError(t, db.QueryRow(`
		SELECT COUNT(*) FROM tenant_sandbox_configs
		WHERE tenant_id = 10000 AND name = '__platform_sandbox__'`).Scan(&platformRows))
	require.Equal(t, 1, platformRows, "a second run must not promote another config")

	tenantID, name, _ := sandboxConfigRow(t, db, "cfg-old")
	require.EqualValues(t, 7, tenantID, "an already-promoted database keeps the remaining workspace rows")
	require.Equal(t, "Old box", name)
	require.EqualValues(t, 7, skillOwnerTenant(t, db, "skill-old"))
}

// Ties fall to the earliest created config, so a rerun on identical data is
// deterministic; a config that carried a description keeps it after the display
// name is prepended.
func TestSQLitePlatformSandboxTieGoesToTheEarliestCreated(t *testing.T) {
	db := openPlatformSandboxFixture(t)

	insertSandboxConfig(t, db, "cfg-late", 9, "Late", "", "2026-01-05 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-early", 8, "Early", "", "2026-01-03 00:00:00", false)
	insertAgentReference(t, db, "a1", 8, "cfg-late")
	insertAgentReference(t, db, "a2", 9, "cfg-early")

	runPlatformSandboxMigration(t, db, "up")

	tenantID, name, description := sandboxConfigRow(t, db, "cfg-early")
	require.EqualValues(t, 10000, tenantID)
	require.Equal(t, "__platform_sandbox__", name)
	require.Equal(t, "Early", description.String)

	tenantID, name, _ = sandboxConfigRow(t, db, "cfg-late")
	require.EqualValues(t, 9, tenantID)
	require.Equal(t, "Late", name)
}

// The rollback cannot recover the previous owner, so it only undoes the
// selection: the row regains the display name stored in description and stops
// being the platform default.
func TestSQLitePlatformSandboxRollbackRestoresTheDisplayName(t *testing.T) {
	db := openPlatformSandboxFixture(t)

	insertSandboxConfig(t, db, "cfg-busy", 8, "Team B", "E2B in FRA", "2026-01-02 00:00:00", false)
	insertAgentReference(t, db, "a1", 8, "cfg-busy")
	insertSkillRow(t, db, "skill-b", 8, "cfg-busy")

	runPlatformSandboxMigration(t, db, "up")
	runPlatformSandboxMigration(t, db, "down")

	tenantID, name, description := sandboxConfigRow(t, db, "cfg-busy")
	require.EqualValues(t, 10000, tenantID)
	require.Equal(t, "Team B", name)
	require.False(t, description.Valid)
	require.EqualValues(t, 10000, skillOwnerTenant(t, db, "skill-b"),
		"install rows stay with the config the rollback left behind")
}

// Installs recorded through the catalog path after 000043 landed under the
// caller's workspace instead of the platform tenant, so the platform-scoped
// reads, writes and uninstalls (which resolve a platform config to tenant 10000)
// could not see them: the manage drawer rendered empty and offered no uninstall.
// The backfill moves exactly those rows onto the platform tenant and leaves every
// other row (normal workspace installs, rows already on 10000) untouched.
func TestSQLitePlatformSandboxBackfillsLateSkillInstalls(t *testing.T) {
	db := openPlatformSandboxFixture(t)

	insertSandboxConfig(t, db, "cfg-platform", 10000, "__platform_sandbox__", "Shared", "2026-01-01 00:00:00", false)
	insertSandboxConfig(t, db, "cfg-ws", 7, "Workspace box", "", "2026-01-02 00:00:00", false)

	// A platform-config install that drifted to the caller's workspace.
	insertSkillRow(t, db, "skill-late", 8, "cfg-platform")
	// A platform-config install already on the platform tenant: untouched.
	insertSkillRow(t, db, "skill-ok", 10000, "cfg-platform")
	// A normal workspace install: untouched.
	insertSkillRow(t, db, "skill-ws", 7, "cfg-ws")

	runPlatformSkillInstallTenantMigration(t, db, "up")

	require.EqualValues(t, 10000, skillOwnerTenant(t, db, "skill-late"),
		"an install of the platform config belongs to the platform tenant")
	require.EqualValues(t, 10000, snapshotOwnerTenant(t, db, "snap-skill-late"))
	require.EqualValues(t, 10000, skillOwnerTenant(t, db, "skill-ok"),
		"a row already on the platform tenant is not moved")
	require.EqualValues(t, 7, skillOwnerTenant(t, db, "skill-ws"),
		"a normal workspace install keeps its workspace")
	require.EqualValues(t, 7, snapshotOwnerTenant(t, db, "snap-skill-ws"))
}
