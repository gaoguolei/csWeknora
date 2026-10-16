package handler

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// resetTo calls the endpoint and returns the decoded result.
func resetTo(t *testing.T, h *ProgrammingHandler, commit string) (bool, string) {
	t.Helper()
	var data struct {
		Reset  bool   `json:"reset"`
		Detail string `json:"detail"`
	}
	body, _ := json.Marshal(gitResetRequest{Commit: commit})
	programmingGitCall(t, h.GitReset, "POST", "/git-reset", string(body), &data)
	return data.Reset, data.Detail
}

func TestProgrammingGitResetKeepsChangesOfDroppedCommits(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "first")
	first := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitWrite(t, m, "added.txt", "added\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "second")

	reset, detail := resetTo(t, h, first)
	if !reset {
		t.Fatalf("reset refused: %s", detail)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != first {
		t.Fatalf("HEAD = %s, want %s", got, first)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "log", "-1", "--format=%s")); got != "first" {
		t.Fatalf("dropped commit still on the branch: %q", got)
	}
	// The dropped commit's changes must survive as uncommitted work.
	if content, err := os.ReadFile(filepath.Join(m.root, "app.txt")); err != nil || string(content) != "second\n" {
		t.Fatalf("app.txt = %q, %v; want the edited content", content, err)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "added.txt")); err != nil || string(content) != "added\n" {
		t.Fatalf("added.txt = %q, %v; want the added file", content, err)
	}
	status := programmingGitRun(t, m, "status", "--porcelain")
	if !strings.Contains(status, "app.txt") || !strings.Contains(status, "added.txt") {
		t.Fatalf("restored changes are not visible in the tree: %q", status)
	}
}

func TestProgrammingGitResetRefusesPublishedCommit(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "first")
	first := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")
	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "second")
	programmingGitRun(t, m, "push", "-q", "origin", "main")
	programmingGitWrite(t, m, "app.txt", "third\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "third")
	third := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))

	// first is already published, so rewinding to it would rewrite remote history.
	reset, detail := resetTo(t, h, first)
	if reset || detail == "" {
		t.Fatalf("published commit must be refused: reset=%v detail=%q", reset, detail)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != third {
		t.Fatalf("refused reset moved HEAD to %s", got)
	}
}

func TestProgrammingGitResetAllowsUnpushedCommitWithUpstream(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "first")
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")
	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "second")
	second := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitWrite(t, m, "app.txt", "third\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "third")

	// second is above the upstream commit, so dropping third stays local.
	reset, detail := resetTo(t, h, second)
	if !reset {
		t.Fatalf("unpushed commit refused: %s", detail)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != second {
		t.Fatalf("HEAD = %s, want %s", got, second)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "app.txt")); err != nil || string(content) != "third\n" {
		t.Fatalf("dropped change was lost: %q, %v", content, err)
	}
}

func TestProgrammingGitResetRefusesDirtyTreeAndOtherCommit(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "first")
	first := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "second")
	second := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))

	// An unrelated branch's commit is not a target for this branch.
	programmingGitRun(t, m, "branch", "side", first)
	programmingGitWrite(t, m, "side.txt", "side\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "side only")
	side := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitRun(t, m, "reset", "--hard", "-q", second)

	if reset, detail := resetTo(t, h, side); reset || detail == "" {
		t.Fatalf("commit from another branch must be refused: reset=%v detail=%q", reset, detail)
	}
	if reset, detail := resetTo(t, h, second); reset || detail == "" {
		t.Fatalf("resetting to HEAD must be refused: reset=%v detail=%q", reset, detail)
	}
	programmingGitWrite(t, m, "dirty.txt", "dirty\n")
	if reset, detail := resetTo(t, h, first); reset || detail == "" {
		t.Fatalf("dirty tree must be refused: reset=%v detail=%q", reset, detail)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != second {
		t.Fatalf("refused reset moved HEAD to %s", got)
	}
}

func TestProgrammingGitResetRejectsMalformedCommit(t *testing.T) {
	h, _ := newProgrammingGitTest(t)
	body, _ := json.Marshal(gitResetRequest{Commit: "HEAD~1"})
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/git-reset", strings.NewReader(string(body)))
	c.Params = gin.Params{{Key: "session_id", Value: "git-test"}}
	h.GitReset(c)
	if len(c.Errors) == 0 || !strings.Contains(c.Errors.String(), "invalid reset commit") {
		t.Fatalf("non-object-id commit must be rejected before running git: %v", c.Errors)
	}
}

// Reverting to the upstream commit itself drops only unpushed commits, which is
// safe — but the panel must offer that target, otherwise a branch with a single
// unpushed commit has no revertable entry at all.
func TestProgrammingGitResetToUpstreamDropsOnlyLocalCommits(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "pushed commit")
	published := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")
	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local commit")

	reset, detail := resetTo(t, h, published)
	if !reset {
		t.Fatalf("resetting to the upstream commit was refused: %s", detail)
	}
	if got := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD")); got != published {
		t.Fatalf("HEAD = %s, want %s", got, published)
	}
	if content, err := os.ReadFile(filepath.Join(m.root, "app.txt")); err != nil || string(content) != "second\n" {
		t.Fatalf("dropped change was lost: %q, %v", content, err)
	}
}
