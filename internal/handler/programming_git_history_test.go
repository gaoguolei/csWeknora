package handler

import (
	"strings"
	"testing"
)

// The revert button only appears on commits the backend classifies as
// "outgoing" (or "unknown" when the branch has no upstream). Pin that
// classification against a real repository, because a wrong status silently
// removes the button from the panel.
func TestProgrammingGitHistoryClassifiesPushedAndLocalCommits(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	origin := t.TempDir()
	programmingGitRun(t, m, "init", "--bare", "-q", origin)

	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "pushed commit")
	pushed := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))
	programmingGitRun(t, m, "remote", "add", "origin", origin)
	programmingGitRun(t, m, "push", "-q", "-u", "origin", "main")

	programmingGitWrite(t, m, "app.txt", "second\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local commit")
	local := strings.TrimSpace(programmingGitRun(t, m, "rev-parse", "HEAD"))

	var report struct {
		Commits  []gitHistoryEntry `json:"commits"`
		Head     string            `json:"head"`
		Upstream string            `json:"upstream"`
	}
	programmingGitCall(t, h.GitHistory, "GET", "/git-history", "", &report)

	if report.Head != local || report.Upstream != pushed {
		t.Fatalf("history snapshot: head=%s upstream=%s", report.Head, report.Upstream)
	}
	statuses := make(map[string]string, len(report.Commits))
	for _, commit := range report.Commits {
		statuses[commit.SHA] = commit.Status
	}
	if got := statuses[local]; got != "outgoing" {
		t.Errorf("unpushed commit status = %q, want outgoing", got)
	}
	if got := statuses[pushed]; got != "synced" {
		t.Errorf("pushed commit status = %q, want synced", got)
	}
}

// A branch with no upstream keeps every commit at "unknown", which the panel
// treats as revertable.
func TestProgrammingGitHistoryWithoutUpstream(t *testing.T) {
	h, m := newProgrammingGitTest(t)
	programmingGitWrite(t, m, "app.txt", "first\n")
	programmingGitRun(t, m, "add", "-A")
	programmingGitRun(t, m, "commit", "-q", "-m", "local commit")

	var report struct {
		Commits  []gitHistoryEntry `json:"commits"`
		Upstream string            `json:"upstream"`
	}
	programmingGitCall(t, h.GitHistory, "GET", "/git-history", "", &report)
	if report.Upstream != "" {
		t.Fatalf("upstream = %q, want empty", report.Upstream)
	}
	if len(report.Commits) != 1 || report.Commits[0].Status != "unknown" {
		t.Fatalf("commits = %+v, want one unknown-status commit", report.Commits)
	}
}
