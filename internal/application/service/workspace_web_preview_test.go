package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/stretchr/testify/require"
)

type previewManager struct {
	stubPinnedManager
	results []*sandbox.ExecuteResult
	cmds    []string
	calls   []sandbox.ShellExecOptions
}

func (m *previewManager) RequireRunningSessionSandbox(context.Context, string) error { return nil }

func (m *previewManager) SessionWorkspaceLayout(context.Context, string) (sandbox.WorkspaceLayout, error) {
	return sandbox.WorkspaceLayout{Root: "/workspace"}, nil
}

func (m *previewManager) ExecShellCommandWithOptions(_ context.Context, _ string, command string, opts sandbox.ShellExecOptions) (*sandbox.ExecuteResult, error) {
	m.cmds = append(m.cmds, command)
	m.calls = append(m.calls, opts)
	if i := len(m.cmds) - 1; i < len(m.results) {
		return m.results[i], nil
	}
	return nil, nil
}

type webPreviewManager struct {
	previewManager
	response *http.Response
	err      error
	requests int
	port     int
	request  *http.Request
}

func (m *webPreviewManager) RoundTripSessionPreview(_ context.Context, sessionID, sandboxID string, port int, req *http.Request) (*http.Response, error) {
	m.requests++
	m.port = port
	m.request = req
	return m.response, m.err
}

func TestWebPreviewDiscoveryChecksGatewayBeforeReportingReady(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		err    error
		want   string
	}{
		{"reachable HTML", 200, nil, "ready"},
		{"gateway 502 despite local readiness", 502, nil, "unreachable"},
		{"gateway authentication rejected", 403, nil, "unreachable"},
		{"relay connection refused", 0, errors.New("dial tcp: connection refused"), "unreachable"},
		{"gateway timeout", 0, context.DeadlineExceeded, "unreachable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := &webPreviewManager{err: tc.err}
			m.id, m.ok = "sandbox-1", true
			m.results = []*sandbox.ExecuteResult{{Stdout: `{"status":"ready","url":"http://127.0.0.1:5173/","port":5173,"relay_port":45678,"secret":"` + strings.Repeat("a", 64) + `"}`}}
			if tc.status != 0 {
				m.response = &http.Response{StatusCode: tc.status, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<!doctype html><html>demo</html>"))}
			}
			a := NewPinnedSessionSandbox(stubPinReader{configID: "cfg", tenantID: 1}, &stubTenantSandboxResolver{mgr: m}, nil, nil)
			preview, err := a.DiscoverWebPreview(context.Background(), "session", false)
			require.NoError(t, err)
			require.Equal(t, tc.want, preview.Status)
			require.Equal(t, 1, m.requests)
			require.Equal(t, 45678, m.port, "probe the same relay used by the iframe")
			require.Equal(t, strings.Repeat("a", 64), m.request.Header.Get("X-WeKnora-Preview-Relay"))
			require.Equal(t, "/", m.request.URL.Path)
			_, bounded := m.request.Context().Deadline()
			require.True(t, bounded, "an unreachable gateway must not hang discovery")
			if tc.want != "ready" {
				require.NotEmpty(t, preview.Detail)
				require.NotContains(t, preview.Detail, strings.Repeat("a", 64))
				require.Empty(t, preview.Secret)
				require.Empty(t, preview.URL)
			}
		})
	}
}

func TestWebPreviewProbeRejectsGatewayJSONAndRedactsProviderErrors(t *testing.T) {
	response := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"gateway":"ok"}`))}
	require.Contains(t, webPreviewProbeDetail(response, nil), "did not return the project HTML")
	detail := webPreviewProbeDetail(nil, errors.New("provider access-token=private-credential"))
	require.NotEmpty(t, detail)
	require.NotContains(t, detail, "private-credential")
}
