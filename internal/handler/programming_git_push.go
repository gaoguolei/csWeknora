package handler

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/gin-gonic/gin"
)

var gitPushSSHPattern = regexp.MustCompile(`^(?:[A-Za-z0-9._-]+@)?(?:\[[0-9a-fA-F:]+\]|[A-Za-z0-9._-]+):[^\s]+$`)

var gitPushURLPattern = regexp.MustCompile(`https?://[^\s'"<>]+`)

// A read-only helper supplies the dialog credentials through Git's standard
// credential protocol. Store/erase operations are ignored, so nothing is saved.
// The script contains variable names only; secrets are execution environment
// values and never appear in command arguments or repository configuration.
const gitPushCredentialHelper = `!f() {
    if [ "$1" != get ]; then return 0; fi
    protocol= host=
    while IFS= read -r line; do
        [ -n "$line" ] || break
        case "$line" in
            protocol=*) protocol=${line#protocol=} ;;
            host=*) host=${line#host=} ;;
        esac
    done
    if [ "$protocol" = "$WEKNORA_GIT_PROTOCOL" ] && [ "$host" = "$WEKNORA_GIT_HOST" ]; then
        printf 'username=%s\npassword=%s\n' "$WEKNORA_GIT_USERNAME" "$WEKNORA_GIT_PASSWORD"
    fi
}; f`

// Strip transport tracing from user-facing errors while preserving Git's
// fatal/error/remote output. Detect status codes before filtering these lines.
var gitPushTraceLinePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}\.\d+\s+http\.c:\d+\s+(?:==|=>|<=) `)

// Git colours remote errors with ANSI escapes; the panel renders plain text, so
// the escapes survive as literal "[31m" noise.
var gitPushANSIPattern = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")

var gitPushHTTPAuthPattern = regexp.MustCompile(`(?i)HTTP/\S+\s+(?:401|403)\b`)

func gitPushFailureDetail(raw string) string {
	raw = gitPushANSIPattern.ReplaceAllString(raw, "")
	lines := make([]string, 0)
	for _, line := range strings.Split(raw, "\n") {
		if !gitPushTraceLinePattern.MatchString(line) {
			lines = append(lines, line)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// gitPushAuthSummary reduces a refused-credential transcript to the server's own
// reason. The full output is a multi-line log (remote prefix, session id, a
// fatal line repeating the URL) while the dialog already states that
// authentication failed, so only the line explaining why is worth showing.
func gitPushAuthSummary(detail string) string {
	fatal := ""
	for _, line := range strings.Split(gitPushANSIPattern.ReplaceAllString(detail, ""), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "fatal:") {
			// The fatal line only names the URL, so keep it as a fallback.
			fatal = line
			continue
		}
		for _, marker := range []string{"incorrect", "invalid", "denied", "unauthorized", "not authorized", "permission"} {
			if strings.Contains(lower, marker) {
				return line
			}
		}
	}
	return fatal
}

// gitPushAuthFailed reports whether the push failed on credentials: either the
// server rejected the supplied ones, or Git could not obtain new ones and
// stopped asking. The second form is what an HTTP 401 looks like once the
// credential is refused and no prompt is allowed, so it is an authentication
// failure too — reporting it as a generic refusal would hide the real cause.
func gitPushAuthFailed(detail string) bool {
	lower := strings.ToLower(detail)
	for _, marker := range []string{
		"authentication failed",
		"access denied",
		"permission denied",
		"invalid username or password",
		"incorrect username or password",
		"could not read username",
		"could not read password",
		"terminal prompts disabled",
		"unable to read askpass response",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return gitPushHTTPAuthPattern.MatchString(detail)
}

// GitPush publishes only the current branch of an owned session repository.
// It never forces an update, sends tags, or pushes other local branches.
func (h *ProgrammingHandler) GitPush(c *gin.Context) {
	ctx := c.Request.Context()
	if _, ok := h.loadOwnedSession(c); !ok {
		return
	}
	var req struct {
		Repo      string `json:"repo"`
		Username  string `json:"username"`
		Password  string `json:"password"`
		RemoteURL string `json:"remote_url"`
		UseSSH    bool   `json:"use_ssh"`
	}
	if c.Request.Body != nil {
		if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			c.Error(apperrors.NewBadRequestError("invalid push request"))
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
	refOut, _, code, err := h.execGit(ctx, sessionID, workdir, "symbolic-ref --quiet HEAD", 10*time.Second)
	if err != nil {
		c.Error(apperrors.NewConflictError("git is unavailable for this workspace"))
		return
	}
	localRef := strings.TrimSpace(refOut)
	branch, localBranch := strings.CutPrefix(localRef, "refs/heads/")
	if code != 0 || !localBranch || !validGitBranchName(branch) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pushed": false, "detail": "check out a local branch before pushing"}})
		return
	}
	upstreamOut, _, code, err := h.execGit(ctx, sessionID, workdir,
		"for-each-ref --format='%(upstream:remotename)%09%(upstream:remoteref)' "+shellQuotePOSIX(localRef), 10*time.Second)
	if err != nil || code != 0 {
		c.Error(apperrors.NewConflictError("git upstream could not be read"))
		return
	}
	remote, destination, _ := strings.Cut(strings.TrimSuffix(upstreamOut, "\n"), "\t")
	setUpstream := remote == "" && destination == ""
	if setUpstream {
		remotesOut, _, remoteCode, remoteErr := h.execGit(ctx, sessionID, workdir, "remote", 10*time.Second)
		if remoteErr != nil || remoteCode != 0 {
			c.Error(apperrors.NewConflictError("git remotes could not be read"))
			return
		}
		remotes := strings.Fields(remotesOut)
		for _, name := range remotes {
			if name == "origin" {
				remote = name
				break
			}
		}
		if remote == "" && len(remotes) == 1 {
			remote = remotes[0]
		}
		if remote == "" {
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pushed": false, "detail": "configure an upstream branch or an origin remote before pushing"}})
			return
		}
		destination = "refs/heads/" + branch
	}
	if remote == "." || remote == "" || !strings.HasPrefix(destination, "refs/heads/") {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pushed": false, "detail": "the upstream must point to a branch on a configured remote"}})
		return
	}
	// Resolve the effective push URL, including insteadOf/pushInsteadOf.
	// Reject multiple destinations: one dialog authorizes one server only.
	urlOut, _, code, err := h.execGit(ctx, sessionID, workdir,
		"remote get-url --push --all -- "+shellQuotePOSIX(remote), 10*time.Second)
	urls := strings.Fields(urlOut)
	if err != nil || code != 0 || len(urls) != 1 {
		c.Error(apperrors.NewConflictError("configure exactly one push URL for this remote"))
		return
	}
	target := urls[0]
	parsed, parseErr := url.Parse(target)
	mode := ""
	if parseErr == nil && (parsed.Scheme == "https" || parsed.Scheme == "http") && parsed.Host != "" {
		// Embedded credentials must never bypass the credentials in the dialog.
		parsed.User, parsed.RawQuery, parsed.Fragment = nil, "", ""
		target = parsed.String()
		mode = "http"
	} else if (parseErr == nil && parsed.Scheme == "ssh" && parsed.Host != "") || gitPushSSHPattern.MatchString(target) {
		mode = "ssh"
		if parseErr == nil && parsed.Scheme == "ssh" {
			if parsed.User != nil {
				parsed.User = url.User(parsed.User.Username())
			}
			parsed.RawQuery, parsed.Fragment = "", ""
			target = parsed.String()
		}
	} else {
		c.Error(apperrors.NewBadRequestError("push authentication requires an HTTP(S) or SSH remote"))
		return
	}
	username := strings.TrimSpace(req.Username)
	suppliedCredentials := username != "" || req.Password != ""
	usingSavedCredentials := false
	askForAuth := func(expired bool, detail string) {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
			"pushed": false, "auth_required": true, "auth_failed": expired,
			"auth_mode": mode, "remote_url": target, "username": username, "detail": detail,
		}})
	}
	if (req.RemoteURL != "" || suppliedCredentials || req.UseSSH) && req.RemoteURL != target {
		askForAuth(false, "the remote URL changed before authentication")
		return
	}
	if mode == "http" && !suppliedCredentials {
		savedName, savedPassword, saved := h.readSandboxGitCredential(ctx, sessionID, workdir, target)
		if !saved {
			askForAuth(false, "")
			return
		}
		username, req.Password = savedName, savedPassword
		usingSavedCredentials = true
	} else if mode == "ssh" && !req.UseSSH {
		askForAuth(false, "")
		return
	}

	env := map[string]string{
		// Prompts are disabled so a push can never block waiting for input on a
		// terminal nobody is watching. Askpass is deliberately left unset: a
		// program that fails to answer only replaces Git's own diagnosis of the
		// server's rejection with its own error. Credentials come from the
		// dialog or this session's private store instead.
		"GIT_TERMINAL_PROMPT": "0",
		// Belt and braces: if some other variable still turns tracing on, keep
		// credential material out of the output.
		"GIT_TRACE_REDACT":     "1",
		"WEKNORA_GIT_PUSH_URL": target,
	}
	// Push to the resolved URL directly. Appending a remote.pushurl override
	// would retain old URL values and could publish to multiple destinations.
	options := "-c push.followTags=false"
	if mode == "http" {
		if username == "" || req.Password == "" || len(username) > 200 || len(req.Password) > 8192 ||
			strings.ContainsAny(username, ":\r\n\x00") || strings.ContainsAny(req.Password, "\r\n\x00") {
			c.Error(apperrors.NewBadRequestError("remote username and password or access token are required"))
			return
		}
		env["WEKNORA_GIT_USERNAME"] = username
		env["WEKNORA_GIT_PASSWORD"] = req.Password
		env["WEKNORA_GIT_PROTOCOL"] = parsed.Scheme
		env["WEKNORA_GIT_HOST"] = strings.ToLower(parsed.Host)
		// A username-only URL lets Git fill the password from our helper
		// before the first request. Unlike a manual Authorization header,
		// Git can correctly handle the server's challenge and rejection.
		executionURL := *parsed
		executionURL.Host = strings.ToLower(executionURL.Host)
		executionURL.User = url.User(username)
		env["WEKNORA_GIT_PUSH_URL"] = executionURL.String()
		// Clear all inherited helpers, headers and cookies for this invocation.
		options += " -c credential.helper= -c " + shellQuotePOSIX("credential.helper="+gitPushCredentialHelper) +
			" -c http.extraHeader= -c http.cookieFile= -c http.saveCookies=false" +
			" -c " + shellQuotePOSIX("http."+executionURL.String()+".extraHeader=") +
			" -c " + shellQuotePOSIX("http."+executionURL.String()+".cookieFile=") +
			" -c " + shellQuotePOSIX("http."+executionURL.String()+".followRedirects=false")
	} else {
		// SSH repositories authenticate with a key, never an HTTP password.
		// Preserve repository SSH options (such as a custom identity file).
		sshOut, _, sshCode, sshErr := h.execGit(ctx, sessionID, workdir, "config --get core.sshCommand", 10*time.Second)
		if sshErr != nil || (sshCode != 0 && sshCode != 1) {
			c.Error(apperrors.NewConflictError("SSH configuration could not be read"))
			return
		}
		sshCommand := strings.TrimSpace(sshOut)
		if sshCommand == "" {
			sshCommand = "ssh"
		}
		env["GIT_SSH_COMMAND"] = sshCommand + " -o BatchMode=yes"
	}
	// Pin the commit as well as the branch destination, so a concurrent local
	// commit cannot change what the authentication attempt publishes.
	shaOut, _, shaCode, shaErr := h.execGit(ctx, sessionID, workdir,
		"rev-parse --verify "+shellQuotePOSIX(localRef+"^{commit}"), 10*time.Second)
	pushedSHA := strings.TrimSpace(shaOut)
	if shaErr != nil || shaCode != 0 || !validGitHistorySHA(pushedSHA) {
		c.Error(apperrors.NewConflictError("create a local commit before pushing"))
		return
	}
	args := " --porcelain -- \"$WEKNORA_GIT_PUSH_URL\" " + shellQuotePOSIX(pushedSHA+":"+destination)
	// Contact receive-pack first to validate remote access without changing refs.
	// Both invocations share the quiet prefix so tracing stays off no matter
	// what the surrounding environment exports.
	command := "set +x; " + gitEnvQuietPrefix + "git -C " + shellQuotePOSIX(workdir) + " " + options + " push --dry-run" + args +
		" && git -C " + shellQuotePOSIX(workdir) + " " + options + " push" + args
	result, execErr := h.workspace.ExecWorkspaceCommand(ctx, sessionID, command, workdir, 60*time.Second, env)
	if execErr != nil {
		c.Error(apperrors.NewConflictError("git push could not complete; local commits were preserved"))
		return
	}
	if result.ExitCode != 0 {
		detail := strings.TrimSpace(result.Stderr)
		if detail == "" {
			detail = strings.TrimSpace(result.Stdout)
		}
		authFailed := gitPushAuthFailed(detail)
		detail = gitPushFailureDetail(detail)
		// Also redact echoes of the supplied secret before returning Git output.
		if req.Password != "" {
			for _, secret := range []string{req.Password, url.QueryEscape(req.Password), url.PathEscape(req.Password),
				base64.StdEncoding.EncodeToString([]byte(username + ":" + req.Password))} {
				detail = strings.ReplaceAll(detail, secret, "[redacted]")
			}
		}
		// Git may include credential-bearing remote URLs in its errors.
		detail = gitPushURLPattern.ReplaceAllStringFunc(detail, func(raw string) string {
			u, err := url.Parse(raw)
			if err != nil {
				return "[remote URL]"
			}
			u.User, u.RawQuery, u.Fragment = nil, "", ""
			return u.String()
		})
		// A refused credential prints a whole transcript; the dialog only needs
		// the server's own reason, since it already says authentication failed.
		if authFailed {
			if summary := gitPushAuthSummary(detail); summary != "" {
				detail = summary
			}
		}
		if detail == "" {
			if authFailed {
				detail = "remote authentication failed"
			} else {
				detail = "git push was refused"
			}
		}
		if usingSavedCredentials && authFailed {
			h.forgetSandboxGitCredential(ctx, sessionID, workdir, target, username)
			askForAuth(true, detail)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"pushed": false, "detail": detail, "auth_failed": authFailed}})
		return
	}
	successData := gin.H{"pushed": true}
	if mode == "http" && username != "" {
		remembered := h.rememberSandboxGitCredential(ctx, sessionID, workdir, target, username, req.Password)
		successData["credentials_saved"] = remembered
		if !remembered {
			successData["detail"] = "推送成功，但未能保存沙箱凭据，下次推送可能需要重新输入。"
		}
		h.recordRepoAuthorName(ctx, sessionID, workdir, username)
	}
	warn := func(detail string) {
		if previous, ok := successData["detail"].(string); ok && previous != "" {
			detail = previous + " " + detail
		}
		successData["detail"] = detail
		c.JSON(http.StatusOK, gin.H{"success": true, "data": successData})
	}
	if setUpstream {
		// Using a literal URL above must not turn branch.remote into that URL:
		// preserve the named remote so future fetch/pull and branch counts work.
		for _, setting := range []struct{ key, value string }{
			{"branch." + branch + ".remote", remote}, {"branch." + branch + ".merge", destination},
		} {
			_, _, configCode, configErr := h.execGit(ctx, sessionID, workdir,
				"config --local "+shellQuotePOSIX(setting.key)+" "+shellQuotePOSIX(setting.value), 10*time.Second)
			if configErr != nil || configCode != 0 {
				warn("pushed successfully, but upstream tracking could not be saved")
				return
			}
		}
	}
	// A push to a literal URL does not update the named remote's tracking
	// refs. Record the successfully published snapshot for ahead/history UI;
	// any subsequent local commit remains ahead of this exact SHA.
	trackingOut, _, trackingCode, trackingErr := h.execGit(ctx, sessionID, workdir,
		"for-each-ref --format='%(upstream:remotename)%09%(upstream:remoteref)%09%(upstream)' "+shellQuotePOSIX(localRef), 10*time.Second)
	tracking := strings.Split(strings.TrimSuffix(trackingOut, "\n"), "\t")
	if trackingErr == nil && trackingCode == 0 && len(tracking) == 3 &&
		tracking[0] == remote && tracking[1] == destination && strings.HasPrefix(tracking[2], "refs/remotes/") {
		_, _, updateCode, updateErr := h.execGit(ctx, sessionID, workdir,
			"update-ref "+shellQuotePOSIX(tracking[2])+" "+shellQuotePOSIX(pushedSHA), 10*time.Second)
		if updateErr != nil || updateCode != 0 {
			warn("pushed successfully, but the local remote-tracking ref could not be updated")
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": successData})
}

// recordRepoAuthorName stores the account that just authenticated a push as the
// repository's author name, so the next commit does not ask for an identity
// again. Only an empty config is filled: a name configured deliberately through
// the identity dialog is never overwritten. Errors are ignored — the push has
// already succeeded and must still be reported as such.
func (h *ProgrammingHandler) recordRepoAuthorName(ctx context.Context, sessionID, workdir, name string) {
	current, _, code, err := h.execGit(ctx, sessionID, workdir, "config --local --get user.name", 10*time.Second)
	if err != nil || (code == 0 && strings.TrimSpace(current) != "") {
		return
	}
	_, _, _, _ = h.execGit(ctx, sessionID, workdir, "config --local user.name "+shellQuotePOSIX(name), 10*time.Second)
}
