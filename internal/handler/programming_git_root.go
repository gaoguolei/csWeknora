package handler

import (
	"path/filepath"
	"sort"
	"strings"
)

// symbolic-ref also reports the branch before the repository's first commit.
// Detached HEADs fall back to their commit ID instead of the literal "HEAD".
const gitCurrentBranchArgs = "symbolic-ref --quiet --short HEAD || git rev-parse --short HEAD"

// The changes panel assumed the workspace root is a git repository, but the
// agent may clone one or more projects into subdirectories — /workspace/app
// and /workspace/lib each with their own .git while /workspace itself is not
// a repo. These helpers find every repository root so each project gets its
// own changes group, and git commands run against that repo root.
//
// Discovery is deliberately shallow: the workspace root itself, then the first
// level of subdirectories. Cloning into a subdirectory is the common case;
// deeper nesting is rare and not worth a recursive walk that could trip over
// node_modules.

// candidateGitRepoDirs returns the workspace-relative directories worth probing
// for a .git, sorted for determinism: the root itself (""), then each
// first-level subdirectory in name order. Input paths may carry the
// workspace-root prefix; it is stripped. Only first-level directories qualify.
func candidateGitRepoDirs(workspaceRoot string, dirPaths []string) []string {
	prefix := strings.TrimSuffix(workspaceRoot, "/") + "/"
	subs := make([]string, 0, len(dirPaths))
	for _, p := range dirPaths {
		rel := strings.TrimPrefix(p, prefix)
		if rel == "" || strings.Contains(rel, "/") {
			continue // only first-level subdirectories of the workspace root
		}
		subs = append(subs, rel)
	}
	sort.Strings(subs)
	// Root first, then subdirectories in name order.
	return append([]string{""}, subs...)
}

// gitWorkdir resolves the working directory git commands should run in: the
// repository root when it sits in a subdirectory, else the workspace root.
// relRepo is the workspace-relative repo root ("" = root).
func gitWorkdir(workspaceRoot, relRepo string) string {
	if relRepo == "" {
		return workspaceRoot
	}
	return strings.TrimSuffix(workspaceRoot, "/") + "/" + relRepo
}

// gitRepoName is the display name for a repository group: the directory name
// for a subdirectory repo, or the workspace's own name for a root repo.
func gitRepoName(workspaceRoot, relRepo string) string {
	if relRepo != "" {
		return relRepo
	}
	trimmed := strings.TrimSuffix(workspaceRoot, "/")
	if i := strings.LastIndex(trimmed, "/"); i >= 0 {
		return trimmed[i+1:]
	}
	return trimmed
}

// gitNeedsIdentity reports whether a git failure is the missing-identity case
// the panel can offer to fix. git phrases it a few ways depending on version
// and whether name or email is the missing half; all share "tell me who you
// are" or "unable to auto-detect".
func gitNeedsIdentity(detail string) bool {
	s := strings.ToLower(detail)
	return strings.Contains(s, "tell me who you are") ||
		strings.Contains(s, "unable to auto-detect") ||
		strings.Contains(s, "author identity unknown") ||
		strings.Contains(s, "committer identity unknown")
}

// parseGitBranchRefs separates local and remote refs. Symbolic aliases such as
// origin/HEAD are not switchable branches and must not become dropdown items.
func parseGitBranchRefs(raw string) (local, remote []string) {
	local, remote = make([]string, 0), make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		ref, symref, _ := strings.Cut(line, "\t")
		if symref != "" {
			continue
		}
		if name, ok := strings.CutPrefix(ref, "refs/heads/"); ok && name != "" {
			local = append(local, name)
		} else if name, ok := strings.CutPrefix(ref, "refs/remotes/"); ok && name != "" {
			remote = append(remote, name)
		}
	}
	return local, remote
}

// Remote names may themselves contain slashes. Match the longest configured
// remote prefix instead of assuming the first slash starts the branch name.
func localGitBranchForRemote(branch, remotesRaw string) string {
	matched := ""
	for _, remote := range strings.Split(remotesRaw, "\n") {
		if remote != "" && strings.HasPrefix(branch, remote+"/") && len(remote) > len(matched) {
			matched = remote
		}
	}
	if matched == "" {
		return ""
	}
	return strings.TrimPrefix(branch, matched+"/")
}

// validGitBranchName guards the switch endpoint: a branch name becomes a
// `git switch` argument, so it must never start with '-' (an option) or
// carry ref metacharacters. git's own rules are stricter; this is
// the minimal safe subset the branch list can already produce.
func validGitBranchName(name string) bool {
	if name == "" || strings.HasPrefix(name, "-") {
		return false
	}
	// Reject whitespace and git's ref metacharacters. '[' and ']' are listed
	// via a Contains check instead of a character class to avoid the escaping
	// ambiguity of "[]" inside a set.
	if strings.ContainsAny(name, " \t\n\r~^:?*") {
		return false
	}
	return !strings.ContainsAny(name, "[]\\")
}

// sameFilesystemPath reports whether two paths name the same directory. git's
// --show-toplevel resolves symlinks (macOS /tmp -> /private/tmp), while the
// workdir we built does not, so a literal string compare would reject every
// repo on such systems. We clean both and resolve symlinks, falling back to
// the cleaned form when resolution fails (e.g. the path is inside a sandbox we
// can't stat from the host).
func sameFilesystemPath(a, b string) bool {
	ca, cb := filepath.Clean(a), filepath.Clean(b)
	if ca == cb {
		return true
	}
	if ra, err := filepath.EvalSymlinks(ca); err == nil {
		ca = ra
	}
	if rb, err := filepath.EvalSymlinks(cb); err == nil {
		cb = rb
	}
	return ca == cb
}
