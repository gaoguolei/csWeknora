package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// Run the handler's actual shell commands in isolated repositories, so quoting
// and Git's argument semantics are covered along with the response parsers.
type programmingGitSession struct{ interfaces.SessionService }

func (programmingGitSession) GetOwnedSession(context.Context, string) (*types.Session, error) {
	return &types.Session{ID: "git-test"}, nil
}

type programmingGitPin struct{}

func (programmingGitPin) Read(context.Context, string) (service.SandboxPin, error) {
	return service.SandboxPin{ConfigID: "git-test", TenantID: 1}, nil
}

type programmingGitFiles interface{ sandbox.SessionFileStore }

type programmingGitManager struct {
	sandbox.Manager
	programmingGitFiles
	root string
	env  []string
}

func (m *programmingGitManager) Resolve(context.Context, uint64, string) (sandbox.Manager, error) {
	return m, nil
}

func (m *programmingGitManager) SessionFileStore() sandbox.SessionFileStore { return m }

func (m *programmingGitManager) SessionShellExecutor() sandbox.SessionShellExecutor { return m }

func (m *programmingGitManager) SessionWorkspaceLayout(context.Context, string) (sandbox.WorkspaceLayout, error) {
	return sandbox.WorkspaceLayout{Root: m.root}, nil
}

func (m *programmingGitManager) ListSessionDirectory(context.Context, string, string) ([]sandbox.RemoteDirEntry, error) {
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return nil, err
	}
	var dirs []sandbox.RemoteDirEntry
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, sandbox.RemoteDirEntry{Path: filepath.Join(m.root, e.Name()), Type: sandbox.RemoteEntryDir})
		}
	}
	return dirs, nil
}

func (m *programmingGitManager) ExecShellCommand(ctx context.Context, _, command, dir string, _ time.Duration, _ map[string]string) (*sandbox.ExecuteResult, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir, cmd.Env = dir, m.env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, err
		}
	}
	return &sandbox.ExecuteResult{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: cmd.ProcessState.ExitCode()}, nil
}

func newProgrammingGitTest(t *testing.T) (*ProgrammingHandler, *programmingGitManager) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	m := &programmingGitManager{root: t.TempDir()}
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") && !strings.HasPrefix(e, "HOME=") && !strings.HasPrefix(e, "XDG_CONFIG_HOME=") {
			m.env = append(m.env, e)
		}
	}
	m.env = append(m.env, "HOME="+t.TempDir(), "XDG_CONFIG_HOME="+t.TempDir(), "GIT_CONFIG_NOSYSTEM=1", "GIT_TERMINAL_PROMPT=0")
	programmingGitRun(t, m, "init", "-q", "-b", "main")
	programmingGitRun(t, m, "config", "user.name", "Git Test")
	programmingGitRun(t, m, "config", "user.email", "git-test@example.com")
	h := NewProgrammingHandler(programmingGitSession{}, service.NewPinnedSessionSandbox(programmingGitPin{}, m, nil, nil))
	return h, m
}

func programmingGitRun(t *testing.T, m *programmingGitManager, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = m.root, m.env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func programmingGitWrite(t *testing.T, m *programmingGitManager, path, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(m.root, path), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func programmingGitCall(t *testing.T, handler gin.HandlerFunc, method, target, body string, data any) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	c.Params = gin.Params{{Key: "session_id", Value: "git-test"}}
	handler(c)
	if len(c.Errors) != 0 {
		t.Fatalf("handler errors: %v", c.Errors)
	}
	w := c.Writer
	if w.Status() != 200 {
		t.Fatalf("status = %d", w.Status())
	}
	var response struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(response.Data, data); err != nil {
		t.Fatal(err)
	}
}

func TestProgrammingGitBranchesShellAndUnbornHead(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	var data struct {
		Current  string   `json:"current"`
		Branches []string `json:"branches"`
	}
	programmingGitCall(t, h.GitBranches, "GET", "/git-branches", "", &data)
	if data.Current != "main" || len(data.Branches) != 0 {
		t.Fatalf("unborn branch: %+v", data)
	}
	var remote gitRemoteReport
	programmingGitCall(t, h.GitRemote, "GET", "/git-remote", "", &remote)
	if !remote.Available || remote.Branch != "main" {
		t.Fatalf("unborn remote: %+v", remote)
	}
	programmingGitRun(t, m, "commit", "-q", "--allow-empty", "-m", "initial")
	programmingGitRun(t, m, "branch", "feature/test")
	programmingGitCall(t, h.GitBranches, "GET", "/git-branches", "", &data)
	if data.Current != "main" || strings.Join(data.Branches, ",") != "feature/test,main" {
		t.Fatalf("branches: %+v", data)
	}
}

func TestProgrammingGitStatusUnusualPathsAndUnbornChanges(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "staged.txt", "staged\n")
	programmingGitRun(t, m, "add", "staged.txt")
	programmingGitWrite(t, m, "staged.txt", "working\nsecond\n")
	for _, path := range []string{"a\x1eb.txt", "--odd.txt", "文件 with space.txt"} {
		programmingGitWrite(t, m, path, "one\ntwo\nthree\n")
	}
	var data struct {
		Available bool `json:"available"`
		Repos     []struct {
			Files []gitStatusEntry `json:"files"`
		} `json:"repos"`
	}
	programmingGitCall(t, h.GitStatus, "GET", "/git-status", "", &data)
	if !data.Available || len(data.Repos) != 1 || len(data.Repos[0].Files) != 4 {
		t.Fatalf("status: %+v", data)
	}
	for _, f := range data.Repos[0].Files {
		want := 3
		if f.Path == "staged.txt" {
			want = 2
		}
		if f.Added != want || f.Removed != 0 {
			t.Errorf("counts for %q = +%d/-%d, want +%d/-0", f.Path, f.Added, f.Removed, want)
		}
	}
}

func TestProgrammingGitSwitchOnlyLocalBranches(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "tracked.txt", "one\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "initial")
	programmingGitRun(t, m, "tag", "v1")
	programmingGitRun(t, m, "branch", "feature")
	for _, target := range []string{"v1", "HEAD", "tracked.txt", "feature"} {
		var data struct {
			Switched bool `json:"switched"`
		}
		body, _ := json.Marshal(gitBranchSwitchRequest{Branch: target})
		programmingGitCall(t, h.GitSwitchBranch, "POST", "/git-switch-branch", string(body), &data)
		if data.Switched != (target == "feature") {
			t.Errorf("switch %q = %v", target, data.Switched)
		}
		current := strings.TrimSpace(programmingGitRun(t, m, "symbolic-ref", "--short", "HEAD"))
		want := "main"
		if target == "feature" {
			want = "feature"
		}
		if current != want {
			t.Fatalf("current = %q, want %q", current, want)
		}
	}
}

func TestProgrammingGitDiscardBeforeFirstCommit(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "staged.txt", "staged\n")
	programmingGitRun(t, m, "add", "staged.txt")
	programmingGitWrite(t, m, "staged.txt", "unstaged\n")
	programmingGitWrite(t, m, "untracked.txt", "new\n")
	programmingGitWrite(t, m, ".git/info/exclude", "ignored.txt\n")
	programmingGitWrite(t, m, "ignored.txt", "forced addition\n")
	programmingGitRun(t, m, "add", "-f", "ignored.txt")
	var data struct {
		Discarded bool   `json:"discarded"`
		Detail    string `json:"detail"`
	}
	programmingGitCall(t, h.GitDiscard, "POST", "/git-discard", "{}", &data)
	if !data.Discarded {
		t.Fatalf("discard: %+v", data)
	}
	if status := programmingGitRun(t, m, "status", "--porcelain"); status != "" {
		t.Fatalf("dirty after discard: %q", status)
	}
	for _, path := range []string{"staged.txt", "untracked.txt", "ignored.txt"} {
		if _, err := os.Stat(filepath.Join(m.root, path)); !os.IsNotExist(err) {
			t.Fatalf("%s was not removed: %v", path, err)
		}
	}
}

func TestProgrammingGitRepoNamesAreExact(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	child := filepath.Join(m.root, " app ")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	childManager := &programmingGitManager{root: child, env: m.env}
	programmingGitRun(t, childManager, "init", "-q", "-b", "child")
	var data struct {
		Current string `json:"current"`
	}
	programmingGitCall(t, h.GitBranches, "GET", "/git-branches?repo=%20app%20", "", &data)
	if data.Current != "child" {
		t.Fatalf("current = %q", data.Current)
	}
	if _, err := h.resolveGitRepoWorkdir(context.Background(), "git-test", sandbox.WorkspaceLayout{Root: m.root}, m, "/"); err == nil {
		t.Fatal("invalid repo must not silently target the workspace root")
	}
}

func TestProgrammingGitPullRejectsMalformedChunkedBody(t *testing.T) {
	h, _ := newProgrammingGitTest(t)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/git-pull", strings.NewReader("{"))
	c.Request.ContentLength = -1
	c.Params = gin.Params{{Key: "session_id", Value: "git-test"}}
	h.GitPull(c)
	if len(c.Errors) == 0 || !strings.Contains(c.Errors.String(), "invalid pull request") {
		t.Fatalf("malformed chunked body must be rejected before running git: %v", c.Errors)
	}
}

func TestProgrammingGitCommitPreservesLiteralMessage(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "app.txt", "app\n")
	message := "fix: it's literal $(touch injected)\n\n支持多行消息"
	body, _ := json.Marshal(gitCommitRequest{Message: message})
	var data struct {
		Committed bool   `json:"committed"`
		Detail    string `json:"detail"`
	}
	programmingGitCall(t, h.GitCommit, "POST", "/git-commit", string(body), &data)
	if !data.Committed {
		t.Fatalf("commit: %+v", data)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "log", "-1", "--format=%B")); got != message {
		t.Fatalf("message = %q, want %q", got, message)
	}
	if status := programmingGitRun(t, m, "status", "--porcelain"); status != "" {
		t.Fatalf("dirty after commit: %q", status)
	}
	if _, err := os.Stat(filepath.Join(m.root, "injected")); !os.IsNotExist(err) {
		t.Fatal("message executed as shell")
	}
}

func TestProgrammingGitRemoteAndFastForwardPull(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "initial\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "initial")
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")
	clone := filepath.Join(t.TempDir(), "remote-writer")
	programmingGitRun(t, m, "clone", "-q", "-b", "main", origin, clone)
	writer := &programmingGitManager{root: clone, env: m.env}
	programmingGitRun(t, writer, "config", "user.name", "Remote Test")
	programmingGitRun(t, writer, "config", "user.email", "remote@example.com")
	pushChange := func(content string) {
		programmingGitWrite(t, writer, "app.txt", content)
		programmingGitRun(t, writer, "add", "-A")
		programmingGitRun(t, writer, "commit", "-q", "-m", "remote update")
		programmingGitRun(t, writer, "push", "-q", "origin", "main")
	}
	pushChange("remote\n")
	var remote gitRemoteReport
	programmingGitCall(t, h.GitRemote, "GET", "/git-remote?refresh=1", "", &remote)
	if !remote.Fetched || remote.Branch != "main" || remote.Upstream != "origin/main" || remote.Ahead != 0 || remote.Behind != 1 || len(remote.Incoming) != 1 {
		t.Fatalf("remote: %+v", remote)
	}
	var pull struct {
		Pulled bool   `json:"pulled"`
		Detail string `json:"detail"`
	}
	programmingGitCall(t, h.GitPull, "POST", "/git-pull", "{}", &pull)
	if !pull.Pulled {
		t.Fatalf("pull: %+v", pull)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "app.txt")); err != nil || string(content) != "remote\n" {
		t.Fatalf("file: %q, %v", content, err)
	}
	programmingGitWrite(t, m, "local.txt", "local\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local update")
	head := programmingGitRun(t, m, "rev-parse", "HEAD")
	pushChange("diverged\n")
	programmingGitCall(t, h.GitRemote, "GET", "/git-remote?refresh=1", "", &remote)
	if remote.Ahead != 1 || remote.Behind != 1 {
		t.Fatalf("diverged remote: %+v", remote)
	}
	programmingGitCall(t, h.GitPull, "POST", "/git-pull", "{}", &pull)
	if pull.Pulled || pull.Detail == "" {
		t.Fatalf("diverged pull must be refused: %+v", pull)
	}
	if got := programmingGitRun(t, m, "rev-parse", "HEAD"); got != head {
		t.Fatal("failed pull moved HEAD")
	}
}
