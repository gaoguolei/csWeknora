package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/gin-gonic/gin"
)

type gitHistoryEntry struct {
	SHA         string   `json:"sha"`
	ShortSHA    string   `json:"short_sha"`
	Author      string   `json:"author"`
	CommittedAt string   `json:"committed_at"`
	Subject     string   `json:"subject"`
	Body        string   `json:"body"`
	Parents     []string `json:"parents"`
	Refs        string   `json:"refs"`
	Status      string   `json:"status"`
	CanReset    bool     `json:"can_reset"`
}

func validGitHistorySHA(value string) bool {
	return (len(value) == 40 || len(value) == 64) && strings.Trim(value, "0123456789abcdefABCDEF") == ""
}

// Git uses NUL between fields and records, so tabs/newlines in commit messages
// cannot corrupt the list. Eight fields form each complete record.
func parseGitHistory(raw string) []gitHistoryEntry {
	fields := strings.Split(raw, "\x00")
	entries := make([]gitHistoryEntry, 0)
	for i := 0; i+7 < len(fields); i += 8 {
		if !validGitHistorySHA(fields[i]) {
			continue
		}
		entries = append(entries, gitHistoryEntry{
			SHA: fields[i], ShortSHA: fields[i+1], Author: fields[i+2], CommittedAt: fields[i+3],
			Subject: fields[i+4], Body: fields[i+5], Parents: strings.Fields(fields[i+6]), Refs: fields[i+7], Status: "unknown",
		})
	}
	return entries
}

// GitHistory reads commits reachable from the current branch and its upstream.
// Full object IDs anchor pagination so new commits cannot shift the next page.
func (h *ProgrammingHandler) GitHistory(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	limit, skip := 30, 0
	for name, target := range map[string]*int{"limit": &limit, "skip": &skip} {
		if value := c.Query(name); value != "" {
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || (name == "limit" && (parsed < 1 || parsed > 100)) {
				c.Error(apperrors.NewBadRequestError("invalid history pagination"))
				return
			}
			*target = parsed
		}
	}
	head, upstream := c.Query("head"), c.Query("upstream")
	if (head != "" && !validGitHistorySHA(head)) || (upstream != "" && !validGitHistorySHA(upstream)) {
		c.Error(apperrors.NewBadRequestError("invalid history snapshot"))
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
	if head == "" {
		out, _, code, execErr := h.execGit(ctx, sessionID, workdir, "rev-parse --verify HEAD", 10*time.Second)
		if execErr != nil {
			c.Error(apperrors.NewConflictError("git history is unavailable for this workspace"))
			return
		}
		if code != 0 {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"commits": []gitHistoryEntry{}, "head": "", "upstream": "", "has_more": false}})
			return
		}
		head = strings.TrimSpace(out)
		upstreamOut, _, upstreamCode, upstreamErr := h.execGit(ctx, sessionID, workdir, "rev-parse --verify '@{u}'", 10*time.Second)
		if upstreamErr == nil && upstreamCode == 0 {
			upstream = strings.TrimSpace(upstreamOut)
		} else {
			upstream = ""
		}
	}
	if !validGitHistorySHA(head) || (upstream != "" && !validGitHistorySHA(upstream)) {
		c.Error(apperrors.NewConflictError("git history snapshot is unavailable"))
		return
	}
	revisions := shellQuotePOSIX(head)
	if upstream != "" && upstream != head {
		revisions += " " + shellQuotePOSIX(upstream)
	}
	pageArgs := fmt.Sprintf("--topo-order --max-count=%d --skip=%d %s --", limit+1, skip, revisions)
	raw, _, code, execErr := h.execGit(ctx, sessionID, workdir,
		"log -z --format='%H%x00%h%x00%an%x00%cI%x00%s%x00%b%x00%P%x00%D' "+pageArgs, 20*time.Second)
	if execErr != nil || code != 0 {
		c.Error(apperrors.NewConflictError("git history could not be read"))
		return
	}
	entries := parseGitHistory(raw)
	hasMore := len(entries) > limit
	if hasMore {
		entries = entries[:limit]
	}
	// Eligibility is calculated from this branch's actual ancestry. History
	// includes incoming commits too; matching the upstream SHA alone does not
	// make one a valid reset target. A stale page or detached HEAD has no actions.
	git := "git -C " + shellQuotePOSIX(workdir)
	liveOut, _, liveCode, liveErr := h.execGit(ctx, sessionID, workdir,
		"symbolic-ref --quiet HEAD && "+git+" rev-parse --verify HEAD", 10*time.Second)
	live := strings.Fields(liveOut)
	resetBranch := liveErr == nil && liveCode == 0 && len(live) == 2 && live[1] == head && strings.HasPrefix(live[0], "refs/heads/")
	if len(entries) > 0 {
		statusArgs := "log --format=%H " + pageArgs + " | while IFS= read -r sha; do local_code=0; remote_code=1; " +
			git + " merge-base --is-ancestor \"$sha\" " + shellQuotePOSIX(head) + " || local_code=$?; "
		if upstream != "" {
			statusArgs += "remote_code=0; " + git + " merge-base --is-ancestor \"$sha\" " + shellQuotePOSIX(upstream) + " || remote_code=$?; " +
				`status=unknown; if [ "$local_code" = 0 ] && [ "$remote_code" = 0 ]; then status=synced; ` +
				`elif [ "$local_code" = 0 ] && [ "$remote_code" = 1 ]; then status=outgoing; ` +
				`elif [ "$local_code" = 1 ] && [ "$remote_code" = 0 ]; then status=incoming; fi; `
		} else {
			statusArgs += "status=unknown; "
		}
		statusArgs += "can_reset=false; "
		if resetBranch {
			statusArgs += `if [ "$local_code" = 0 ] && [ "$sha" != ` + shellQuotePOSIX(head) + ` ]; then `
			if upstream == "" {
				statusArgs += "can_reset=true; "
			} else {
				statusArgs += "dropped=$(" + git + " rev-list --count " + shellQuotePOSIX(head) + " ^\"$sha\") && " +
					"local_only=$(" + git + " rev-list --count " + shellQuotePOSIX(head) + " ^\"$sha\" ^" + shellQuotePOSIX(upstream) + ") && " +
					`[ "$dropped" = "$local_only" ] && can_reset=true; `
			}
			statusArgs += "fi; "
		}
		statusArgs += `printf '%s\t%s\t%s\n' "$sha" "$status" "$can_reset"; done`
		statusOut, _, statusCode, statusErr := h.execGit(ctx, sessionID, workdir, statusArgs, 30*time.Second)
		if statusErr == nil && statusCode == 0 {
			type eligibility struct {
				status string
				reset  bool
			}
			bySHA := make(map[string]eligibility)
			for _, line := range strings.Split(statusOut, "\n") {
				fields := strings.Split(line, "\t")
				if len(fields) == 3 {
					bySHA[fields[0]] = eligibility{fields[1], fields[2] == "true"}
				}
			}
			for i := range entries {
				if data, ok := bySHA[entries[i].SHA]; ok {
					entries[i].Status, entries[i].CanReset = data.status, data.reset
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"commits": entries, "head": head, "upstream": upstream, "has_more": hasMore}})
}
