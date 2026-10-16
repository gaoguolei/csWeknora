package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A diverged branch cannot fast-forward. Realign unwinds the local commits
// first so the same pull succeeds, keeping their content in the working tree —
// the sandbox has no terminal, so a refused pull would strand the user.
func TestProgrammingGitPullRealignKeepsLocalChanges(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "base\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "base")
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")

	// Someone else pushes a commit touching a different file.
	clone := filepath.Join(t.TempDir(), "other")
	programmingGitRun(t, m, "clone", "-q", "-b", "main", origin, clone)
	other := &programmingGitManager{root: clone, env: m.env}
	programmingGitRun(t, other, "config", "user.name", "Other")
	programmingGitRun(t, other, "config", "user.email", "other@example.com")
	programmingGitWrite(t, other, "remote.txt", "remote\n")
	programmingGitRun(t, other, "add", "-A")
	programmingGitRun(t, other, "commit", "-q", "-m", "remote update")
	programmingGitRun(t, other, "push", "-q", "origin", "main")
	remoteHead := strings.TrimSpace(programmingGitRun(t, other, "rev-parse", "HEAD"))

	// The workspace has an unpushed commit of its own: the branch has diverged.
	programmingGitWrite(t, m, "local.txt", "local\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local update")

	// A plain pull must still refuse, as before.
	var refused struct {
		Pulled bool   `json:"pulled"`
		Detail string `json:"detail"`
	}
	programmingGitCall(t, h.GitPull, "POST", "/git-pull", "{}", &refused)
	if refused.Pulled || refused.Detail == "" {
		t.Fatalf("diverged pull must be refused: %+v", refused)
	}

	body, _ := json.Marshal(map[string]bool{"realign": true})
	var aligned struct {
		Pulled  bool `json:"pulled"`
		Aligned bool `json:"aligned"`
	}
	programmingGitCall(t, h.GitPull, "POST", "/git-pull", string(body), &aligned)
	if !aligned.Pulled || !aligned.Aligned {
		t.Fatalf("realign pull failed: %+v", aligned)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != remoteHead {
		t.Fatalf("HEAD = %s, want the remote head %s", got, remoteHead)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "local.txt")); err != nil || string(content) != "local\n" {
		t.Fatalf("the unwound change was lost: %q, %v", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "remote.txt")); err != nil || string(content) != "remote\n" {
		t.Fatalf("the pulled file is missing: %q, %v", content, err)
	}
	if status := programmingGitRun(t, m, "status", "--porcelain"); !strings.Contains(status, "local.txt") {
		t.Fatalf("local.txt should be an uncommitted change again: %q", status)
	}
}

// When the remote touched the same file the fast-forward cannot apply at all.
// The branch must be put back exactly as it was rather than left rewound.
func TestProgrammingGitPullRealignRollsBackOnOverlap(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "base\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "base")
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")

	clone := filepath.Join(t.TempDir(), "other")
	programmingGitRun(t, m, "clone", "-q", "-b", "main", origin, clone)
	other := &programmingGitManager{root: clone, env: m.env}
	programmingGitRun(t, other, "config", "user.name", "Other")
	programmingGitRun(t, other, "config", "user.email", "other@example.com")
	programmingGitWrite(t, other, "app.txt", "remote\n")
	programmingGitRun(t, other, "add", "-A")
	programmingGitRun(t, other, "commit", "-q", "-m", "remote update")
	programmingGitRun(t, other, "push", "-q", "origin", "main")

	programmingGitWrite(t, m, "app.txt", "local\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local update")
	localHead := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))

	body, _ := json.Marshal(map[string]bool{"realign": true})
	var result struct {
		Pulled         bool   `json:"pulled"`
		Detail         string `json:"detail"`
		RollbackFailed bool   `json:"rollback_failed"`
	}
	programmingGitCall(t, h.GitPull, "POST", "/git-pull", string(body), &result)
	if result.Pulled || result.Detail == "" {
		t.Fatalf("overlapping update must be refused: %+v", result)
	}
	if result.RollbackFailed {
		t.Fatal("the branch was left rewound")
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != localHead {
		t.Fatalf("HEAD = %s, want the original %s", got, localHead)
	}
	if status := programmingGitRun(t, m, "status", "--porcelain"); status != "" {
		t.Fatalf("the tree should be clean again, got %q", status)
	}
}
