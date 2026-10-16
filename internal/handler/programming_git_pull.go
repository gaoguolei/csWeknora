package handler

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Realign a clean branch using a three-way stash application. Resetting to the
// common ancestor and pulling with the edits still present rejects even
// non-overlapping edits to the same file. Save those edits first instead.
func (h *ProgrammingHandler) realignGitBranch(ctx context.Context, sessionID, workdir string) map[string]any {
	result := map[string]any{"pulled": false}
	fail := func(detail string) map[string]any {
		result["detail"] = detail
		return result
	}
	run := func(args string, timeout time.Duration) (string, bool) {
		out, _, code, err := h.execGit(ctx, sessionID, workdir, args, timeout)
		return strings.TrimSpace(out), err == nil && code == 0
	}
	if busy, err := h.workspace.HasActiveTurn(ctx, sessionID); err != nil || busy {
		return fail("请等待智能体完成当前操作，再对齐远程。")
	}
	branch, branchOK := run("symbolic-ref --quiet HEAD", 10*time.Second)
	if !branchOK || !strings.HasPrefix(branch, "refs/heads/") {
		return fail("请先切换到本地分支，再对齐远程。")
	}
	status, ok := run("status --porcelain=v1 -z --untracked-files=all", 10*time.Second)
	if !ok {
		return fail("无法读取当前改动，尚未对齐远程。")
	}
	if status != "" {
		return fail("请先提交或丢弃未提交的改动，再对齐远程；已有改动未被修改。")
	}
	// Fetch first: a network/credential failure must not rewind local commits.
	fetch, fetchErr := h.workspace.ExecWorkspaceCommand(ctx, sessionID,
		gitEnvQuietPrefix+"GIT_TERMINAL_PROMPT=0 git -C "+shellQuotePOSIX(workdir)+" -c core.askPass= fetch --no-tags", workdir, 60*time.Second, nil)
	if fetchErr != nil || fetch == nil || fetch.ExitCode != 0 {
		return fail("获取远程更新失败，请检查网络和仓库凭据；本地提交未被修改。")
	}
	// A fetch may take time; recheck before making any local change.
	currentBranch, currentOK := run("symbolic-ref --quiet HEAD", 10*time.Second)
	status, ok = run("status --porcelain=v1 -z --untracked-files=all", 10*time.Second)
	busy, busyErr := h.workspace.HasActiveTurn(ctx, sessionID)
	if !currentOK || currentBranch != branch || !ok || status != "" || busyErr != nil || busy {
		return fail("获取远程期间分支或工作区状态发生变化，请等待操作完成后重试；本地提交未被修改。")
	}
	head, headOK := run("rev-parse --verify HEAD", 10*time.Second)
	upstream, upstreamOK := run("rev-parse --verify --quiet '@{u}'", 10*time.Second)
	if !headOK || !upstreamOK || !validGitHistorySHA(head) || !validGitHistorySHA(upstream) {
		return fail("请先配置可用的上游分支，再对齐远程。")
	}
	base, baseOK := run("merge-base "+shellQuotePOSIX(head)+" "+shellQuotePOSIX(upstream), 10*time.Second)
	if !baseOK || !validGitHistorySHA(base) {
		return fail("当前分支与上游没有共同祖先，请检查仓库和上游分支配置。")
	}
	if head == upstream {
		result["pulled"] = true
		return result
	}
	if head == base {
		if _, ok := run("merge --ff-only "+shellQuotePOSIX(upstream), 30*time.Second); !ok {
			return fail("无法快进到远程提交，本地改动未被丢弃。")
		}
		result["pulled"] = true
		return result
	}
	if err := h.ensureLocalGitAuthor(ctx, sessionID, workdir); err != nil {
		return fail(err.Error())
	}
	// Retain both the original commit history and the saved edit snapshot. Never
	// clear the user's stash stack, or drop a potentially concurrent stash.
	backup := fmt.Sprintf("refs/weknora/backups/realign-%d", time.Now().UnixNano())
	if _, ok := run("update-ref "+shellQuotePOSIX(backup)+" "+shellQuotePOSIX(head), 10*time.Second); !ok {
		return fail("无法创建本地提交备份，尚未对齐远程。")
	}
	result["backup_ref"] = backup
	previousStash, _ := run("rev-parse --verify --quiet refs/stash", 10*time.Second)
	savedStash := ""
	rollback := func(detail string) map[string]any {
		restored := true
		if savedStash != "" {
			_, restored = run("stash apply "+shellQuotePOSIX(savedStash), 30*time.Second)
		}
		if restored {
			_, restored = run("reset --mixed "+shellQuotePOSIX(head), 30*time.Second)
		}
		result["rollback_failed"] = !restored
		if !restored {
			detail += " 自动恢复未完成，原提交和改动备份已保留，请勿丢弃当前改动。"
		}
		return fail(detail)
	}
	if _, ok := run("reset --mixed "+shellQuotePOSIX(base), 30*time.Second); !ok {
		return fail("无法准备对齐操作，原提交备份已保留。")
	}
	delta, ok := run("status --porcelain=v1 -z --untracked-files=all", 10*time.Second)
	if !ok {
		return rollback("无法读取本地提交的改动，已尝试恢复原分支。")
	}
	if delta != "" {
		// Stage additions so add/add conflicts are merged as tracked changes;
		// leaving them untracked would make stash apply refuse existing files.
		if _, ok := run("add -A", 30*time.Second); !ok {
			return rollback("无法保存本地改动，已尝试恢复原分支。")
		}
		_, stashOK := run("stash push --include-untracked -m "+shellQuotePOSIX("WeKnora realign backup "+head), 30*time.Second)
		stash, shaOK := run("rev-parse --verify --quiet refs/stash", 10*time.Second)
		if shaOK && validGitHistorySHA(stash) && stash != previousStash {
			savedStash = stash
			result["stash_ref"] = stash
		}
		if !stashOK || savedStash == "" {
			return rollback("本地改动备份未完成，已尝试恢复原分支。")
		}
	}
	if _, ok := run("merge --ff-only "+shellQuotePOSIX(upstream), 30*time.Second); !ok {
		return rollback("对齐远程失败，已尝试恢复原分支。")
	}
	result["pulled"], result["aligned"] = true, true
	if savedStash != "" {
		if _, ok := run("stash apply "+shellQuotePOSIX(savedStash), 30*time.Second); !ok {
			unmerged, statusOK := run("ls-files --unmerged -z", 10*time.Second)
			if statusOK && unmerged != "" {
				result["conflicts"] = true
				return result
			}
			result["restore_failed"] = true
			return fail("远程已对齐，但本地改动未能完整恢复。原提交和改动备份已保留，请先恢复备份再继续。")
		}
	}
	return result
}
