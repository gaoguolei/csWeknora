package sandbox

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// desktopEchoServer is a websockify stand-in: it records the handshake
// request and accepts the upgrade.
func desktopEchoServer(t *testing.T, seen *http.Request) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{
		CheckOrigin:  func(*http.Request) bool { return true },
		Subprotocols: []string{"binary"},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r.Clone(r.Context())
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _, _ = conn.NextReader()
	}))
}

func TestWebsocketDialerAttachesInboundTokenAndExtraHeaders(t *testing.T) {
	var seen http.Request
	srv := desktopEchoServer(t, &seen)
	defer srv.Close()

	pool := NewSandboxGatewayTransportPoolWithPolicy(nil, OutboundURLPolicy{AllowPrivate: true})
	pool.InboundTokens().Put("sbx-1", "tok-abc")

	cfg := &Config{
		Type:                  SandboxTypeCube,
		CubeProxyURL:          srv.URL,
		CubeSandboxDomain:     "cube.app",
		AllowPrivateEndpoints: true,
	}
	dialer := pool.WebsocketDialerFor(cfg)
	require.NotNil(t, dialer)

	extra := http.Header{}
	extra.Set("Authorization", "Basic dGVzdDp0ZXN0")

	conn, resp, err := dialer.Dial(context.Background(), "sbx-1", 6080, "/websockify", extra)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Gateway credential, injected from THIS pool's registry.
	require.Equal(t, "tok-abc", seen.Header.Get(InboundTokenHeader))
	// websockify credential, passed through from the caller.
	require.Equal(t, "Basic dGVzdDp0ZXN0", seen.Header.Get("Authorization"))
	// Host header is what the gateway routes on.
	require.Equal(t, "6080-sbx-1.cube.app", seen.Host)
	require.Equal(t, "/websockify", seen.URL.Path)
	// binary is the only acceptable subprotocol: base64 would turn the RFB
	// byte stream into text and desync the handshake counter in Task 9.
	require.Equal(t, "binary", conn.Subprotocol())
}

func TestWebsocketDialerUsesItsOwnPoolRegistry(t *testing.T) {
	// The process runs two pools with two registries. A dialer must read the
	// one its own pool owns; reading the other yields an empty token and a
	// gateway 401 that looks exactly like the replica-affinity bug.
	var seen http.Request
	srv := desktopEchoServer(t, &seen)
	defer srv.Close()

	publicPool := NewSandboxGatewayTransportPool(nil)
	privatePool := NewSandboxGatewayTransportPoolWithPolicy(nil, OutboundURLPolicy{AllowPrivate: true})
	publicPool.InboundTokens().Put("sbx-1", "public-token")
	privatePool.InboundTokens().Put("sbx-1", "private-token")

	cfg := &Config{
		Type:                  SandboxTypeCube,
		CubeProxyURL:          srv.URL,
		CubeSandboxDomain:     "cube.app",
		AllowPrivateEndpoints: true,
	}
	conn, _, err := privatePool.WebsocketDialerFor(cfg).
		Dial(context.Background(), "sbx-1", 6080, "/websockify", nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	require.Equal(t, "private-token", seen.Header.Get(InboundTokenHeader))
}

func TestWebsocketDialerBlocksPrivateAddressWhenPolicyForbids(t *testing.T) {
	// httptest listens on 127.0.0.1, which the default policy must refuse.
	var seen http.Request
	srv := desktopEchoServer(t, &seen)
	defer srv.Close()

	pool := NewSandboxGatewayTransportPool(nil) // DefaultOutboundURLPolicy
	pool.InboundTokens().Put("sbx-1", "tok")

	cfg := &Config{
		Type:              SandboxTypeCube,
		CubeProxyURL:      srv.URL,
		CubeSandboxDomain: "cube.app",
	}
	_, _, err := pool.WebsocketDialerFor(cfg).
		Dial(context.Background(), "sbx-1", 6080, "/websockify", nil)
	require.Error(t, err, "SSRF guard must refuse a loopback gateway under the default policy")
}

func TestWebsocketDialerRejectsEmptySandboxDomain(t *testing.T) {
	pool := NewSandboxGatewayTransportPoolWithPolicy(nil, OutboundURLPolicy{AllowPrivate: true})
	cfg := &Config{Type: SandboxTypeCube, CubeProxyURL: "http://127.0.0.1:1", AllowPrivateEndpoints: true}
	_, _, err := pool.WebsocketDialerFor(cfg).
		Dial(context.Background(), "sbx-1", 6080, "/websockify", nil)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "domain")
}

func TestWebsocketDialerE2BEmptyDomainUsesSDKDefault(t *testing.T) {
	// E2B Cloud configs may omit sandbox_domain: go-e2b fills e2b.app for
	// envd, and the desktop Host header has to match that same default.
	var seen http.Request
	srv := desktopEchoServer(t, &seen)
	defer srv.Close()

	pool := NewSandboxGatewayTransportPoolWithPolicy(nil, OutboundURLPolicy{AllowPrivate: true})
	pool.InboundTokens().Put("sbx-1", "tok")

	cfg := &Config{
		Type:                  SandboxTypeE2B,
		E2BProxyURL:           srv.URL,
		AllowPrivateEndpoints: true,
	}
	conn, _, err := pool.WebsocketDialerFor(cfg).
		Dial(context.Background(), "sbx-1", 6080, "/websockify", nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.Equal(t, "6080-sbx-1.e2b.app", seen.Host)
}

func TestWebsocketDialerE2BConfiguredDomainWinsOverDefault(t *testing.T) {
	var seen http.Request
	srv := desktopEchoServer(t, &seen)
	defer srv.Close()

	pool := NewSandboxGatewayTransportPoolWithPolicy(nil, OutboundURLPolicy{AllowPrivate: true})
	pool.InboundTokens().Put("sbx-1", "tok")

	cfg := &Config{
		Type:                  SandboxTypeE2B,
		E2BProxyURL:           srv.URL,
		E2BSandboxDomain:      "sandbox.internal",
		AllowPrivateEndpoints: true,
	}
	conn, _, err := pool.WebsocketDialerFor(cfg).
		Dial(context.Background(), "sbx-1", 6080, "/websockify", nil)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.Equal(t, "6080-sbx-1.sandbox.internal", seen.Host)
}
