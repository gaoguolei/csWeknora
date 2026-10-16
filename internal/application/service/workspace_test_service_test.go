package service

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func workspaceTestFixture(t *testing.T) (*WorkspaceTestService, *types.WorkspaceTestRun, *fakeFileService, context.Context) {
	t.Helper()
	db := newPinTestDB(t)
	require.NoError(t, db.AutoMigrate(&types.Message{}, &types.MessageArtifactRecord{}, &types.WorkspaceTestRun{}))
	require.NoError(t, db.Model(&types.Session{}).Create(map[string]any{"id": "test-session", "tenant_id": 7, "user_id": "owner"}).Error)
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner")
	files := &fakeFileService{}
	collector := NewArtifactCollector(nil, files, nil, nil, ArtifactCollectorConfig{})
	svc := NewWorkspaceTestService(repository.NewWorkspaceTestRunRepository(db), nil, nil, repository.NewSessionRepository(db), repository.NewMessageRepository(db), collector)
	run := &types.WorkspaceTestRun{ID: uuid.NewString(), TenantID: 7, UserID: "owner", SessionID: "test-session", Status: "completed", Request: types.WorkspaceTestRequest{Source: "manual"}, Result: types.WorkspaceTestResult{Status: "completed", Outcome: "failed", Log: "<script>alert(1)</script>", Checks: []types.WorkspaceTestCheck{{WorkspaceTestTarget: types.WorkspaceTestTarget{ID: "test", Kind: "tests", Name: "Unit tests"}, Status: "failed", Cases: []types.WorkspaceTestCaseResult{{Name: "real assertion", Status: "failed", Detail: "Expected 1; received 2"}}}}}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	require.NoError(t, svc.repo.Create(ctx, run))
	return svc, run, files, ctx
}

type localWorkspaceTestManager struct {
	stubPinnedManager
	root string
}

func (m *localWorkspaceTestManager) SessionWorkspaceLayout(context.Context, string) (sandbox.WorkspaceLayout, error) {
	return sandbox.WorkspaceLayout{Root: m.root, Origin: sandbox.WorkspaceOriginHost}, nil
}
func (m *localWorkspaceTestManager) ExecShellCommand(ctx context.Context, _, command, directory string, timeout time.Duration, _ map[string]string) (*sandbox.ExecuteResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = directory
	output, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		code = 1
	}
	return &sandbox.ExecuteResult{Stdout: string(output), ExitCode: code}, nil
}

func TestWorkspaceTestRealDetachedRunProducesDownloadableArtifacts(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node runtime unavailable")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("Python runtime unavailable")
	}
	svc, _, files, ctx := workspaceTestFixture(t)
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"demo","scripts":{"test":"node --test unit.test.mjs"}}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unit.test.mjs"), []byte(`import test from 'node:test';import assert from 'node:assert/strict';test('real assertion',()=>assert.equal(2+2,4));`), 0600))
	mgr := &localWorkspaceTestManager{root: root}
	svc.workspace = NewPinnedSessionSandbox(stubPinReader{configID: "cfg", tenantID: 7}, &stubTenantSandboxResolver{mgr: mgr}, nil, nil)
	run, err := svc.Start(ctx, "test-session", types.WorkspaceTestRequest{Timeout: 10}, SandboxPin{})
	require.NoError(t, err)
	require.True(t, run.Active())
	require.Eventually(t, func() bool {
		updated, err := svc.Refresh(ctx, "test-session", run.ID)
		if err != nil || updated.Active() || updated.ReportMessageID == "" {
			return false
		}
		run = updated
		return true
	}, 20*time.Second, 100*time.Millisecond)
	require.Equal(t, "passed", run.Result.Outcome, run.Result.Detail+run.Result.Log)
	require.Len(t, run.Result.Checks, 1)
	require.Len(t, run.Result.Checks[0].Cases, 1)
	require.Equal(t, "real assertion", run.Result.Checks[0].Cases[0].Name)
	message, err := svc.messages.GetMessage(ctx, run.SessionID, run.ReportMessageID)
	require.NoError(t, err)
	require.Len(t, message.Artifacts, 2)
	require.Len(t, files.saved, 2)
}

func TestWorkspaceTestOldFractionalDurationsCanCompleteAndCancel(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("Python runtime unavailable")
	}
	for _, status := range []string{"completed", "cancelled"} {
		t.Run(status, func(t *testing.T) {
			svc, run, files, ctx := workspaceTestFixture(t)
			root := t.TempDir()
			mgr := &localWorkspaceTestManager{root: root}
			svc.workspace = NewPinnedSessionSandbox(stubPinReader{configID: "cfg", tenantID: 7}, &stubTenantSandboxResolver{mgr: mgr}, nil, nil)
			run.Status = "running"
			run.RunnerStarted = true
			run.Result = types.WorkspaceTestResult{Status: "running", Outcome: "unknown", Checks: []types.WorkspaceTestCheck{}}
			require.NoError(t, svc.repo.Save(ctx, run))
			folder := filepath.Join(root, ".weknora", "test-runs", run.ID)
			require.NoError(t, os.MkdirAll(folder, 0700))
			// Reproduce checks.1.cases.0 from the old runner, which may have
			// finished in the sandbox while Go still shows it as running.
			snapshot := `{"status":"STATUS","outcome":"unknown","checks":[{"id":"backend","cases":[]},{"id":"frontend","status":"passed","cases":[{"name":"real assertion","status":"passed","duration_ms":328.5659089999999}]}]}`
			require.NoError(t, os.WriteFile(filepath.Join(folder, "state.json"), []byte(strings.Replace(snapshot, "STATUS", status, 1)), 0600))
			var got *types.WorkspaceTestRun
			var err error
			if status == "cancelled" {
				got, err = svc.Cancel(ctx, run.SessionID, run.ID)
				require.FileExists(t, filepath.Join(folder, "cancel"))
			} else {
				got, err = svc.Refresh(ctx, run.SessionID, run.ID)
			}
			require.NoError(t, err)
			require.Equal(t, status, got.Status)
			require.False(t, got.Active())
			require.Equal(t, int64(328), got.Result.Checks[1].Cases[0].DurationMs)
			require.NotEmpty(t, got.ReportMessageID)
			require.Len(t, files.saved, 2)
			stored, err := svc.repo.Get(ctx, run.TenantID, run.UserID, run.SessionID, run.ID)
			require.NoError(t, err)
			require.Equal(t, int64(328), stored.Result.Checks[1].Cases[0].DurationMs)
			for _, data := range files.saved {
				if len(data) > 0 && data[0] == '{' {
					var report types.WorkspaceTestRun
					require.NoError(t, json.Unmarshal(data, &report))
					require.Equal(t, int64(328), report.Result.Checks[1].Cases[0].DurationMs)
				}
			}
		})
	}
}

func TestWorkspaceTestReportPersistsToSessionArtifactsWithoutLiveSandbox(t *testing.T) {
	svc, run, files, ctx := workspaceTestFixture(t)
	got, err := svc.Refresh(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.NotEmpty(t, got.ReportMessageID)
	require.Equal(t, "failed", got.Result.Outcome)
	require.Empty(t, got.ReportError)
	message, err := svc.messages.GetMessage(ctx, run.SessionID, got.ReportMessageID)
	require.NoError(t, err)
	require.True(t, message.IsCompleted)
	require.Len(t, message.Artifacts, 2)
	require.Equal(t, ".html", message.Artifacts[0].FileType)
	require.Equal(t, ".json", message.Artifacts[1].FileType)
	arts, err := svc.messages.GetSessionArtifacts(ctx, run.SessionID)
	require.NoError(t, err)
	require.Len(t, arts, 2, "existing artifact drawer and library must see test reports")
	for _, data := range files.saved {
		if strings.Contains(string(data), "<!doctype html>") {
			require.Contains(t, string(data), "Expected 1; received 2")
			require.NotContains(t, string(data), "<script>alert(1)</script>")
			require.Contains(t, string(data), "&lt;script&gt;")
		}
	}
	_, err = svc.Refresh(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.Equal(t, 2, files.seq, "reloading a completed run must not duplicate report uploads")
}

func TestWorkspaceTestFailedReportStorageDoesNotFabricateSuccessAndCanRetry(t *testing.T) {
	svc, run, files, ctx := workspaceTestFixture(t)
	files.saveErr = errors.New("storage offline")
	got, err := svc.Refresh(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.Empty(t, got.ReportMessageID)
	require.NotEmpty(t, got.ReportError)
	require.Equal(t, "failed", got.Result.Outcome)
	files.saveErr = nil
	got, err = svc.Refresh(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.NotEmpty(t, got.ReportMessageID)
	require.Empty(t, got.ReportError)
}

func TestWorkspaceTestReportIsOwnerAndSessionScoped(t *testing.T) {
	svc, run, files, ctx := workspaceTestFixture(t)
	other := context.WithValue(ctx, types.UserIDContextKey, "another-owner")
	_, err := svc.Refresh(other, run.SessionID, run.ID)
	require.Error(t, err)
	_, err = svc.Refresh(ctx, "another-session", run.ID)
	require.Error(t, err)
	require.Zero(t, files.seq)
}

func TestWorkspaceTestCancelBeforeLaunchNeverCreatesASandbox(t *testing.T) {
	svc, run, _, ctx := workspaceTestFixture(t)
	run.Status = "queued"
	run.Result.Status = "queued"
	require.NoError(t, svc.repo.Save(ctx, run))
	cancelled, err := svc.Cancel(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelling", cancelled.Status)
	// No workspace/starter is configured: touching either would panic.
	require.True(t, svc.cancelBeforeLaunch(ctx, run))
	got, err := svc.Refresh(ctx, run.SessionID, run.ID)
	require.NoError(t, err)
	require.Equal(t, "cancelled", got.Status)
	require.NotEmpty(t, got.ReportMessageID)
}

func TestWorkspaceTestBrowserFailureIsSeparateClientEvidenceAndHasArtifacts(t *testing.T) {
	svc, parent, _, ctx := workspaceTestFixture(t)
	probe, err := svc.RecordBrowserCheck(ctx, parent.SessionID, parent.ID, "blocked", "Browser could not resolve preview DNS")
	require.NoError(t, err)
	require.Equal(t, "browser", probe.Result.Checks[0].Kind)
	require.Equal(t, "blocked", probe.Result.Outcome)
	require.NotEmpty(t, probe.ReportMessageID)
	original, err := svc.repo.Get(ctx, parent.TenantID, parent.UserID, parent.SessionID, parent.ID)
	require.NoError(t, err)
	require.Equal(t, "failed", original.Result.Outcome, "client evidence must not overwrite native assertions")
	arts, err := svc.messages.GetSessionArtifacts(ctx, parent.SessionID)
	require.NoError(t, err)
	require.Len(t, arts, 2)
}
