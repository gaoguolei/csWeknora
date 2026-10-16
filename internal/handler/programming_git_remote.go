package handler

import (
	"strconv"
	"strings"
)

// Read cached refs in the same sandbox invocation as file status. Initial
// rendering must not depend on a second request or an external fetch.
const gitCachedRemoteCommandArgs = "branch=$(git " + gitCurrentBranchArgs + " 2>/dev/null) || branch=; " +
	"upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null) || upstream=; " +
	"counts=$(git rev-list --left-right --count HEAD...'@{u}' 2>/dev/null) || counts='0 0'; " +
	"printf '%s\\n%s\\n%s\\n' \"$branch\" \"$upstream\" \"$counts\""

func parseGitCachedRemote(raw string) gitRemoteReport {
	// A preceding -z record can leave one extra NUL at the block boundary.
	lines := strings.Split(strings.TrimLeft(raw, "\x00"), "\n")
	if len(lines) < 3 || lines[0] == "" {
		return gitRemoteReport{}
	}
	ahead, behind := parseGitAheadBehind(lines[2])
	return gitRemoteReport{Available: true, Branch: lines[0], Upstream: lines[1], Ahead: ahead, Behind: behind}
}

// gitIncomingEntry is one commit the local branch is behind its upstream:
// what `git log HEAD..@{u}` reports, trimmed to what the panel renders.
type gitIncomingEntry struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// gitRemoteReport is the JSON for the changes panel's remote strip. Upstream
// is empty on a branch that never set one; Ahead/Behind count the two sides of
// HEAD...@{u}. Fetched records whether this read refreshed the refs from the
// remote — a sandbox without network still shows the last known state.
type gitRemoteReport struct {
	Available bool               `json:"available"`
	Branch    string             `json:"branch"`
	Upstream  string             `json:"upstream"`
	Ahead     int                `json:"ahead"`
	Behind    int                `json:"behind"`
	Fetched   bool               `json:"fetched"`
	Incoming  []gitIncomingEntry `json:"incoming"`
}

// parseGitAheadBehind decodes `git rev-list --left-right --count HEAD...@{u}`:
// "<ahead>\t<behind>". A malformed line counts as zero on both sides rather
// than failing the whole report.
func parseGitAheadBehind(raw string) (ahead, behind int) {
	fields := strings.Fields(raw)
	if len(fields) != 2 {
		return 0, 0
	}
	ahead, _ = strconv.Atoi(fields[0])
	behind, _ = strconv.Atoi(fields[1])
	return ahead, behind
}

// parseGitIncoming decodes `git log --pretty=format:%h%x09%s HEAD..@{u}` — one
// "<sha>\t<subject>" per line. The tab keeps subjects with spaces intact.
func parseGitIncoming(raw string) []gitIncomingEntry {
	out := make([]gitIncomingEntry, 0)
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimRight(line, "\r"); line == "" {
			continue
		}
		sha, subject, _ := strings.Cut(line, "\t")
		out = append(out, gitIncomingEntry{SHA: sha, Subject: subject})
	}
	return out
}

// gitRemoteReportJSON keeps the gin shape at the call site without leaking the
// parser structs into the handler, mirroring gitStatusReportJSON.
func gitRemoteReportJSON(r gitRemoteReport) map[string]any {
	incoming := make([]map[string]any, 0, len(r.Incoming))
	for _, e := range r.Incoming {
		incoming = append(incoming, map[string]any{"sha": e.SHA, "subject": e.Subject})
	}
	return map[string]any{
		"available": r.Available,
		"branch":    r.Branch,
		"upstream":  r.Upstream,
		"ahead":     r.Ahead,
		"behind":    r.Behind,
		"fetched":   r.Fetched,
		"incoming":  incoming,
	}
}
