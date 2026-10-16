package handler

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// workspaceStateLive and workspaceStateExpired tell the code panel whether the
// session still owns a sandbox. A reclaimed sandbox leaves the tree empty, and
// an empty tree on its own reads as "nothing was created yet".
const (
	workspaceStateLive     = "live"
	workspaceStateExpired  = "expired"
	workspaceExpiredReason = "workspace_expired"
)

const (
	programmingMaxFileBytes  = 2 * 1024 * 1024
	programmingMaxImageBytes = 20 * 1024 * 1024
	programmingMaxCommand    = 8 * 1024
	programmingMaxTimeout    = 60 * time.Second
)

// ProgrammingHandler exposes the P0 programming-space surface on top of the
// existing session sandbox. A session is the current compatibility boundary:
// it already owns the Agent conversation, sandbox pin, user scope and
// terminal lifecycle, so the programming UI cannot accidentally escape those
// existing authorization rules.
type ProgrammingHandler struct {
	tests          *service.WorkspaceTestService
	previewLeases  *previewLeaseStore
	sessions       interfaces.SessionService
	workspace      *service.PinnedSessionSandbox
	previewStarter *service.SandboxTerminalService
	previewAgents  interfaces.CustomAgentService
	previewShares  interfaces.AgentShareService
}

func NewProgrammingHandler(
	sessions interfaces.SessionService,
	workspace *service.PinnedSessionSandbox,
) *ProgrammingHandler {
	return &ProgrammingHandler{sessions: sessions, workspace: workspace}
}

func NewProgrammingHandlerWithPreview(sessions interfaces.SessionService, workspace *service.PinnedSessionSandbox,
	starter *service.SandboxTerminalService, agents interfaces.CustomAgentService, shares interfaces.AgentShareService) *ProgrammingHandler {
	h := NewProgrammingHandler(sessions, workspace)
	h.previewStarter, h.previewAgents, h.previewShares = starter, agents, shares
	return h
}

type programmingFileRequest struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	BaseHash string `json:"base_hash,omitempty"`
}

type programmingMoveFileRequest struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type programmingCommandRequest struct {
	Command string `json:"command"`
	Path    string `json:"path,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

type programmingNode struct {
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Kind     string    `json:"kind"`
	Size     int64     `json:"size,omitempty"`
	Modified time.Time `json:"modified,omitempty"`
}

type programmingFile struct {
	Path     string    `json:"path"`
	Content  string    `json:"content"`
	Hash     string    `json:"hash"`
	Size     int       `json:"size"`
	Modified time.Time `json:"modified,omitempty"`
	Encoding string    `json:"encoding,omitempty"`
	MIMEType string    `json:"mime_type,omitempty"`
}

// ListTree returns the current workspace files. The store performs the
// provider-specific path checks; this handler only projects absolute provider
// paths back into the relative paths shown in the editor.
func (h *ProgrammingHandler) ListTree(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	sessionID := programmingSessionID(c)
	requestedPath := strings.TrimSpace(strings.ReplaceAll(c.Query("path"), "\\", "/"))
	directory := layout.Root
	if requestedPath != "" {
		resolved, rel, resolveErr := resolveWorkspacePath(layout, requestedPath)
		if resolveErr != nil || rel == "" {
			c.Error(apperrors.NewBadRequestError("path must stay inside the workspace"))
			return
		}
		directory = resolved
	}

	var (
		entries []sandbox.RemoteDirEntry
		err     error
	)
	if lister, supported := store.(interface {
		ListSessionDirectory(context.Context, string, string) ([]sandbox.RemoteDirEntry, error)
	}); supported {
		entries, err = lister.ListSessionDirectory(ctx, sessionID, directory)
	} else {
		entries, err = store.ListSessionFiles(ctx, sessionID, directory)
	}
	workspaceState := workspaceStateLive
	if errors.Is(err, sandbox.ErrNoLiveSessionSandbox) {
		// No live sandbox: an empty tree either way. A session that still
		// carries a binding had its sandbox reaped, and the panel should say
		// so; a session that was never provisioned is simply empty.
		if h.reclaimedWorkspace(ctx, sessionID) {
			workspaceState = workspaceStateExpired
		}
		entries, err = nil, nil
	}
	if err != nil {
		c.Error(apperrors.NewConflictError("workspace is not running or cannot be listed"))
		return
	}

	nodes := make([]programmingNode, 0, len(entries)*2)
	seen := make(map[string]bool, len(entries))
	for _, entry := range entries {
		rel, err := relativeWorkspacePath(layout.Root, entry.Path)
		if err != nil || rel == "" || seen[rel] {
			continue
		}
		seen[rel] = true
		nodes = append(nodes, programmingNode{
			Path:     rel,
			Name:     entry.Name,
			Kind:     map[bool]string{true: "directory", false: "file"}[entry.Type == sandbox.RemoteEntryDir],
			Size:     entry.Size,
			Modified: entry.ModTime,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Kind != nodes[j].Kind {
			return nodes[i].Kind == "directory"
		}
		return nodes[i].Path < nodes[j].Path
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"root":            layout.Root,
		"origin":          map[bool]string{true: "host", false: "sandbox"}[layout.IsHost()],
		"workspace_state": workspaceState,
		"nodes":           nodes,
	}})
}

// workspaceExpiredError converts a reclaimed-sandbox failure into the
// answer the code panel acts on. HTTP 404 keeps the client's "this path is
// gone" handling — a reclaimed workspace really does not contain the file —
// while the reason tells it not to blame the path.
func workspaceExpiredError() *apperrors.AppError {
	return apperrors.NewNotFoundError(
		"the session workspace has been reclaimed, so its files are no longer available").
		WithDetails(gin.H{"reason": workspaceExpiredReason})
}

// reclaimedWorkspace reports whether the session's file operations failed
// because the sandbox was reaped. A session that never had a sandbox has no
// binding to lose, and its empty workspace is normal rather than expired.
func (h *ProgrammingHandler) reclaimedWorkspace(ctx context.Context, sessionID string) bool {
	if h == nil || h.workspace == nil {
		return false
	}
	_, bound := h.workspace.BoundSandboxID(ctx, sessionID)
	return bound
}

// reclaimedWorkspaceError turns a no-live-sandbox failure into the 404 that
// explains a reaped sandbox, or returns nil for any other failure.
func (h *ProgrammingHandler) reclaimedWorkspaceError(
	ctx context.Context, sessionID string, err error,
) error {
	if !errors.Is(err, sandbox.ErrNoLiveSessionSandbox) || !h.reclaimedWorkspace(ctx, sessionID) {
		return nil
	}
	return workspaceExpiredError()
}

// workspaceWriteConflict answers a failed mutation. A reclaimed sandbox is a
// conflict rather than a bad request, but the reason lets the panel offer the
// one action that helps: ask the Agent to rebuild the workspace.
func (h *ProgrammingHandler) workspaceWriteConflict(
	c *gin.Context, sessionID string, err error, message string,
) {
	if errors.Is(err, sandbox.ErrNoLiveSessionSandbox) &&
		h.reclaimedWorkspace(c.Request.Context(), sessionID) {
		c.Error(apperrors.NewConflictError(
			"the session workspace has been reclaimed and cannot be modified").
			WithDetails(gin.H{"reason": workspaceExpiredReason}))
		return
	}
	c.Error(apperrors.NewConflictError(message))
}

// ReadFile reads a workspace file. Browser-supported raster images are returned
// as base64 for the image viewer; other binary files never enter the text editor.
func (h *ProgrammingHandler) ReadFile(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	absolute, rel, err := resolveWorkspacePath(layout, c.Query("path"))
	if err != nil || rel == "" {
		c.Error(apperrors.NewBadRequestError("path must stay inside the workspace"))
		return
	}
	sessionID := programmingSessionID(c)
	stat, err := store.StatSessionFile(ctx, sessionID, absolute)
	if err != nil {
		// A reclaimed sandbox is reported before plain not-found: both answer
		// 404, but only the expired case can tell the reader that the whole
		// workspace is gone instead of sending them looking for a path bug.
		if expired := h.reclaimedWorkspaceError(ctx, sessionID, err); expired != nil {
			c.Error(expired)
			return
		}
		// Host workspaces surface the operating system's own missing-file
		// error, which means the same thing as a provider not-found.
		if sandbox.IsRemoteNotFound(err) || errors.Is(err, fs.ErrNotExist) {
			c.Error(apperrors.NewNotFoundError("file not found"))
			return
		}
		c.Error(apperrors.NewConflictError("workspace is not running or the file cannot be read"))
		return
	}
	if stat == nil || stat.Type != sandbox.RemoteEntryFile {
		c.Error(apperrors.NewNotFoundError("file not found"))
		return
	}
	imageMIME := programmingImageMIME(rel)
	maxBytes := programmingMaxFileBytes
	if imageMIME != "" {
		maxBytes = programmingMaxImageBytes
	}
	if stat.Size > int64(maxBytes) {
		c.Error(apperrors.NewBadRequestError("file is too large for the editor"))
		return
	}
	content, err := store.ReadSessionFile(ctx, sessionID, absolute)
	if err != nil {
		if expired := h.reclaimedWorkspaceError(ctx, sessionID, err); expired != nil {
			c.Error(expired)
			return
		}
		if sandbox.IsRemoteNotFound(err) || errors.Is(err, fs.ErrNotExist) {
			c.Error(apperrors.NewNotFoundError("file not found"))
			return
		}
		c.Error(apperrors.NewConflictError("workspace is not running or the file cannot be read"))
		return
	}
	// Recheck the bytes in case the file grew between stat and read.
	if len(content) > maxBytes {
		c.Error(apperrors.NewBadRequestError("file is too large for the viewer"))
		return
	}
	if imageMIME != "" {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": programmingFile{
			Path: rel, Content: base64.StdEncoding.EncodeToString(content),
			Hash: contentHash(content), Size: len(content), Modified: stat.ModTime,
			Encoding: "base64", MIMEType: imageMIME,
		}})
		return
	}
	if strings.IndexByte(string(content), 0) >= 0 {
		c.Error(apperrors.NewBadRequestError("binary files cannot be opened in the editor"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": filePayload(rel, content, stat.ModTime)})
}

func programmingImageMIME(filePath string) string {
	switch strings.ToLower(path.Ext(filePath)) {
	case ".png", ".apng":
		return "image/png"
	case ".jpg", ".jpeg", ".jfif":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".ico":
		return "image/x-icon"
	case ".avif":
		return "image/avif"
	default:
		return ""
	}
}

// WriteFile applies an optimistic hash check before writing. This is the
// first version of the requested FileVersion contract: clients send the hash
// they opened, and an out-of-date write receives VERSION_CONFLICT instead of
// overwriting another editor or Agent's change.
func (h *ProgrammingHandler) WriteFile(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req programmingFileRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid file request"))
		return
	}
	if len(req.Content) > programmingMaxFileBytes {
		c.Error(apperrors.NewBadRequestError("file is too large"))
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	absolute, rel, err := resolveWorkspacePath(layout, req.Path)
	if err != nil || rel == "" {
		c.Error(apperrors.NewBadRequestError("path must stay inside the workspace"))
		return
	}
	sessionID := programmingSessionID(c)
	current, stat, readErr := readWorkspaceFile(ctx, store, sessionID, absolute)
	if readErr == nil && req.BaseHash != "" && req.BaseHash != contentHash(current) {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "VERSION_CONFLICT", "message": "file changed since it was opened", "data": filePayload(rel, current, stat.ModTime)})
		return
	}
	if readErr != nil && req.BaseHash != "" {
		c.JSON(http.StatusConflict, gin.H{"success": false, "code": "VERSION_CONFLICT", "message": "file was created or removed since it was opened"})
		return
	}
	if err := store.WriteSessionWorkspaceFile(ctx, sessionID, absolute, []byte(req.Content)); err != nil {
		h.workspaceWriteConflict(c, sessionID, err, "workspace is not writable")
		return
	}
	now := time.Now()
	if latest, latestStat, err := readWorkspaceFile(ctx, store, sessionID, absolute); err == nil {
		current, now = latest, latestStat.ModTime
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": filePayload(rel, current, now)})
}

// CreateFile creates an empty text file and refuses to overwrite an existing
// file. The editor uses this endpoint for the explorer's new-file action so a
// stale, partially loaded tree cannot accidentally replace a project file.
func (h *ProgrammingHandler) CreateFile(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req programmingFileRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid file request"))
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	absolute, rel, err := resolveWorkspacePath(layout, req.Path)
	if err != nil || rel == "" {
		c.Error(apperrors.NewBadRequestError("path must stay inside the workspace"))
		return
	}
	sessionID := programmingSessionID(c)
	if stat, statErr := store.StatSessionFile(ctx, sessionID, absolute); statErr == nil && stat != nil {
		c.Error(apperrors.NewConflictError("file already exists"))
		return
	}
	if err := store.WriteSessionWorkspaceFile(ctx, sessionID, absolute, []byte{}); err != nil {
		h.workspaceWriteConflict(c, sessionID, err, "workspace is not writable")
		return
	}
	now := time.Now()
	if stat, statErr := store.StatSessionFile(ctx, sessionID, absolute); statErr == nil && stat != nil {
		now = stat.ModTime
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": filePayload(rel, nil, now)})
}

// MoveFile moves one regular file to another existing directory. The provider
// keeps its filesystem-specific move implementation behind the file store.
func (h *ProgrammingHandler) MoveFile(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req programmingMoveFileRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid move request"))
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	source, sourceRel, err := resolveWorkspacePath(layout, req.Source)
	if err != nil || sourceRel == "" {
		c.Error(apperrors.NewBadRequestError("source path must stay inside the workspace"))
		return
	}
	target, targetRel, err := resolveWorkspacePath(layout, req.Target)
	if err != nil || targetRel == "" || path.Clean(source) == path.Clean(target) {
		c.Error(apperrors.NewBadRequestError("target path must stay inside the workspace"))
		return
	}
	sessionID := programmingSessionID(c)
	sourceStat, statErr := store.StatSessionFile(ctx, sessionID, source)
	if expired := h.reclaimedWorkspaceError(ctx, sessionID, statErr); expired != nil {
		c.Error(expired)
		return
	}
	if statErr != nil || sourceStat == nil || sourceStat.Type != sandbox.RemoteEntryFile {
		c.Error(apperrors.NewNotFoundError("source file not found"))
		return
	}
	if targetStat, targetErr := store.StatSessionFile(ctx, sessionID, target); targetErr == nil && targetStat != nil {
		c.Error(apperrors.NewConflictError("target file already exists"))
		return
	}
	parentStat, parentErr := store.StatSessionFile(ctx, sessionID, path.Dir(target))
	if parentErr != nil || parentStat == nil || parentStat.Type != sandbox.RemoteEntryDir {
		c.Error(apperrors.NewBadRequestError("target directory does not exist"))
		return
	}
	mover, supported := store.(interface {
		MoveSessionWorkspaceFile(context.Context, string, string, string) error
	})
	if !supported {
		c.Error(apperrors.NewConflictError("workspace does not support moving files"))
		return
	}
	if err := mover.MoveSessionWorkspaceFile(ctx, sessionID, source, target); err != nil {
		h.workspaceWriteConflict(c, sessionID, err, "workspace could not move the file")
		return
	}
	payload := filePayload(targetRel, nil, sourceStat.ModTime)
	payload.Size = int(sourceStat.Size)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": payload})
}

// DeleteFile removes one regular file from the active workspace.
func (h *ProgrammingHandler) DeleteFile(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req programmingFileRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid delete request"))
		return
	}
	layout, store, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	absolute, rel, err := resolveWorkspacePath(layout, req.Path)
	if err != nil || rel == "" {
		c.Error(apperrors.NewBadRequestError("path must stay inside the workspace"))
		return
	}
	sessionID := programmingSessionID(c)
	stat, statErr := store.StatSessionFile(ctx, sessionID, absolute)
	if expired := h.reclaimedWorkspaceError(ctx, sessionID, statErr); expired != nil {
		c.Error(expired)
		return
	}
	if statErr != nil || stat == nil || stat.Type != sandbox.RemoteEntryFile {
		c.Error(apperrors.NewNotFoundError("file not found"))
		return
	}
	remover, supported := store.(interface {
		RemoveSessionWorkspaceFile(context.Context, string, string) error
	})
	if !supported {
		c.Error(apperrors.NewConflictError("workspace does not support deleting files"))
		return
	}
	if err := remover.RemoveSessionWorkspaceFile(ctx, sessionID, absolute); err != nil {
		h.workspaceWriteConflict(c, sessionID, err, "workspace file could not be deleted")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"path": rel}})
}

// workspaceGitRepo is one git repository discovered in the session workspace.
// RelRoot is the workspace-relative repo root ("" for the workspace root);
// Name is what the changes panel labels the group.
type workspaceGitRepo struct {
	RelRoot string
	Name    string
}

// gitEnvQuietPrefix clears the environment variables Git treats as "enabled
// when present". Assigning "0" does not silence them — GIT_CURL_VERBOSE=0 still
// dumps every curl exchange — so an inherited value (for example a debug flag a
// user set to 0) would flood the panel with transport logs. Removing the
// variables is the only reliable way to stop them. Askpass programs are cleared
// here too: a sandbox or host GIT_ASKPASS could otherwise prompt, or fail with
// its own noise, instead of letting Git report the real authentication result.
const gitEnvQuietPrefix = "unset GIT_TRACE GIT_TRACE_PACKET GIT_TRACE_PACK_ACCESS" +
	" GIT_TRACE_PERFORMANCE GIT_TRACE_SETUP GIT_TRACE_SHALLOW GIT_TRACE_CURL" +
	" GIT_TRACE_CURL_NO_DATA GIT_CURL_VERBOSE GIT_TRACE2 GIT_TRACE2_EVENT" +
	" GIT_TRACE2_PERF GIT_ASKPASS SSH_ASKPASS; "

// execGit runs a fixed git command against one repository. The command is
// prefixed with `git -C <repoWorkdir>` so it operates on the discovered repo
// root regardless of where the workspace root is. Only fixed, server-built
// commands ever reach this — never user-supplied shell.
func (h *ProgrammingHandler) execGit(ctx context.Context, sessionID, repoWorkdir, args string, timeout time.Duration) (string, string, int, error) {
	result, err := h.workspace.ExecWorkspaceCommand(
		ctx, sessionID, gitEnvQuietPrefix+"git -C "+shellQuotePOSIX(repoWorkdir)+" "+args, repoWorkdir, timeout, nil,
	)
	if err != nil {
		return "", "", 0, err
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// discoverWorkspaceGitRepos finds every git repository in the workspace: the
// root itself, then each first-level subdirectory. A directory is a repo when
// `git -C <dir> rev-parse --show-toplevel` succeeds. Returns nil when the
// workspace holds no repository at all (the panel's "not a git workspace"
// state).
func (h *ProgrammingHandler) discoverWorkspaceGitRepos(ctx context.Context, sessionID string, layout sandbox.WorkspaceLayout, store sandbox.SessionFileStore) []workspaceGitRepo {
	var dirPaths []string
	if store != nil {
		var entries []sandbox.RemoteDirEntry
		var err error
		if lister, ok := store.(interface {
			ListSessionDirectory(context.Context, string, string) ([]sandbox.RemoteDirEntry, error)
		}); ok {
			entries, err = lister.ListSessionDirectory(ctx, sessionID, layout.Root)
		} else {
			entries, err = store.ListSessionFiles(ctx, sessionID, layout.Root)
		}
		if err == nil {
			for _, e := range entries {
				if e.Type == sandbox.RemoteEntryDir {
					dirPaths = append(dirPaths, e.Path)
				}
			}
		}
	}
	repos := make([]workspaceGitRepo, 0, 1)
	candidates := candidateGitRepoDirs(layout.Root, dirPaths)
	if len(candidates) == 0 {
		return repos
	}
	// Probe candidates in parallel: each `rev-parse` is a remote shell call, and
	// serializing them adds up on a workspace with several first-level
	// subdirectories. The common "root is the only repo" case still answers in
	// one batch instead of N+1 round-trips. Each goroutine writes its own slot,
	// so the sorted candidate order is preserved without locking.
	probes := make([]*workspaceGitRepo, len(candidates))
	var wg sync.WaitGroup
	for i, rel := range candidates {
		wg.Add(1)
		go func(i int, rel string) {
			defer wg.Done()
			workdir := gitWorkdir(layout.Root, rel)
			// A candidate is a repository root only when its own toplevel resolves to
			// itself. `rev-parse --git-dir` would be wrong here: it succeeds in ANY
			// subdirectory of a repo by walking up to the parent's .git, which would
			// mislabel every plain subdirectory of a root repo as its own repository.
			// --show-toplevel instead tells us which repo actually owns this dir; only
			// when that equals the candidate do we have an independent root.
			topOut, _, code, err := h.execGit(ctx, sessionID, workdir, "rev-parse --show-toplevel", 10*time.Second)
			if err != nil || code != 0 {
				return
			}
			top := strings.TrimSuffix(topOut, "\n")
			// git resolves symlinks in the toplevel (e.g. /tmp -> /private/tmp on
			// macOS), while workdir carries the path as we built it. Compare on the
			// cleaned, symlink-resolved form of each so a resolved path still matches.
			if !sameFilesystemPath(top, workdir) {
				// This dir lives inside a repo rooted elsewhere (already covered by
				// the root repo, or it's a nested checkout we deliberately skip).
				return
			}
			probes[i] = &workspaceGitRepo{RelRoot: rel, Name: gitRepoName(layout.Root, rel)}
		}(i, rel)
	}
	wg.Wait()
	for _, repo := range probes {
		if repo != nil {
			repos = append(repos, *repo)
		}
	}
	return repos
}

// GitStatus reports the session workspace's uncommitted changes for the
// conversation's "变更" panel, grouped by repository: each discovered repo gets
// its own file list, so a project cloned into a subdirectory shows up the same
// as one at the workspace root. Each repo runs `git status --porcelain=v1 -z`
// joined with `git diff --numstat HEAD --` (or an empty-tree diff on a repo
// without a first commit). Read-only; nothing here writes to git.
func (h *ProgrammingHandler) GitStatus(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
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

	repos := h.discoverWorkspaceGitRepos(ctx, sessionID, layout, store)
	if len(repos) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"available": false, "repos": []gin.H{},
		}})
		return
	}

	repoReports := make([]gin.H, 0, len(repos))
	for _, repo := range repos {
		workdir := gitWorkdir(layout.Root, repo.RelRoot)
		// status and numstat run as one shell command: the panel needs both, and
		// two git invocations each pay a full sandbox round-trip (workspace prep
		// plus the command itself). Folding them into one halves that cost and
		// removes the dependency on concurrent shell execution. A double-NUL
		// separator splits the NUL-terminated status block from the numstat
		// records.
		//
		// Each diff stage is best-effort so one failing still yields a usable
		// file list. `diff HEAD` fails on a repository without a first commit, so
		// an empty-tree diff stands in for it, including unstaged edits to added
		// files. Neither sees untracked files, which is
		// why `ls-files --others` names them and `diff --no-index /dev/null <path>`
		// counts them — that last command exits 1 whenever it finds a difference,
		// so its output is read regardless of exit code.
		args := gitStatusCommandArgs + " && printf '\\000\\000' && { " + gitCachedRemoteCommandArgs + "; }"
		combined, _, code, err := h.execGit(ctx, sessionID, workdir, args, 15*time.Second)
		if err != nil || code != 0 {
			// A repo that vanished or broke between discovery and status is
			// skipped, not fatal to the others.
			continue
		}
		parts := strings.SplitN(combined, gitStatusBlockSeparator, 3)
		numstat := ""
		if len(parts) > 1 {
			numstat = parts[1]
		}
		report := buildGitStatusReport(parts[0], numstat)
		row := gin.H{
			"root":  repo.RelRoot,
			"name":  repo.Name,
			"files": gitStatusReportJSON(report)["files"],
		}
		if len(parts) > 2 {
			row["remote"] = gitRemoteReportJSON(parseGitCachedRemote(parts[2]))
		}
		repoReports = append(repoReports, row)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"available": true, "repos": repoReports,
	}})
}

// gitCommitRequest is the body of the manual commit action. Message is the
// commit message; Repo is the workspace-relative repository root this commit
// targets ("" = the workspace root repo). Paths is the explicitly selected
// file list. A nil Paths retains the all-files behavior for legacy callers;
// an explicitly empty list must never silently stage everything.
type gitCommitRequest struct {
	Message string   `json:"message"`
	Repo    string   `json:"repo"`
	Paths   []string `json:"paths"`
}

// resolveGitRepoWorkdir validates the client-supplied repo root against the
// repositories actually discovered in the workspace, returning the absolute
// workdir git should run in. An empty repo picks the sole repository — or the
// root repo when that's the only kind present. A repo that was never
// discovered (a guessed or traversal path) is rejected so the write endpoints
// can only ever touch a repo the panel actually showed.
func (h *ProgrammingHandler) resolveGitRepoWorkdir(ctx context.Context, sessionID string, layout sandbox.WorkspaceLayout, store sandbox.SessionFileStore, repo string) (string, error) {
	repos := h.discoverWorkspaceGitRepos(ctx, sessionID, layout, store)
	if repo == "" {
		if len(repos) == 1 {
			return gitWorkdir(layout.Root, repos[0].RelRoot), nil
		}
		// Ambiguous or absent: only a root repo is a safe implicit target.
		for _, r := range repos {
			if r.RelRoot == "" {
				return layout.Root, nil
			}
		}
		return "", fmt.Errorf("repository is ambiguous; specify one")
	}
	for _, r := range repos {
		if r.RelRoot == repo {
			return gitWorkdir(layout.Root, repo), nil
		}
	}
	return "", fmt.Errorf("repository not found in workspace")
}

// GitCommit powers the "commit" action in the conversation's "变更" panel.
// Unlike GitStatus it writes to git, but only through the two fixed commands
// buildGitCommitCommands renders — the free-form RunCommand channel stays the
// only way to run arbitrary shell, and this endpoint can never become one.
// The message reaches git as one shell-quoted -m argument, so multi-line
// messages and shell metacharacters survive verbatim.
func (h *ProgrammingHandler) GitCommit(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req gitCommitRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid commit request"))
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		c.Error(apperrors.NewBadRequestError("commit message is required"))
		return
	}
	if len(message) > gitCommitMaxMessage {
		c.Error(apperrors.NewBadRequestError("commit message is too long"))
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
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, req.Repo)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}

	if err := h.ensureLocalGitAuthor(ctx, sessionID, workdir); err != nil {
		c.Error(apperrors.NewConflictError(err.Error()))
		return
	}

	addCmd, commitCmd := buildGitCommitCommands(message)
	var selected []string
	if req.Paths != nil {
		statusOut, _, statusCode, statusErr := h.execGit(ctx, sessionID, workdir, "status --porcelain=v1 -z --untracked-files=all", 10*time.Second)
		if statusErr != nil || statusCode != 0 {
			c.Error(apperrors.NewConflictError("git changes could not be read"))
			return
		}
		selected, err = selectedGitCommitPaths(req.Paths, statusOut)
		if err != nil {
			c.Error(apperrors.NewBadRequestError(err.Error()))
			return
		}
		addCmd, commitCmd = buildSelectedGitCommitCommands(message, selected)
	}
	for step, cmd := range []string{addCmd, commitCmd} {
		if step == 1 && req.Paths != nil {
			// An added-then-deleted file can disappear from the index after
			// staging. Only pass paths known to the index or HEAD to --only;
			// deleted tracked files and both rename sides remain supported.
			pathspecs := quotedGitCommitPaths(selected)
			knownOut, _, knownCode, knownErr := h.execGit(ctx, sessionID, workdir,
				"--literal-pathspecs ls-files -z --cached -- "+pathspecs+
					" && { git -C "+shellQuotePOSIX(workdir)+" --literal-pathspecs ls-tree -r --name-only -z HEAD -- "+pathspecs+" 2>/dev/null || true; }", 10*time.Second)
			if knownErr != nil || knownCode != 0 {
				c.Error(apperrors.NewConflictError("selected git paths could not be read"))
				return
			}
			known := make(map[string]bool)
			for _, p := range strings.Split(knownOut, "\x00") {
				known[p] = true
			}
			commitPaths := make([]string, 0, len(selected))
			for _, p := range selected {
				if known[p] {
					commitPaths = append(commitPaths, p)
				}
			}
			if len(commitPaths) == 0 {
				c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"committed": false, "detail": "selected files have no remaining changes to commit"}})
				return
			}
			_, cmd = buildSelectedGitCommitCommands(message, commitPaths)
		}
		stdout, stderr, exitCode, execErr := h.execGit(ctx, sessionID, workdir, strings.TrimPrefix(cmd, "git "), 30*time.Second)
		if execErr != nil {
			c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
			return
		}
		if exitCode != 0 {
			detail := strings.TrimSpace(stderr)
			if detail == "" {
				detail = strings.TrimSpace(stdout)
			}
			// Preserve Git's reason for a refused local commit. The legacy
			// needs_identity flag remains for other API consumers; this panel
			// supplies missing author metadata automatically and never prompts
			// for remote credentials during a local commit.
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
				"committed":      false,
				"detail":         detail,
				"needs_identity": gitNeedsIdentity(detail),
			}})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"committed": true}})
}

// GitRemote reports how the session workspace's branch relates to its upstream
// for the changes panel's remote strip: branch, upstream, ahead/behind counts
// and the commits the branch is behind. When the caller asks to refresh, a
// bounded `git fetch` runs first; a sandbox without network keeps the last
// known refs instead of failing the whole read. Everything here is read-only
// apart from that fetch — the state-changing pull lives in GitPull.
func (h *ProgrammingHandler) GitRemote(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
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
	run := func(args string, timeout time.Duration) (string, int, error) {
		stdout, _, code, execErr := h.execGit(ctx, sessionID, workdir, args, timeout)
		if execErr != nil {
			return "", 0, execErr
		}
		return stdout, code, nil
	}

	report := gitRemoteReport{Available: true}
	branchOut, code, err := run(gitCurrentBranchArgs, 10*time.Second)
	if err != nil || code != 0 {
		// Not a git repo (or git missing): same "available:false, no error"
		// contract GitStatus uses, so the panel hides the strip silently.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gitRemoteReportJSON(gitRemoteReport{})})
		return
	}
	report.Branch = strings.TrimSpace(branchOut)

	upstreamOut, upstreamCode, _ := run("rev-parse --abbrev-ref --symbolic-full-name @{u}", 10*time.Second)
	report.Upstream = strings.TrimSpace(upstreamOut)
	if c.Query("refresh") == "1" {
		// Fetch refreshes the refs the counts below read. It is best-effort: an
		// offline sandbox keeps the cached state and Fetched stays false.
		// Use an explicit heads refspec so single-branch clones can discover
		// other branches too, including when the current branch has no upstream.
		fetchArgs := "remote | { failed=0; while IFS= read -r remote; do GIT_TERMINAL_PROMPT=0 git -C " + shellQuotePOSIX(workdir) +
			` fetch --quiet --prune -- "$remote" "+refs/heads/*:refs/remotes/$remote/*" || failed=1; done; exit "$failed"; }`
		_, fetchCode, fetchErr := run(fetchArgs, 15*time.Second)
		report.Fetched = fetchErr == nil && fetchCode == 0
	}
	if upstreamCode != 0 || report.Upstream == "" {
		// A branch without an upstream still shows its name; counts stay zero.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gitRemoteReportJSON(report)})
		return
	}

	countsOut, countsCode, _ := run("rev-list --left-right --count HEAD...@{u}", 10*time.Second)
	if countsCode == 0 {
		report.Ahead, report.Behind = parseGitAheadBehind(countsOut)
	}
	if report.Behind > 0 {
		logOut, logCode, _ := run("log --pretty=format:%h%x09%s HEAD..@{u}", 10*time.Second)
		if logCode == 0 {
			report.Incoming = parseGitIncoming(logOut)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gitRemoteReportJSON(report)})
}

// GitPull normally fast-forwards only. Explicit realignment saves local
// committed edits, moves to the fetched upstream and applies the edits back
// with a three-way merge; true conflicts remain visible in the working tree.
func (h *ProgrammingHandler) GitPull(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req struct {
		Repo    string `json:"repo"`
		Realign bool   `json:"realign"`
	}
	// An empty body is valid; a malformed body must be rejected.
	if c.Request.Body != nil {
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			c.Error(apperrors.NewBadRequestError("invalid pull request"))
			return
		}
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
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, req.Repo)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if req.Realign {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": h.realignGitBranch(ctx, sessionID, workdir)})
		return
	}
	stdout, stderr, exitCode, execErr := h.execGit(ctx, sessionID, workdir, "pull --ff-only", 60*time.Second)
	if execErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if exitCode != 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = strings.TrimSpace(stdout)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"pulled": false, "detail": detail,
		}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pulled": true}})
}

// gitIdentityRequest is the body of the identity action: the name and email
// git should commit with, plus the repo whose config they belong to.
type gitIdentityRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Repo  string `json:"repo"`
}

// GitIdentity sets user.name and user.email for one repository — the fix the
// changes panel offers when a commit fails with "Author identity unknown". It
// writes repo-local config (`git config`, not --global) so the identity stays
// inside this session's sandbox and never leaks onto the host or into a shared
// image. Both values go through shellQuotePOSIX, so a name with a quote or a
// newline reaches git verbatim instead of becoming extra shell.
func (h *ProgrammingHandler) GitIdentity(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req gitIdentityRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid identity request"))
		return
	}
	name := strings.TrimSpace(req.Name)
	email := strings.TrimSpace(req.Email)
	if name == "" || email == "" {
		c.Error(apperrors.NewBadRequestError("name and email are required"))
		return
	}
	if len(name) > 200 || len(email) > 200 {
		c.Error(apperrors.NewBadRequestError("name or email is too long"))
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
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, req.Repo)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	for _, args := range []string{
		"config user.name " + shellQuotePOSIX(name),
		"config user.email " + shellQuotePOSIX(email),
	} {
		if _, stderr, code, execErr := h.execGit(ctx, sessionID, workdir, args, 15*time.Second); execErr != nil || code != 0 {
			_ = stderr
			c.Error(apperrors.NewConflictError("git identity could not be saved"))
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"saved": true}})
}

// GitBranches lists local and fetched remote branches for the changes panel's
// branch switcher, marking the current local branch. Read-only.
func (h *ProgrammingHandler) GitBranches(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
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
	branchOut, _, code, err := h.execGit(ctx, sessionID, workdir,
		"for-each-ref --format='%(refname)%09%(symref)' refs/heads refs/remotes", 10*time.Second)
	if err != nil || code != 0 {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	currentOut, _, _, _ := h.execGit(ctx, sessionID, workdir, gitCurrentBranchArgs, 10*time.Second)
	local, remote := parseGitBranchRefs(branchOut)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"branches":        local,
		"remote_branches": remote,
		"current":         strings.TrimSpace(currentOut),
	}})
}

// gitBranchSwitchRequest names the branch to check out and the repo it lives in.
type gitBranchSwitchRequest struct {
	Branch string `json:"branch"`
	Repo   string `json:"repo"`
	Remote bool   `json:"remote"`
}

// GitSwitchBranch switches a local branch or creates a remote tracking branch.
// An existing local branch of the same name is reused without resetting it.
// It refuses when the working tree is dirty — switching with uncommitted
// changes either fails inside git or
// silently carries them onto the new branch, and the panel's whole contract is
// that the user can see what is dirty first, so we make that an explicit
// precondition instead of letting git decide halfway.
func (h *ProgrammingHandler) GitSwitchBranch(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req gitBranchSwitchRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid branch switch request"))
		return
	}
	branch := strings.TrimSpace(req.Branch)
	if !validGitBranchName(branch) {
		c.Error(apperrors.NewBadRequestError("invalid branch name"))
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
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, req.Repo)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	// Dirty check: any uncommitted change (staged or not, tracked or not) blocks
	// the switch so the user commits or discards it deliberately first.
	statusOut, _, code, err := h.execGit(ctx, sessionID, workdir, "status --porcelain=v1 -z", 10*time.Second)
	if err != nil || code != 0 {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if len(parseGitStatusPorcelainZ(statusOut)) > 0 {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"switched": false, "detail": "working tree has uncommitted changes",
		}})
		return
	}
	switchArgs := "switch --no-guess " + shellQuotePOSIX(branch)
	if req.Remote {
		remoteRef := "refs/remotes/" + branch
		_, _, refCode, refErr := h.execGit(ctx, sessionID, workdir, "show-ref --verify --quiet "+shellQuotePOSIX(remoteRef), 10*time.Second)
		if refErr != nil {
			c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
			return
		}
		_, _, aliasCode, aliasErr := h.execGit(ctx, sessionID, workdir, "symbolic-ref --quiet "+shellQuotePOSIX(remoteRef), 10*time.Second)
		if refCode != 0 || aliasErr != nil || aliasCode != 1 {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"switched": false, "detail": "remote branch not found or is a symbolic alias"}})
			return
		}
		remotesOut, _, remoteCode, remoteErr := h.execGit(ctx, sessionID, workdir, "remote", 10*time.Second)
		localName := localGitBranchForRemote(branch, remotesOut)
		if remoteErr != nil || remoteCode != 0 || !validGitBranchName(localName) {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"switched": false, "detail": "remote branch has no configured remote or valid local branch name"}})
			return
		}
		_, _, localCode, localErr := h.execGit(ctx, sessionID, workdir, "show-ref --verify --quiet "+shellQuotePOSIX("refs/heads/"+localName), 10*time.Second)
		if localErr != nil || (localCode != 0 && localCode != 1) {
			c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
			return
		}
		branch = localName
		switchArgs = "switch --no-guess " + shellQuotePOSIX(localName)
		if localCode == 1 {
			// A single-branch clone's persisted fetch refspec may not map this
			// branch. Override it for this invocation so Git can set tracking
			// metadata without changing the user's remote configuration.
			remoteName := strings.TrimSuffix(strings.TrimPrefix(remoteRef, "refs/remotes/"), "/"+localName)
			fetchConfig := "remote." + remoteName + ".fetch=+refs/heads/*:refs/remotes/" + remoteName + "/*"
			switchArgs = "-c " + shellQuotePOSIX(fetchConfig) + " switch --track -c " + shellQuotePOSIX(localName) + " " + shellQuotePOSIX(remoteRef)
		}
	}
	stdout, stderr, exitCode, execErr := h.execGit(ctx, sessionID, workdir, switchArgs, 30*time.Second)
	if execErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if exitCode != 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = strings.TrimSpace(stdout)
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"switched": false, "detail": detail}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"switched": true, "branch": branch}})
}

// gitDiscardRequest names the repo whose uncommitted changes should be dropped.
type gitDiscardRequest struct {
	Repo string `json:"repo"`
}

// GitDiscard drops every uncommitted change in a repository: tracked files are
// restored to HEAD (`git restore --worktree --staged`) and untracked files are
// removed (`git clean -fd`). This is destructive and unrecoverable — the panel
// confirms with the user before calling, and the endpoint runs only these two
// fixed commands against a discovered repo, never a client-supplied path.
func (h *ProgrammingHandler) GitDiscard(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req gitDiscardRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid discard request"))
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
	workdir, err := h.resolveGitRepoWorkdir(ctx, sessionID, layout, store, req.Repo)
	if err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	// An unborn branch has no HEAD to restore. Remove its added files first;
	// clean then removes the remaining untracked files.
	_, stderr, code, execErr := h.execGit(ctx, sessionID, workdir, "rev-parse --verify HEAD", 10*time.Second)
	if execErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	restoreArgs := "restore --worktree --staged ."
	if code != 0 {
		// Only clean an unborn branch whose symbolic target has no ref yet.
		// Other repository errors must not turn into a cleanup operation.
		branchRef, _, branchCode, branchErr := h.execGit(ctx, sessionID, workdir, "symbolic-ref --quiet HEAD", 10*time.Second)
		if branchErr != nil || branchCode != 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"discarded": false, "detail": strings.TrimSpace(stderr)}})
			return
		}
		_, _, refCode, refErr := h.execGit(ctx, sessionID, workdir, "show-ref --verify --quiet "+shellQuotePOSIX(strings.TrimSuffix(branchRef, "\n")), 10*time.Second)
		if refErr != nil || refCode != 1 {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"discarded": false, "detail": strings.TrimSpace(stderr)}})
			return
		}
		restoreArgs = "rm -r -f --ignore-unmatch -- ."
	}
	for _, args := range []string{restoreArgs, "clean -fd"} {
		stdout, stderr, exitCode, execErr := h.execGit(ctx, sessionID, workdir, args, 30*time.Second)
		if execErr != nil {
			c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
			return
		}
		if exitCode != 0 {
			detail := strings.TrimSpace(stderr)
			if detail == "" {
				detail = strings.TrimSpace(stdout)
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"discarded": false, "detail": detail}})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"discarded": true}})
}

// RunCommand powers the first terminal/output panel. It shares the same
// workspace manager and path guard as file access; the underlying sandbox
// backend remains responsible for command policy and OS isolation.
func (h *ProgrammingHandler) RunCommand(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req programmingCommandRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil || strings.TrimSpace(req.Command) == "" {
		c.Error(apperrors.NewBadRequestError("command is required"))
		return
	}
	if len(req.Command) > programmingMaxCommand {
		c.Error(apperrors.NewBadRequestError("command is too long"))
		return
	}
	layout, _, ok := h.resolveWorkspace(c)
	if !ok {
		return
	}
	workDir := layout.Root
	if strings.TrimSpace(req.Path) != "" {
		var err error
		workDir, _, err = resolveWorkspacePath(layout, req.Path)
		if err != nil {
			c.Error(apperrors.NewBadRequestError("working directory must stay inside the workspace"))
			return
		}
	}
	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}
	if timeout > programmingMaxTimeout {
		timeout = programmingMaxTimeout
	}
	result, err := h.workspace.ExecWorkspaceCommand(ctx, programmingSessionID(c), req.Command, workDir, timeout, nil)
	if err != nil {
		c.Error(apperrors.NewConflictError("workspace command is unavailable"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"stdout":      result.Stdout,
		"stderr":      result.Stderr,
		"exit_code":   result.ExitCode,
		"duration_ms": result.Duration.Milliseconds(),
		"killed":      result.Killed,
	}})
}

func (h *ProgrammingHandler) loadOwnedSession(c *gin.Context) (*types.Session, bool) {
	if h == nil || h.sessions == nil {
		c.Error(apperrors.NewInternalServerError("programming service is unavailable"))
		return nil, false
	}
	session, err := h.sessions.GetOwnedSession(c.Request.Context(), programmingSessionID(c))
	if err != nil || session == nil {
		c.Error(apperrors.NewNotFoundError("session not found"))
		return nil, false
	}
	return session, true
}

func (h *ProgrammingHandler) resolveWorkspace(c *gin.Context) (sandbox.WorkspaceLayout, sandbox.SessionFileStore, bool) {
	if h == nil || h.workspace == nil {
		c.Error(apperrors.NewConflictError("programming workspace is unavailable"))
		return sandbox.WorkspaceLayout{}, nil, false
	}
	ctx := c.Request.Context()
	sessionID := programmingSessionID(c)
	layout, err := h.workspace.SessionWorkspaceLayout(ctx, sessionID)
	if err != nil {
		c.Error(apperrors.NewConflictError("workspace is not running"))
		return sandbox.WorkspaceLayout{}, nil, false
	}
	store, err := h.workspace.SessionFileStore(ctx, sessionID)
	if err != nil {
		c.Error(apperrors.NewConflictError("workspace file access is unavailable"))
		return sandbox.WorkspaceLayout{}, nil, false
	}
	return layout, store, true
}

func programmingSessionID(c *gin.Context) string {
	if id := strings.TrimSpace(c.Param("session_id")); id != "" {
		return id
	}
	return strings.TrimSpace(c.Param("id"))
}

func resolveWorkspacePath(layout sandbox.WorkspaceLayout, raw string) (string, string, error) {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" || strings.ContainsRune(raw, '\x00') {
		return "", "", fmt.Errorf("path is required")
	}
	root := path.Clean(layout.Root)
	candidate := raw
	if !strings.HasPrefix(candidate, "/") {
		candidate = path.Join(root, candidate)
	} else {
		candidate = path.Clean(candidate)
	}
	if candidate != root && !strings.HasPrefix(candidate, root+"/") {
		return "", "", fmt.Errorf("path escaped workspace")
	}
	rel := strings.TrimPrefix(candidate, root)
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("invalid workspace path")
	}
	return candidate, rel, nil
}

func relativeWorkspacePath(root, absolute string) (string, error) {
	root = path.Clean(root)
	absolute = path.Clean(strings.ReplaceAll(absolute, "\\", "/"))
	if absolute == root {
		return "", nil
	}
	if !strings.HasPrefix(absolute, root+"/") {
		return "", fmt.Errorf("path outside workspace")
	}
	return strings.TrimPrefix(absolute, root+"/"), nil
}

func readWorkspaceFile(ctx context.Context, store sandbox.SessionFileStore, sessionID, filePath string) ([]byte, *sandbox.RemoteStatEntry, error) {
	stat, err := store.StatSessionFile(ctx, sessionID, filePath)
	if err != nil || stat.Type != sandbox.RemoteEntryFile || stat.Size > programmingMaxFileBytes {
		return nil, nil, fmt.Errorf("file is unavailable")
	}
	content, err := store.ReadSessionFile(ctx, sessionID, filePath)
	if err != nil {
		return nil, nil, err
	}
	return content, stat, nil
}

func filePayload(rel string, content []byte, modified time.Time) programmingFile {
	return programmingFile{
		Path: rel, Content: string(content), Hash: contentHash(content), Size: len(content), Modified: modified,
	}
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
