package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// NUL cannot occur in a Git path; two consecutive NULs therefore separate the
// status and numstat blocks even when filenames contain control characters.
const gitStatusBlockSeparator = "\x00\x00"

const gitStatusCommandArgs = "status --porcelain=v1 -z --untracked-files=all && printf '\\000\\000' && " +
	"{ git diff --numstat -z HEAD -- 2>/dev/null || " +
	"git diff --numstat -z \"$(git hash-object -t tree --stdin < /dev/null)\" --; } && " +
	"{ git ls-files --others --exclude-standard -z | " +
	"xargs -0 -r -n1 git diff --no-index --numstat -z -- /dev/null 2>/dev/null; true; }"

// gitStatusEntry is one row in the conversation's change list: a working-tree
// file git considers dirty, with a normalized kind the frontend can render.
type gitStatusEntry struct {
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
	Status  string `json:"status"`
	Added   int    `json:"added"`
	Removed int    `json:"removed"`
}

// gitStatusReport is the JSON the frontend reads: a normalized list plus the
// ReviewThread-shaped availability flag so "not a git repo" never reads as an
// error.
type gitStatusReport struct {
	Available bool             `json:"available"`
	Files     []gitStatusEntry `json:"files"`
}

// gitNumstatEntry holds one file's line counts from `git diff --numstat`.
type gitNumstatEntry struct {
	Added   int
	Removed int
}

// parseGitStatusPorcelainZ decodes `git status --porcelain=v1 -z`. The -z
// form terminates each record with NUL so paths with spaces or quotes never
// get mangled by C-style quoting. A rename/copy is TWO NUL-separated records
// in "to then from" order — the source is the record immediately after the
// XY one (verified against git 2.x: `R  b.go\0a.go\0`), opposite to the
// non-z "from -> to" display form.
func parseGitStatusPorcelainZ(raw string) []gitStatusEntry {
	records := strings.Split(raw, "\x00")
	out := make([]gitStatusEntry, 0, len(records))
	for i := 0; i < len(records); i++ {
		rec := records[i]
		if rec == "" || len(rec) < 4 {
			continue
		}
		xy := rec[:2]
		entry := gitStatusEntry{Path: rec[3:], Status: normalizeGitStatusXY(xy)}
		// Consume the source according to the raw XY fields. A staged rename
		// subsequently deleted in the worktree is normalized to "deleted".
		if strings.ContainsAny(xy, "RC") && i+1 < len(records) {
			entry.OldPath = records[i+1]
			i++
		}
		out = append(out, entry)
	}
	return out
}

// normalizeGitStatusXY maps a porcelain XY pair to the small set of statuses
// the conversation's change list needs. Unknown pairs stay verbatim so an
// unrecognized record never silently disappears.
func normalizeGitStatusXY(xy string) string {
	switch {
	case xy == "??":
		return "untracked"
	case xy == "!!":
		return "ignored"
	case strings.Contains(xy, "A"):
		return "added"
	case strings.Contains(xy, "D"):
		return "deleted"
	case strings.Contains(xy, "R"):
		return "renamed"
	case strings.Contains(xy, "C"):
		return "copied"
	case strings.Contains(xy, "M") || strings.Contains(xy, "T"):
		return "modified"
	default:
		return xy
	}
}

// parseGitNumstat decodes `git diff --numstat -z`. The -z form only guards the
// PATHS: the two counts still sit on one TAB-separated record, and a rename
// then follows as a separate NUL-terminated source path before the terminating
// NUL. So a plain file reads
//
//	"<added>\t<removed>\t<path>\0"
//
// and a rename reads
//
//	"<added>\t<removed>\t\0<from>\0<to>\0"
//
// which is what this walks. The -z form is what makes non-ASCII and
// space-bearing paths match at all: the plain form C-quotes them
// ("\346\226\207...") while porcelain status always reports them verbatim.
// Both rename sides are keyed because status reports the destination as the
// path while either may carry the counts. Binary files report `-` for both
// columns and count as zero lines changed.
func parseGitNumstat(raw string) map[string]gitNumstatEntry {
	out := map[string]gitNumstatEntry{}
	records := strings.Split(raw, "\x00")
	for i := 0; i < len(records); i++ {
		rec := records[i]
		addedRaw, rest, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		removedRaw, path, ok := strings.Cut(rest, "\t")
		if !ok {
			continue
		}
		added, okA := parseNumstatCount(addedRaw)
		removed, okR := parseNumstatCount(removedRaw)
		if !okA || !okR {
			continue
		}
		entry := gitNumstatEntry{Added: added, Removed: removed}
		if path != "" {
			out[path] = entry
			continue
		}
		// Empty path: this record was a rename's counts, so the next two
		// records are the source and destination paths.
		if i+2 >= len(records) {
			break
		}
		// `diff --no-index /dev/null <path>` (used for untracked files) names
		// /dev/null as its source; it is not a repo path, so it is not keyed.
		if records[i+1] != "/dev/null" {
			out[records[i+1]] = entry
		}
		out[records[i+2]] = entry
		i += 2
	}
	return out
}

// parseNumstatCount reads one numstat count column. Binary entries are `-`,
// which counts as zero; the bool is false when the record is not a count at
// all.
func parseNumstatCount(record string) (int, bool) {
	if record == "-" {
		return 0, true
	}
	n, err := strconv.Atoi(record)
	if err != nil || n < 0 {
		return 0, false
	}
	return n, true
}

// buildGitStatusReport joins porcelain status with numstat line counts.
// Files with unavailable numstat output keep zero counts; the frontend falls
// back to its recorded diff for those.
func buildGitStatusReport(statusRaw, numstatRaw string) gitStatusReport {
	files := parseGitStatusPorcelainZ(statusRaw)
	numstat := parseGitNumstat(numstatRaw)
	for i, f := range files {
		if counts, ok := numstat[f.Path]; ok {
			files[i].Added = counts.Added
			files[i].Removed = counts.Removed
		}
	}
	return gitStatusReport{Available: true, Files: files}
}

// gitStatusReportJSON keeps the gin shape at the call site (available + files)
// without leaking the parser structs into the handler.
func gitStatusReportJSON(report gitStatusReport) gin.H {
	files := make([]gin.H, 0, len(report.Files))
	for _, f := range report.Files {
		row := gin.H{"path": f.Path, "status": f.Status, "added": f.Added, "removed": f.Removed}
		if f.OldPath != "" {
			row["old_path"] = f.OldPath
		}
		files = append(files, row)
	}
	return gin.H{"available": report.Available, "files": files}
}
