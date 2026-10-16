package service

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
)

// The helper normalizes native case durations to integer milliseconds on
// execution and when reading older snapshots, matching WorkspaceTestResult.
//go:embed workspace_tests.py
var workspaceTestsScript string

var ErrWorkspaceTestBusy = errors.New("a validation run is already active in this session")

// Browser probes are explicitly labelled client evidence, separate from
// assertions executed by the sandbox's native test runners.
func (s *WorkspaceTestService) RecordBrowserCheck(ctx context.Context, session, parentID, outcome, detail string) (*types.WorkspaceTestRun, error) {
	tenant, user, err := s.scope(ctx, session)
	if err != nil {
		return nil, err
	}
	if outcome != "passed" && outcome != "blocked" && outcome != "unknown" {
		return nil, errors.New("invalid browser probe outcome")
	}
	parent, err := s.repo.Get(ctx, tenant, user, session, parentID)
	if err != nil || parent.Active() {
		return nil, errors.New("a completed validation run is required")
	}
	if len(detail) > 2000 {
		detail = detail[:2000]
	}
	now := time.Now().UTC()
	run := &types.WorkspaceTestRun{ID: uuid.NewString(), TenantID: tenant, UserID: user, SessionID: session, ProjectID: parent.ProjectID, Status: "completed", Request: types.WorkspaceTestRequest{ProjectID: parent.ProjectID, TargetID: "browser-preview", Source: "manual"}, Result: types.WorkspaceTestResult{Status: "completed", Outcome: outcome, Version: parent.Result.Version, ProjectVersions: parent.Result.ProjectVersions, VersionChanged: parent.Result.VersionChanged, Checks: []types.WorkspaceTestCheck{{WorkspaceTestTarget: types.WorkspaceTestTarget{ID: "browser-preview", Kind: "browser", Name: "Browser preview access"}, Status: outcome, Detail: detail, Cases: []types.WorkspaceTestCaseResult{}}}}, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, run); err != nil {
		return nil, err
	}
	s.publish(ctx, run)
	return run, nil
}

type WorkspaceTestService struct {
	repo      interfaces.WorkspaceTestRunRepository
	workspace *PinnedSessionSandbox
	starter   *SandboxTerminalService
	sessions  interfaces.SessionRepository
	messages  interfaces.MessageRepository
	collector *ArtifactCollector
	locks     [64]sync.Mutex
	watchers  sync.Map
}

func NewWorkspaceTestService(repo interfaces.WorkspaceTestRunRepository, workspace *PinnedSessionSandbox,
	starter *SandboxTerminalService, sessions interfaces.SessionRepository, messages interfaces.MessageRepository,
	collector *ArtifactCollector) *WorkspaceTestService {
	return &WorkspaceTestService{repo: repo, workspace: workspace, starter: starter, sessions: sessions, messages: messages, collector: collector}
}

func (s *WorkspaceTestService) scope(ctx context.Context, session string) (uint64, string, error) {
	tenant, _ := types.TenantIDFromContext(ctx)
	user, _ := types.UserIDFromContext(ctx)
	if tenant == 0 || user == "" || session == "" {
		return 0, "", errors.New("validation requires a session owner")
	}
	owned, err := s.sessions.Get(ctx, tenant, user, session)
	if err != nil || owned == nil || owned.UserID != user {
		return 0, "", errors.New("session not found")
	}
	return tenant, user, nil
}

func (s *WorkspaceTestService) driver(ctx context.Context, session, action, id string, request types.WorkspaceTestRequest) ([]byte, error) {
	layout, err := s.workspace.SessionWorkspaceLayout(ctx, session)
	if err != nil {
		return nil, err
	}
	options := map[string]any{"root": layout.Root, "action": action, "run_id": id, "project_id": request.ProjectID, "target_id": request.TargetID, "target_ids": request.TargetIDs, "files": request.Files, "case_name": request.CaseName, "case_file": request.CaseFile, "timeout": request.Timeout, "host": layout.IsHost()}
	if action == "start" {
		options["script"] = workspaceTestsScript
	}
	args, _ := json.Marshal(options)
	command := "python3 -c " + sandbox.ShellQuote(workspaceTestsScript) + " " + sandbox.ShellQuote(string(args))
	result, err := s.workspace.ExecWorkspaceCommand(ctx, session, command, layout.Root, 20*time.Second, nil)
	if err != nil {
		return nil, err
	}
	if result == nil || result.ExitCode != 0 || result.Killed {
		return nil, errors.New("validation helper could not execute; check Python 3 and sandbox connectivity")
	}
	return []byte(result.Stdout), nil
}

func (s *WorkspaceTestService) Discover(ctx context.Context, session string) (*types.WorkspaceTestDiscovery, error) {
	if _, _, err := s.scope(ctx, session); err != nil {
		return nil, err
	}
	d := &types.WorkspaceTestDiscovery{Status: "ready", Projects: []types.WorkspaceTestProject{}, Warnings: []string{}}
	if err := s.workspace.RequireRunningWorkspace(ctx, session); err != nil {
		d.Status = "sandbox_not_running"
		if errors.Is(err, sandbox.ErrSandboxPaused) {
			d.Status = "paused"
		}
		return d, nil
	}
	data, err := s.driver(ctx, session, "discover", "", types.WorkspaceTestRequest{})
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, d); err != nil {
		return nil, err
	}
	if d.Status == "error" {
		return nil, errors.New("project discovery failed")
	}
	return d, nil
}

func (s *WorkspaceTestService) Start(ctx context.Context, session string, req types.WorkspaceTestRequest, provision SandboxPin) (*types.WorkspaceTestRun, error) {
	tenant, user, err := s.scope(ctx, session)
	if err != nil {
		return nil, err
	}
	active, err := s.repo.Active(ctx, tenant, session)
	if err != nil {
		return nil, err
	}
	if active != nil {
		return nil, ErrWorkspaceTestBusy
	}
	if req.Timeout <= 0 {
		req.Timeout = 300
	}
	if req.Timeout > 900 {
		req.Timeout = 900
	}
	if req.Source != "ai" && req.Source != "automatic" {
		req.Source = "manual"
	}
	if len(req.Files) > 100 || len(req.TargetIDs) > 100 {
		return nil, errors.New("too many test files selected")
	}
	if len(req.CaseName) > 2000 || len(req.CaseFile) > 1024 {
		return nil, errors.New("test selector is too long")
	}
	for _, file := range req.Files {
		if len(file) > 1024 {
			return nil, errors.New("test path is too long")
		}
	}
	now := time.Now().UTC()
	run := &types.WorkspaceTestRun{ID: uuid.NewString(), TenantID: tenant, UserID: user, SessionID: session, ProjectID: req.ProjectID, Request: req, Status: "queued", Result: types.WorkspaceTestResult{Status: "queued", Outcome: "unknown", Checks: []types.WorkspaceTestCheck{}}, CreatedAt: now, UpdatedAt: now}
	if err := s.repo.Create(ctx, run); err != nil {
		if active, _ := s.repo.Active(ctx, tenant, session); active != nil {
			return nil, ErrWorkspaceTestBusy
		}
		return nil, err
	}
	copy := *run
	s.watch(ctx, &copy, provision, true)
	return run, nil
}

func (s *WorkspaceTestService) prepare(ctx context.Context, session string, provision SandboxPin) error {
	if layout, err := s.workspace.SessionWorkspaceLayout(ctx, session); err == nil && layout.IsHost() {
		return nil
	}
	return s.starter.EnsureSessionTestWorkspace(ctx, session, provision)
}

func (s *WorkspaceTestService) watch(parent context.Context, run *types.WorkspaceTestRun, provision SandboxPin, launch bool) {
	if _, loaded := s.watchers.LoadOrStore(run.ID, true); loaded {
		return
	}
	go func() {
		defer s.watchers.Delete(run.ID)
		ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), time.Duration(run.Request.Timeout+180)*time.Second)
		defer cancel()
		if !launch && s.workspace.RequireRunningWorkspace(ctx, run.SessionID) != nil {
			return
		}
		if launch {
			run.Status = "preparing"
			run.Result.Status = "preparing"
			run.UpdatedAt = time.Now().UTC()
			if err := s.repo.Save(ctx, run); err != nil {
				return
			}
			if s.cancelBeforeLaunch(ctx, run) {
				return
			}
			if err := s.prepare(ctx, run.SessionID, provision); err != nil {
				if s.cancelBeforeLaunch(ctx, run) {
					return
				}
				s.fail(ctx, run, "Could not prepare the session sandbox. Check its configuration and credentials.")
				return
			}
			if s.cancelBeforeLaunch(ctx, run) {
				return
			}
		}
		release, err := s.workspace.HoldTestExecution(ctx, run.SessionID)
		if err != nil {
			s.fail(ctx, run, "Could not hold the sandbox for validation.")
			return
		}
		defer release()
		if launch {
			run.SandboxID, _ = s.workspace.BoundSandboxID(ctx, run.SessionID)
			if err := s.repo.Save(ctx, run); err != nil {
				return
			}
			if _, err := s.driver(ctx, run.SessionID, "start", run.ID, run.Request); err != nil {
				s.fail(ctx, run, err.Error())
				return
			}
			run.RunnerStarted = true
			run.UpdatedAt = time.Now().UTC()
			if err := s.repo.Save(ctx, run); err != nil {
				return
			}
			if fresh, err := s.repo.Get(ctx, run.TenantID, run.UserID, run.SessionID, run.ID); err == nil && fresh.CancelRequested {
				_, _ = s.driver(ctx, run.SessionID, "cancel", run.ID, run.Request)
			}
		}
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			current, err := s.Refresh(ctx, run.SessionID, run.ID)
			if err == nil && !current.Active() {
				return
			}
			select {
			case <-ctx.Done():
				// The guest runner owns its deadline and process group. Do not
				// claim cancellation if the provider cannot confirm termination.
				return
			case <-ticker.C:
			}
		}
	}()
}

func (s *WorkspaceTestService) cancelBeforeLaunch(ctx context.Context, run *types.WorkspaceTestRun) bool {
	fresh, err := s.repo.Get(ctx, run.TenantID, run.UserID, run.SessionID, run.ID)
	if err != nil || !fresh.CancelRequested {
		return false
	}
	run.Status = "cancelled"
	run.Result.Status = "cancelled"
	run.Result.Outcome = "unknown"
	run.UpdatedAt = time.Now().UTC()
	_ = s.repo.Save(ctx, run)
	s.publish(ctx, run)
	return true
}

func (s *WorkspaceTestService) fail(ctx context.Context, run *types.WorkspaceTestRun, detail string) {
	run.Status = "error"
	run.Result.Status = "error"
	run.Result.Outcome = "blocked"
	run.Result.Detail = detail
	run.UpdatedAt = time.Now().UTC()
	_ = s.repo.Save(ctx, run)
	s.publish(ctx, run)
}

func (s *WorkspaceTestService) Refresh(ctx context.Context, session, id string) (*types.WorkspaceTestRun, error) {
	tenant, user, err := s.scope(ctx, session)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256([]byte(id))
	lock := &s.locks[int(hash[0])%len(s.locks)]
	lock.Lock()
	defer lock.Unlock()
	run, err := s.repo.Get(ctx, tenant, user, session, id)
	if err != nil {
		return nil, err
	}
	if run.Active() {
		if !run.RunnerStarted {
			if time.Since(run.UpdatedAt) > 2*time.Minute {
				s.fail(ctx, run, "Validation preparation was interrupted before the runner started.")
			}
			return run, nil
		}
		if err := s.workspace.RequireRunningWorkspace(ctx, session); err != nil {
			return run, nil
		}
		if bound, ok := s.workspace.BoundSandboxID(ctx, session); run.SandboxID != "" && (!ok || bound != run.SandboxID) {
			s.fail(ctx, run, "The session now has a different sandbox. This run could not be verified.")
			return run, nil
		}
		data, err := s.driver(ctx, session, "status", id, run.Request)
		if err != nil {
			return run, nil
		}
		var result types.WorkspaceTestResult
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		if result.Status == "" {
			return nil, errors.New("invalid validation state")
		}
		run.Result = result
		run.Status = result.Status
		run.UpdatedAt = time.Now().UTC()
		if err := s.repo.Save(ctx, run); err != nil {
			return nil, err
		}
	}
	if !run.Active() && run.ReportMessageID == "" {
		s.publish(ctx, run)
	}
	return run, nil
}

func (s *WorkspaceTestService) List(ctx context.Context, session string) ([]types.WorkspaceTestRun, error) {
	tenant, user, err := s.scope(ctx, session)
	if err != nil {
		return nil, err
	}
	runs, err := s.repo.List(ctx, tenant, user, session)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		if runs[i].Active() || (runs[i].ReportMessageID == "" && time.Since(runs[i].UpdatedAt) > 10*time.Second) {
			if updated, err := s.Refresh(ctx, session, runs[i].ID); err == nil {
				runs[i] = *updated
			}
			if runs[i].Active() && runs[i].RunnerStarted {
				s.watch(ctx, &runs[i], SandboxPin{}, false)
			}
		}
	}
	return runs, nil
}

func (s *WorkspaceTestService) Cancel(ctx context.Context, session, id string) (*types.WorkspaceTestRun, error) {
	tenant, user, err := s.scope(ctx, session)
	if err != nil {
		return nil, err
	}
	run, err := s.repo.Get(ctx, tenant, user, session, id)
	if err != nil || !run.Active() {
		return run, err
	}
	if err := s.repo.RequestCancel(ctx, run); err != nil {
		return nil, err
	}
	if !run.RunnerStarted {
		return s.repo.Get(ctx, tenant, user, session, id)
	}
	if err := s.prepare(ctx, session, SandboxPin{}); err != nil {
		return nil, err
	}
	if _, err := s.driver(ctx, session, "cancel", id, run.Request); err != nil {
		return nil, err
	}
	return s.Refresh(ctx, session, id)
}

func (s *WorkspaceTestService) publish(ctx context.Context, run *types.WorkspaceTestRun) {
	if s.collector == nil || s.messages == nil {
		run.ReportError = "Report storage is unavailable."
		_ = s.repo.Save(ctx, run)
		return
	}
	jsonReport, _ := json.MarshalIndent(run, "", "  ")
	// Reports expose only user-visible run coordinates, never provider credentials.
	htmlReport := renderWorkspaceTestReport(run)
	files := []struct {
		name string
		data []byte
	}{{"test-report-" + run.ID + ".html", htmlReport}, {"test-results-" + run.ID + ".json", jsonReport}}
	artifacts := types.MessageArtifacts{}
	for _, file := range files {
		entry := sandbox.RemoteDirEntry{Name: file.name, Path: "/workspace/output/test-reports/" + run.ID + "/" + file.name, ModTime: time.Now().UTC()}
		hash := sha256.Sum256(file.data)
		artifact, ok := s.collector.persistBytes(ctx, run.SessionID, "", run.TenantID, entry, file.data, hex.EncodeToString(hash[:]))
		if !ok {
			for _, saved := range artifacts {
				_ = s.collector.fileService.DeleteFile(ctx, saved.URL)
			}
			run.ReportError = "Could not save the validation report. Retry after storage is available."
			_ = s.repo.Save(ctx, run)
			return
		}
		artifacts = append(artifacts, artifact)
	}
	messageID, created, err := s.repo.PublishReport(ctx, run, artifacts)
	if err != nil {
		run.ReportError = "Could not attach the report to this session."
	} else {
		run.ReportMessageID = messageID
		run.ReportError = ""
	}
	for _, artifact := range artifacts {
		if err != nil || !created {
			_ = s.collector.fileService.DeleteFile(ctx, artifact.URL)
		} else {
			s.collector.bindArtifactResource(ctx, artifact.URL, messageID)
		}
	}
	run.UpdatedAt = time.Now().UTC()
	_ = s.repo.Save(ctx, run)
}

func renderWorkspaceTestReport(run *types.WorkspaceTestRun) []byte {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>项目验证报告</title><style>body{max-width:1000px;margin:32px auto;padding:0 20px;font:15px/1.6 system-ui;color:#24312b}table{width:100%;border-collapse:collapse}td,th{padding:10px;border-bottom:1px solid #dde5df;text-align:left}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:#f3f6f4;padding:16px;border-radius:8px}.passed{color:#008b58}.failed,.error{color:#b23737}small{color:#6a746d}h2{margin-top:32px}</style><h1>项目验证报告</h1>`)
	fmt.Fprintf(&b, "<p>结论：<strong>%s</strong> · 状态：%s · 耗时：%.1f 秒</p><small>会话：%s<br>运行：%s<br>执行时间：%s<br>提交：%s</small>", html.EscapeString(run.Result.Outcome), html.EscapeString(run.Status), float64(run.Result.DurationMs)/1000, html.EscapeString(run.SessionID), run.ID, run.CreatedAt.Format(time.RFC3339), html.EscapeString(run.Result.Version["head"]))
	if run.Result.VersionChanged {
		b.WriteString("<p>执行期间代码发生变化，本报告不能代表当前版本。</p>")
	}
	if run.Result.Detail != "" {
		fmt.Fprintf(&b, "<p>%s</p>", html.EscapeString(run.Result.Detail))
	}
	if len(run.Result.ProjectVersions) > 0 {
		b.WriteString("<h2>执行版本</h2><table><tr><th>项目</th><th>仓库</th><th>提交</th><th>工作区有变更</th></tr>")
		projects := make([]string, 0, len(run.Result.ProjectVersions))
		for project := range run.Result.ProjectVersions {
			projects = append(projects, project)
		}
		sort.Strings(projects)
		for _, project := range projects {
			version := run.Result.ProjectVersions[project]
			fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>", html.EscapeString(project), html.EscapeString(version["repo_path"]), html.EscapeString(version["head"]), html.EscapeString(version["dirty"]))
		}
		b.WriteString("</table>")
	}
	b.WriteString("<h2>验证结果</h2><table><tr><th>项目</th><th>目标</th><th>结果</th><th>耗时</th></tr>")
	for _, check := range run.Result.Checks {
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s</td><td class='%s'>%s</td><td>%.1f 秒</td></tr>", html.EscapeString(check.ProjectName), html.EscapeString(check.Name), html.EscapeString(check.Status), html.EscapeString(check.Status), float64(check.DurationMs)/1000)
	}
	b.WriteString("</table>")
	for _, check := range run.Result.Checks {
		fmt.Fprintf(&b, "<h3>%s / %s</h3><pre>%s</pre>", html.EscapeString(check.ProjectName), html.EscapeString(check.Name), html.EscapeString(check.Command))
		if check.Detail != "" {
			fmt.Fprintf(&b, "<pre>%s</pre>", html.EscapeString(check.Detail))
		}
		for _, test := range check.Cases {
			fmt.Fprintf(&b, "<p class='%s'>%s · %s</p>", html.EscapeString(test.Status), html.EscapeString(test.Name), html.EscapeString(test.Status))
			if test.Detail != "" {
				fmt.Fprintf(&b, "<pre>%s</pre>", html.EscapeString(test.Detail))
			}
		}
	}
	b.WriteString("<p>服务联通检查只验证沙箱内部地址；用户浏览器的 DNS、代理及预览入口需要另行验证。</p><h2>执行日志（最多保留最后 100,000 字符）</h2><pre>")
	b.WriteString(html.EscapeString(run.Result.Log))
	b.WriteString("</pre></html>")
	return []byte(b.String())
}
