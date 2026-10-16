package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Tencent/WeKnora/internal/sandbox"
)

//go:embed workspace_web_preview.py
var workspaceWebPreviewScript string

//go:embed workspace_project.py
var workspaceProjectScript string

//go:embed workspace_web_relay.py
var workspaceWebRelayScript string

type WorkspaceWebPreview struct {
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Port      int    `json:"port,omitempty"`
	RelayPort int    `json:"relay_port,omitempty"`
	Secret    string `json:"secret,omitempty"`
}

func (a *PinnedSessionSandbox) DiscoverWebPreview(ctx context.Context, sessionID string, start bool) (*WorkspaceWebPreview, error) {
	mgr := a.manager(ctx, sessionID)
	if mgr == nil {
		return nil, sandbox.ErrNoLiveSessionSandbox
	}
	if err := requireRunningSessionSandbox(ctx, mgr, sessionID); err != nil {
		return nil, err
	}
	runner, ok := mgr.(sessionShellOptionsExecutor)
	if !ok {
		return nil, sandbox.ErrPreviewUnsupported
	}
	if _, ok := mgr.(sandbox.SessionPreviewTransport); !ok {
		return nil, sandbox.ErrPreviewUnsupported
	}
	provider, ok := mgr.(sandbox.SessionWorkspaceLayoutProvider)
	if !ok {
		return nil, sandbox.ErrPreviewUnsupported
	}
	layout, err := provider.SessionWorkspaceLayout(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	layout = layout.Normalized()
	if !layout.HasRoot() || layout.IsHost() {
		return nil, sandbox.ErrPreviewUnsupported
	}
	args, _ := json.Marshal(map[string]any{"root": layout.Root, "start": start, "relay_script": workspaceWebRelayScript})
	command := "python3 -c " + sandbox.ShellQuote(workspaceProjectScript+"\n"+workspaceWebPreviewScript) + " " + sandbox.ShellQuote(string(args))
	result, err := runner.ExecShellCommandWithOptions(ctx, sessionID, command, sandbox.ShellExecOptions{Timeout: 40 * time.Second, SkipWorkspacePrep: true})
	if err != nil {
		return nil, err
	}
	if result == nil || !result.IsSuccess() {
		return nil, fmt.Errorf("project discovery failed")
	}
	var preview WorkspaceWebPreview
	if err := json.Unmarshal([]byte(result.Stdout), &preview); err != nil {
		return nil, fmt.Errorf("invalid project discovery response")
	}
	if preview.Status == "ready" && (!sandbox.ValidProjectPort(preview.Port) || !sandbox.ValidProjectPort(preview.RelayPort) || len(preview.Secret) != 64) {
		return nil, fmt.Errorf("invalid project relay")
	}
	if preview.Status == "ready" {
		sandboxID, bound := a.BoundSandboxID(ctx, sessionID)
		if !bound {
			return nil, sandbox.ErrNoLiveSessionSandbox
		}
		// A listener inside the guest does not prove the gateway can reach it.
		// Use the iframe's authenticated forwarding route before minting its URL.
		probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, "http://localhost/", nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-WeKnora-Preview-Relay", preview.Secret)
		response, err := mgr.(sandbox.SessionPreviewTransport).RoundTripSessionPreview(probeCtx, sessionID, sandboxID, preview.RelayPort, req)
		if detail := webPreviewProbeDetail(response, err); detail != "" {
			return &WorkspaceWebPreview{Status: "unreachable", Port: preview.Port, Detail: detail}, nil
		}
	}
	return &preview, nil
}

func webPreviewProbeDetail(response *http.Response, err error) string {
	if response != nil && response.Body != nil {
		defer response.Body.Close()
	}
	// Do not expose upstream error strings or bodies: they may contain provider
	// credentials. The HTTP status and failure stage suffice for troubleshooting.
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "The project is running, but the server-to-sandbox preview connection timed out. Check the sandbox gateway and preview port routing."
		}
		return "The project is running, but the server cannot connect to its sandbox preview relay. Check the sandbox gateway, routing domain and network access."
	}
	if response == nil || response.Body == nil {
		return "The sandbox preview relay returned no HTTP response."
	}
	if response.StatusCode >= 400 {
		return fmt.Sprintf("The server-to-sandbox preview request returned HTTP %d. Check the sandbox gateway, inbound authentication and relay port routing.", response.StatusCode)
	}
	sample, readErr := io.ReadAll(io.LimitReader(response.Body, 1024))
	if readErr != nil {
		return "The sandbox preview connection closed before the project page could be read."
	}
	sample = bytes.ToLower(sample)
	if !strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "text/html") && !bytes.Contains(sample, []byte("<!doctype html")) && !bytes.Contains(sample, []byte("<html")) {
		return "The sandbox preview route did not return the project HTML page. Check the gateway routing and project frontend port."
	}
	return ""
}

func (a *PinnedSessionSandbox) RoundTripWebPreview(ctx context.Context, sessionID, sandboxID string, port int, req *http.Request) (*http.Response, error) {
	mgr := a.manager(ctx, sessionID)
	if transport, ok := mgr.(sandbox.SessionPreviewTransport); ok {
		return transport.RoundTripSessionPreview(ctx, sessionID, sandboxID, port, req)
	}
	return nil, sandbox.ErrPreviewUnsupported
}
