package repository

import (
	"context"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
	"time"
)

func TestWorkspaceTestRunPersistenceAndActiveSessionConstraint(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/runs.db"), &gorm.Config{})
	require.NoError(t, err)
	sql, _ := db.DB()
	t.Cleanup(func() { _ = sql.Close() })
	require.NoError(t, db.AutoMigrate(&types.WorkspaceTestRun{}))
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX active_run ON workspace_test_runs(tenant_id,session_id) WHERE status IN ('queued','preparing','running','cancelling')`).Error)
	repo := NewWorkspaceTestRunRepository(db)
	ctx := context.Background()
	run := &types.WorkspaceTestRun{ID: "first", TenantID: 1, UserID: "user", SessionID: "session", Status: "running", Request: types.WorkspaceTestRequest{TargetIDs: []string{"file-a"}}, Result: types.WorkspaceTestResult{Status: "running", Outcome: "unknown"}, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	require.NoError(t, repo.Create(ctx, run))
	other := *run
	other.ID = "second"
	require.Error(t, repo.Create(ctx, &other), "two concurrent validations must not share one session")
	run.Status = "completed"
	run.Result = types.WorkspaceTestResult{Status: "completed", Outcome: "no_tests", Log: "actual output"}
	require.NoError(t, repo.Save(ctx, run))
	loaded, err := repo.Get(ctx, 1, "user", "session", "first")
	require.NoError(t, err)
	require.Equal(t, "no_tests", loaded.Result.Outcome)
	require.Equal(t, "actual output", loaded.Result.Log)
	require.Equal(t, []string{"file-a"}, loaded.Request.TargetIDs)
	require.NoError(t, repo.Create(ctx, &other))
	require.NoError(t, repo.RequestCancel(ctx, &other))
	other.Status = "preparing"
	require.NoError(t, repo.Save(ctx, &other))
	cancelling, err := repo.Get(ctx, 1, "user", "session", other.ID)
	require.NoError(t, err)
	require.True(t, cancelling.CancelRequested)
	require.Equal(t, "cancelling", cancelling.Status, "a late startup update must not erase cancellation")
	_, err = repo.Get(ctx, 1, "someone-else", "session", "first")
	require.Error(t, err)
}
