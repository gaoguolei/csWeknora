package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/gin-gonic/gin"
)

// gitResetRequest names the commit the current branch should be moved back to.
// Repo is the workspace-relative repository root; empty means the workspace root.
type gitResetRequest struct {
	Repo   string `json:"repo"`
	Commit string `json:"commit"`
}

// GitReset moves the current branch back to a local commit with
// `git reset --mixed`: the commits above the target leave the branch, while
// their changes stay in the working tree as uncommitted edits, so nothing the
// user wrote is lost. Published history is never rewritten: no dropped commit
// may be reachable from the upstream, and the working tree must already
// be clean, so the restored edits are exactly what the dropped commits held.
func (h *ProgrammingHandler) GitReset(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req gitResetRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("invalid reset request"))
		return
	}
	target := strings.TrimSpace(req.Commit)
	if !validGitHistorySHA(target) {
		c.Error(apperrors.NewBadRequestError("invalid reset commit"))
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
	refuse := func(reason, detail string) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"reset": false, "reason": reason, "detail": detail}})
	}
	// A detached HEAD has no branch to move, and no upstream to protect.
	if _, _, refCode, refErr := h.execGit(ctx, sessionID, workdir, "symbolic-ref --quiet HEAD", 10*time.Second); refErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	} else if refCode != 0 {
		refuse("not_on_branch", "check out a local branch before reverting")
		return
	}
	headOut, _, headCode, headErr := h.execGit(ctx, sessionID, workdir, "rev-parse --verify HEAD", 10*time.Second)
	if headErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	head := strings.TrimSpace(headOut)
	if headCode != 0 || !validGitHistorySHA(head) {
		refuse("not_on_branch", "this repository has no commit to move back from")
		return
	}
	if head == target {
		refuse("already_at_commit", "the branch is already at this commit")
		return
	}
	// Only a commit on the current branch can be a target.
	_, _, ancestorCode, ancestorErr := h.execGit(ctx, sessionID, workdir,
		"merge-base --is-ancestor "+shellQuotePOSIX(target)+" "+shellQuotePOSIX(head), 10*time.Second)
	if ancestorErr != nil || ancestorCode > 1 {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if ancestorCode != 0 {
		refuse("not_on_branch", "this commit is not on the current branch")
		return
	}
	// Compare the commits this reset drops with the subset absent from the
	// upstream. This also permits a shared ancestor of a diverged branch when
	// every dropped commit is local; requiring upstream to precede the target
	// incorrectly refused all such targets.
	upstreamOut, _, upstreamCode, upstreamErr := h.execGit(ctx, sessionID, workdir,
		"rev-parse --verify --quiet '@{u}'", 10*time.Second)
	if upstreamErr != nil || (upstreamCode != 0 && upstreamCode != 1 && upstreamCode != 128) {
		c.Error(apperrors.NewConflictError("git upstream could not be read"))
		return
	}
	upstream := strings.TrimSpace(upstreamOut)
	if upstreamCode == 0 && validGitHistorySHA(upstream) {
		countsOut, _, countsCode, countsErr := h.execGit(ctx, sessionID, workdir,
			"rev-list --count "+shellQuotePOSIX(head)+" ^"+shellQuotePOSIX(target)+
				" && git -C "+shellQuotePOSIX(workdir)+" rev-list --count "+shellQuotePOSIX(head)+
				" ^"+shellQuotePOSIX(target)+" ^"+shellQuotePOSIX(upstream), 10*time.Second)
		counts := strings.Fields(countsOut)
		if countsErr != nil || countsCode != 0 || len(counts) != 2 {
			c.Error(apperrors.NewConflictError("published git history could not be checked"))
			return
		}
		dropped, droppedErr := strconv.Atoi(counts[0])
		local, localErr := strconv.Atoi(counts[1])
		if droppedErr != nil || localErr != nil || dropped != local {
			refuse("published_history", "this reset would remove published commits; only unpushed commits can be reverted")
			return
		}
	}
	// A clean tree keeps the restored edits identical to what the dropped
	// commits contained, instead of mixing them with older uncommitted work.
	statusOut, _, statusCode, statusErr := h.execGit(ctx, sessionID, workdir, "status --porcelain=v1 -z", 10*time.Second)
	if statusErr != nil || statusCode != 0 {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if len(parseGitStatusPorcelainZ(statusOut)) > 0 {
		refuse("dirty_tree", "commit or discard your changes before reverting")
		return
	}
	stdout, stderr, resetCode, resetErr := h.execGit(ctx, sessionID, workdir,
		"reset --mixed "+shellQuotePOSIX(target), 30*time.Second)
	if resetErr != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	if resetCode != 0 {
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = strings.TrimSpace(stdout)
		}
		refuse("git_error", detail)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"reset": true}})
}
