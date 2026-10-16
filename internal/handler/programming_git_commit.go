package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode"

	"github.com/Tencent/WeKnora/internal/types"
)

// Local commits need author metadata, not remote credentials. Preserve existing
// Git config; fill missing fields from the authenticated WeKnora user only.
func (h *ProgrammingHandler) ensureLocalGitAuthor(ctx context.Context, sessionID, workdir string) error {
	name, email := "WeKnora", "weknora@users.weknora.local"
	if user, ok := ctx.Value(types.UserContextKey).(*types.User); ok && user != nil {
		cleanName := strings.TrimSpace(strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || r == '<' || r == '>' {
				return -1
			}
			return r
		}, user.Username))
		if cleanName != "" {
			name = cleanName
		}
		if address, err := mail.ParseAddress(user.Email); err == nil && address.Address == user.Email {
			email = user.Email
		} else {
			email = fmt.Sprintf("user-%x@users.weknora.local", sha256.Sum256([]byte(user.ID)))
		}
	}
	for _, field := range []struct{ key, value string }{{"user.name", name}, {"user.email", email}} {
		out, _, code, err := h.execGit(ctx, sessionID, workdir, "config --get "+field.key, 10*time.Second)
		if err != nil || (code != 0 && code != 1) {
			return fmt.Errorf("git author configuration could not be read")
		}
		if code == 0 && strings.TrimSpace(out) != "" {
			continue
		}
		_, _, code, err = h.execGit(ctx, sessionID, workdir, "config --local "+field.key+" "+shellQuotePOSIX(field.value), 10*time.Second)
		if err != nil || code != 0 {
			return fmt.Errorf("git author configuration could not be saved")
		}
	}
	return nil
}

// gitCommitMaxMessage bounds the commit message a user can send. Multi-line
// messages are fine (the subject/body convention is the user's choice), but a
// runaway paste should be rejected before it reaches the shell.
const gitCommitMaxMessage = 4 * 1024

// shellQuotePOSIX wraps s in POSIX single quotes so the shell passes it
// through as one literal argument. Embedded single quotes become the classic
// '\” sequence (close quote, escaped quote, reopen quote), which is safe for
// any byte the message may carry — including newlines and dollar signs.
func shellQuotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// buildGitCommitCommands renders the fixed command pair behind the manual
// commit action: stage everything, then commit with the user's message as the
// only variable part. Keeping both commands server-side — instead of routing
// through the free-form RunCommand — means the commit surface can never be
// coerced into running anything but these two git invocations.
func buildGitCommitCommands(message string) (add, commit string) {
	return "git add -A", "git commit -m " + shellQuotePOSIX(message)
}

// Resolve selections against Git's own dirty-file list. Renames include both
// sides so selecting the destination also commits deletion of the old path.
// Copy sources are deliberately excluded: they may have unrelated changes.
func selectedGitCommitPaths(requested []string, statusRaw string) ([]string, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("select at least one changed file to commit")
	}
	records := strings.Split(statusRaw, "\x00")
	dirty := make(map[string][]string)
	for i := 0; i < len(records); i++ {
		record := records[i]
		if len(record) < 4 {
			continue
		}
		xy, name := record[:2], record[3:]
		paths := []string{name}
		if strings.ContainsAny(xy, "RC") && i+1 < len(records) {
			i++
			if strings.Contains(xy, "R") && records[i] != "" {
				paths = append(paths, records[i])
			}
		}
		dirty[name] = paths
	}
	paths := make([]string, 0, len(requested))
	seen := make(map[string]bool)
	requestedSet := make(map[string]bool, len(requested))
	for _, name := range requested {
		requestedSet[name] = true
	}
	for _, name := range requested {
		matched, ok := dirty[name]
		if !ok {
			return nil, fmt.Errorf("selected file is no longer in the repository's change list")
		}
		if len(matched) > 1 {
			if _, recreated := dirty[matched[1]]; recreated && !requestedSet[matched[1]] {
				return nil, fmt.Errorf("rename source has separate changes; select that path too before committing")
			}
		}
		for _, p := range matched {
			if !seen[p] {
				paths = append(paths, p)
				seen[p] = true
			}
		}
	}
	if len(quotedGitCommitPaths(paths)) > 24*1024 {
		return nil, fmt.Errorf("selected paths are too long; commit fewer files at a time")
	}
	return paths, nil
}

func quotedGitCommitPaths(paths []string) string {
	quoted := make([]string, len(paths))
	for i, p := range paths {
		quoted[i] = shellQuotePOSIX(p)
	}
	return strings.Join(quoted, " ")
}

func buildSelectedGitCommitCommands(message string, paths []string) (add, commit string) {
	pathspecs := quotedGitCommitPaths(paths)
	return "git --literal-pathspecs add -A -- " + pathspecs,
		"git --literal-pathspecs commit --only -m " + shellQuotePOSIX(message) + " -- " + pathspecs
}
