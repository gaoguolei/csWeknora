package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func previewBaseFor(t *testing.T, envValue, origin, host string) (string, error) {
	t.Helper()
	t.Setenv("WEKNORA_PREVIEW_BASE_URL", envValue)
	request := httptest.NewRequest("POST", "http://app/api/v1/sessions/x/programming/web-preview", nil)
	request.Host = host
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	base, err := previewPublicBase(request)
	if err != nil {
		return "", err
	}
	return base.String(), nil
}

type webPreviewRecoveryManager struct {
	sandbox.Manager
	status int
}

func (m *webPreviewRecoveryManager) Resolve(context.Context, uint64, string) (sandbox.Manager, error) {
	return m, nil
}
func (m *webPreviewRecoveryManager) BoundSandboxID(context.Context, string) (string, bool) {
	return "preview-sandbox", true
}
func (m *webPreviewRecoveryManager) SessionWorkspaceLayout(context.Context, string) (sandbox.WorkspaceLayout, error) {
	return sandbox.WorkspaceLayout{Root: "/workspace"}, nil
}
func (m *webPreviewRecoveryManager) ExecShellCommandWithOptions(context.Context, string, string, sandbox.ShellExecOptions) (*sandbox.ExecuteResult, error) {
	return &sandbox.ExecuteResult{Stdout: `{"status":"ready","port":5173,"relay_port":45678,"secret":"` + strings.Repeat("a", 64) + `"}`}, nil
}
func (m *webPreviewRecoveryManager) RoundTripSessionPreview(context.Context, string, string, int, *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: m.status, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader("<html>project</html>"))}, nil
}

func TestWebPreviewRenewsIframeURLAfterUpstreamGatewayFailure(t *testing.T) {
	t.Setenv("WEKNORA_PREVIEW_BASE_URL", "")
	for _, status := range []int{200, 400, 500, 502, 503, 504} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			m := &webPreviewRecoveryManager{status: status}
			h := NewProgrammingHandler(programmingGitSession{}, service.NewPinnedSessionSandbox(programmingGitPin{}, m, nil, nil))
			router := gin.New()
			router.Use(h.WebPreviewMiddleware(nil))
			router.POST("/sessions/:session_id/web-preview", h.DiscoverWebPreview)
			token := strings.Repeat("b", 32)
			host := "wkpreview-" + token + ".localhost:8080"
			lease := previewLease{UserID: "user", TenantID: 1, SessionID: "git-test", SandboxID: "preview-sandbox", Port: 5173, RelayPort: 45678, Secret: strings.Repeat("a", 64), Host: host, AppOrigin: "http://localhost:5173"}
			require.NoError(t, h.previewLeases.put(context.Background(), token, lease))
			// A failed JS module load can leave the document blank even when
			// subsequent discovery finds the frontend's HTML responding again.
			pageRequest := httptest.NewRequest("GET", "http://"+host+"/src/main.js", nil)
			pageCtx, cancel := context.WithCancel(pageRequest.Context())
			defer cancel()
			pageResponse := httptest.NewRecorder()
			router.ServeHTTP(pageResponse, pageRequest.WithContext(pageCtx))
			require.Equal(t, status, pageResponse.Code, "preserve the project's response status")
			m.status = 200
			request := httptest.NewRequest("POST", "http://localhost:8080/sessions/git-test/web-preview", strings.NewReader(`{"preview_id":"`+token+`"}`))
			request.Header.Set("Origin", "http://localhost:5173")
			request.Header.Set("Content-Type", "application/json")
			ctx := context.WithValue(request.Context(), types.UserIDContextKey, "user")
			ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request.WithContext(ctx))
			require.Equal(t, 200, response.Code)
			var result struct {
				Data struct {
					Status string `json:"status"`
					ID     string `json:"preview_id"`
					URL    string `json:"url"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
			require.Equal(t, "ready", result.Data.Status)
			if status == 502 || status == 503 || status == 504 {
				require.NotEqual(t, token, result.Data.ID, "the iframe must navigate again after a gateway failure")
				require.NotContains(t, result.Data.URL, token)
			} else {
				require.Equal(t, token, result.Data.ID, "normal responses and project compilation errors keep the current frame")
			}
		})
	}
}

func TestWebPreviewBrowserProbeRestrictsCORSAndDoesNotExposeSecrets(t *testing.T) {
	m := &webPreviewRecoveryManager{status: 200}
	h := NewProgrammingHandler(programmingGitSession{}, service.NewPinnedSessionSandbox(programmingGitPin{}, m, nil, nil))
	router := gin.New()
	router.Use(h.WebPreviewMiddleware(nil))
	token := strings.Repeat("b", 32)
	host := "wkpreview-" + token + ".localhost:8080"
	lease := previewLease{UserID: "user", TenantID: 1, SessionID: "git-test", SandboxID: "preview-sandbox", Port: 5173, RelayPort: 45678, Secret: strings.Repeat("a", 64), Host: host, AppOrigin: "http://localhost:5173"}
	require.NoError(t, h.previewLeases.put(context.Background(), token, lease))
	for _, origin := range []string{lease.AppOrigin, "http://other.example"} {
		r := httptest.NewRequest("GET", "http://"+host+"/.weknora/preview-health", nil)
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if origin == lease.AppOrigin {
			require.Equal(t, 200, w.Code)
			require.Equal(t, origin, w.Header().Get("Access-Control-Allow-Origin"))
			require.Contains(t, w.Body.String(), "ready")
		} else {
			require.Equal(t, 403, w.Code)
			require.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
		}
		require.NotContains(t, w.Body.String(), lease.Secret)
	}
}

// The hostname must come from the origin (the address the browser used) and the
// port from this request (the listener that serves the relay). Swapping them
// breaks one of the two cases below.
func TestPreviewPublicBaseFollowsOriginAddress(t *testing.T) {
	cases := []struct {
		name, origin, host, want string
	}{
		{"dev app on another port", "http://localhost:5173", "localhost:8080", "http://localhost:8080"},
		// A colleague's browser loads the page from this machine, but the dev
		// server proxies /api and rewrites Host to localhost. Taking the name from
		// the request would hand them *.localhost — their own loopback.
		{"lan browser behind a dev proxy", "http://192.168.3.178:5173", "localhost:8080", "http://192-168-3-178.nip.io:8080"},
		{"loopback ip", "http://127.0.0.1:5173", "127.0.0.1:8080", "http://localhost:8080"},
		{"same origin", "http://192.168.3.178:8080", "192.168.3.178:8080", "http://192-168-3-178.nip.io:8080"},
		{"default port", "http://10.0.0.5", "10.0.0.5", "http://10-0-0-5.nip.io:80"},
		{"tls address", "https://192.168.3.178", "192.168.3.178", "https://192-168-3-178.nip.io:443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := previewBaseFor(t, "", tc.origin, tc.host)
			if err != nil {
				t.Fatalf("previewPublicBase(origin %q, host %q) = %v", tc.origin, tc.host, err)
			}
			if got != tc.want {
				t.Fatalf("base = %q, want %q", got, tc.want)
			}
		})
	}
}

// A named host owns DNS we do not control, so it still needs the variable.
func TestPreviewPublicBaseRequiresDomainForNamedOrigin(t *testing.T) {
	cases := []struct{ origin, host string }{
		{"http://weknora.example.com", "weknora.example.com:8080"},
		{"", "192.168.3.178:8080"},
		{"null", "192.168.3.178:8080"},
	}
	for _, tc := range cases {
		if got, err := previewBaseFor(t, "", tc.origin, tc.host); err == nil {
			t.Fatalf("origin %q host %q must not resolve a preview base, got %q", tc.origin, tc.host, got)
		}
	}
}

// An explicit base always wins, so a deployment behind wildcard DNS can pin it.
func TestPreviewPublicBasePrefersConfiguredBase(t *testing.T) {
	got, err := previewBaseFor(t, "https://preview.example.com", "http://192.168.3.178:5173", "192.168.3.178:8080")
	if err != nil || got != "https://preview.example.com" {
		t.Fatalf("configured base ignored: %q, %v", got, err)
	}
	// A base carrying a path, query or credentials is not a host to route.
	for _, bad := range []string{"preview.example.com", "https://preview.example.com/x", "https://u@preview.example.com", "https://preview.example.com/?a=1"} {
		if got, err := previewBaseFor(t, bad, "http://192.168.3.178:5173", "192.168.3.178:8080"); err == nil {
			t.Fatalf("base %q must be rejected, got %q", bad, got)
		}
	}
}
