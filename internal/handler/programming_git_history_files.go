package handler

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/gin-gonic/gin"
)

type gitHistoryFile struct {
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
	Status  string `json:"status"`
}

// --name-status -z separates status and paths with NUL, including both paths
// for renames/copies. Never split filenames on tabs or newlines.
func parseGitHistoryFiles(raw string) []gitHistoryFile {
	fields := strings.Split(raw, "\x00")
	files := make([]gitHistoryFile, 0)
	for i := 0; i+1 < len(fields); {
		status, name := fields[i], fields[i+1]
		i += 2
		if status == "" || name == "" {
			continue
		}
		file := gitHistoryFile{Path: name, Status: status[:1]}
		if file.Status == "R" || file.Status == "C" {
			if i >= len(fields) || fields[i] == "" {
				break
			}
			file.OldPath, file.Path = name, fields[i]
			i++
		}
		files = append(files, file)
	}
	return files
}

// GitHistoryFiles reads a commit's changes relative to its first parent (or
// the empty tree for the initial commit). It never reads today's worktree, so
// deleted files, renames and incoming commits can all be inspected correctly.
func (h *ProgrammingHandler) GitHistoryFiles(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	commit := c.Query("commit")
	if !validGitHistorySHA(commit) {
		c.Error(apperrors.NewBadRequestError("invalid history commit"))
		return
	}
	if h.workspace == nil {
		c.Error(apperrors.NewConflictError("programming workspace is unavailable"))
		return
	}
	sessionID := programmingSessionID(c)
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, c.Query("repo"))
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	metadata, _, code, err := h.execGit(ctx, sessionID, workdir,
		"--no-pager show --no-patch --no-notes --no-show-signature --format='%H%x00%P' "+shellQuotePOSIX(commit)+" --", 10*time.Second)
	fields := strings.Split(strings.TrimSpace(metadata), "\x00")
	if err != nil || code != 0 || len(fields) != 2 || !strings.EqualFold(fields[0], commit) {
		c.Error(apperrors.NewNotFoundError("commit is unavailable in this repository"))
		return
	}
	parents := strings.Fields(fields[1])
	base := ""
	if len(parents) > 0 {
		base = parents[0]
	} else {
		base, _, code, err = h.execGit(ctx, sessionID, workdir, "hash-object -t tree --stdin < /dev/null", 10*time.Second)
		base = strings.TrimSpace(base)
	}
	if err != nil || code != 0 || !validGitHistorySHA(base) {
		c.Error(apperrors.NewConflictError("commit parent could not be read"))
		return
	}
	diffArgs := "--no-pager --literal-pathspecs diff --no-ext-diff --no-textconv --no-color -M " + shellQuotePOSIX(base) + " " + shellQuotePOSIX(commit)
	raw, _, code, err := h.execGit(ctx, sessionID, workdir, diffArgs+" --name-status -z --", 20*time.Second)
	if err != nil || code != 0 {
		c.Error(apperrors.NewConflictError("commit files could not be read"))
		return
	}
	files := parseGitHistoryFiles(raw)
	data := gin.H{"files": files, "parent": base}
	if requested, present := c.GetQuery("path"); present {
		var selected *gitHistoryFile
		for i := range files {
			if files[i].Path == requested {
				selected = &files[i]
				break
			}
		}
		if selected == nil {
			c.Error(apperrors.NewBadRequestError("file is not changed in this commit"))
			return
		}
		data["files"] = []gitHistoryFile{*selected}
		paths := []string{selected.Path}
		if selected.OldPath != "" {
			paths = append(paths, selected.OldPath)
		}
		// Bound the preview without putting file content or temporary output in
		// the repository. Disable external diff/textconv programs for this read.
		const maxPatch = 256 * 1024
		command := "tmp=$(mktemp) || exit 1; trap 'rm -f -- \"$tmp\"' EXIT; git -C " + shellQuotePOSIX(workdir) + " " +
			diffArgs + " --no-color --unified=3 -- " + quotedGitCommitPaths(paths) + " > \"$tmp\" && head -c 262145 \"$tmp\""
		result, execErr := h.workspace.ExecWorkspaceCommand(ctx, sessionID, gitEnvQuietPrefix+command, workdir, 30*time.Second, nil)
		if execErr != nil || result == nil || result.ExitCode != 0 {
			c.Error(apperrors.NewConflictError("commit file diff could not be read"))
			return
		}
		patch := strings.ToValidUTF8(result.Stdout, "�")
		data["patch_truncated"] = len(result.Stdout) > maxPatch || len(patch) > maxPatch
		if len(patch) > maxPatch {
			patch = patch[:maxPatch]
			for !utf8.ValidString(patch) && len(patch) > 0 {
				patch = patch[:len(patch)-1]
			}
		}
		data["patch"] = strings.ToValidUTF8(patch, "�")
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}
