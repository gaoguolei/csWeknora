package handler

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
)

// The provider executes these commands only in the pinned session sandbox.
// Keep credentials outside the repository and separate even if a provider
// reuses a home directory. Never return credentials through an HTTP response.
func sandboxGitCredentialFile(sessionID string) string {
	return fmt.Sprintf("${HOME:?}/.config/weknora/git/%x/credentials", sha256.Sum256([]byte(sessionID)))
}

func (h *ProgrammingHandler) hasRemoteCredentialWorkspace(ctx context.Context, sessionID string) bool {
	if h.workspace == nil {
		return false
	}
	layout, err := h.workspace.SessionWorkspaceLayout(ctx, sessionID)
	return err == nil && !layout.IsHost()
}

func gitCredentialEnvironment(target, username, password string) (map[string]string, bool) {
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, false
	}
	path := strings.Trim(u.Path, "/")
	if strings.ContainsAny(u.Host+path+username+password, "\r\n\x00") {
		return nil, false
	}
	return map[string]string{
		"WEKNORA_GIT_PROTOCOL": u.Scheme, "WEKNORA_GIT_HOST": strings.ToLower(u.Host),
		"WEKNORA_GIT_PATH": path, "WEKNORA_GIT_USERNAME": username, "WEKNORA_GIT_PASSWORD": password,
		"GIT_TERMINAL_PROMPT": "0", "GIT_TRACE_REDACT": "1",
	}, true
}

const gitCredentialInput = `printf 'protocol=%s\nhost=%s\npath=%s\n' "$WEKNORA_GIT_PROTOCOL" "$WEKNORA_GIT_HOST" "$WEKNORA_GIT_PATH"`

func (h *ProgrammingHandler) readSandboxGitCredential(ctx context.Context, sessionID, workdir, target string) (string, string, bool) {
	// Credential protocol output is private, even if the caller has an output
	// observer for ordinary workspace commands.
	ctx = sandbox.WithCommandOutput(ctx, nil)
	if !h.hasRemoteCredentialWorkspace(ctx, sessionID) {
		return "", "", false
	}
	env, ok := gitCredentialEnvironment(target, "", "")
	if !ok {
		return "", "", false
	}
	file := sandboxGitCredentialFile(sessionID)
	command := "set +x; " + gitEnvQuietPrefix + "[ -f \"" + file + "\" ] && [ ! -L \"" + file + "\" ] && " +
		gitCredentialInput + " | git credential-store --file \"" + file + "\" get"
	result, err := h.workspace.ExecWorkspaceCommand(ctx, sessionID, command, workdir, 10*time.Second, env)
	if err != nil || result == nil || result.ExitCode != 0 {
		return "", "", false
	}
	username, password := "", ""
	for _, line := range strings.Split(result.Stdout, "\n") {
		key, value, _ := strings.Cut(line, "=")
		if key == "username" {
			username = value
		}
		if key == "password" {
			password = value
		}
	}
	return username, password, username != "" && password != ""
}

// Remember only credentials that successfully published a commit. Git's store
// file is plaintext, with a private directory and mode 0600; neither the
// password nor token is stored in .git/config, the worktree, or the browser.
func (h *ProgrammingHandler) rememberSandboxGitCredential(ctx context.Context, sessionID, workdir, target, username, password string) bool {
	if !h.hasRemoteCredentialWorkspace(ctx, sessionID) {
		return false
	}
	env, ok := gitCredentialEnvironment(target, username, password)
	if !ok {
		return false
	}
	file := sandboxGitCredentialFile(sessionID)
	helper := "!git credential-store --file \"" + file + "\""
	key := "credential." + target
	command := "set +x; " + gitEnvQuietPrefix + "umask 077; credential_file=\"" + file + "\"; credential_dir=${credential_file%/*}; " +
		"[ ! -L \"$credential_dir\" ] && [ ! -L \"$credential_file\" ] && mkdir -p -- \"$credential_dir\" && chmod 700 \"$credential_dir\" && " +
		"{ " + gitCredentialInput + `; printf 'username=%s\npassword=%s\n' "$WEKNORA_GIT_USERNAME" "$WEKNORA_GIT_PASSWORD"; } | ` +
		"git credential-store --file \"$credential_file\" store && chmod 600 \"$credential_file\" && " +
		"git -C " + shellQuotePOSIX(workdir) + " config --local --replace-all " + shellQuotePOSIX(key+".helper") + " '' && " +
		"git -C " + shellQuotePOSIX(workdir) + " config --local --add " + shellQuotePOSIX(key+".helper") + " " + shellQuotePOSIX(helper) + " && " +
		"git -C " + shellQuotePOSIX(workdir) + " config --local " + shellQuotePOSIX(key+".useHttpPath") + " true && " +
		"git -C " + shellQuotePOSIX(workdir) + " config --local " + shellQuotePOSIX(key+".username") + " " + shellQuotePOSIX(username) + " && " +
		"git -C " + shellQuotePOSIX(workdir) + " config --global user.name " + shellQuotePOSIX(username)
	result, err := h.workspace.ExecWorkspaceCommand(ctx, sessionID, command, workdir, 15*time.Second, env)
	return err == nil && result != nil && result.ExitCode == 0
}

func (h *ProgrammingHandler) forgetSandboxGitCredential(ctx context.Context, sessionID, workdir, target, username string) {
	env, ok := gitCredentialEnvironment(target, username, "")
	if !ok {
		return
	}
	command := "set +x; " + gitEnvQuietPrefix + "{ " + gitCredentialInput + `; printf 'username=%s\n' "$WEKNORA_GIT_USERNAME"; } | ` +
		"git credential-store --file \"" + sandboxGitCredentialFile(sessionID) + "\" erase"
	_, _ = h.workspace.ExecWorkspaceCommand(ctx, sessionID, command, workdir, 10*time.Second, env)
}
