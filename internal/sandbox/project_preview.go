package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

var ErrPreviewUnsupported = errors.New("sandbox project preview is unavailable")

// Project ports never include control-plane or desktop listeners.
func ValidProjectPort(port int) bool {
	return port >= 1024 && port <= 65535 && port != 49983 && port != 5900 && port != 6080 && port != 9222
}

type RemotePreviewTransport interface {
	RoundTripPreview(context.Context, RemoteSandboxHandle, int, *http.Request) (*http.Response, error)
}

type SessionPreviewTransport interface {
	RoundTripSessionPreview(context.Context, string, string, int, *http.Request) (*http.Response, error)
}

func (m *SessionBoundManager) RoundTripSessionPreview(ctx context.Context, sessionID, expectedID string, port int, req *http.Request) (*http.Response, error) {
	if !ValidProjectPort(port) {
		return nil, ErrPreviewUnsupported
	}
	transport, ok := m.client.(RemotePreviewTransport)
	if !ok {
		return nil, ErrPreviewUnsupported
	}
	if err := m.RequireRunningSessionSandbox(ctx, sessionID); err != nil {
		return nil, err
	}
	handle, found, err := m.lookupSessionHandle(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if !found || handle.ID() != expectedID {
		return nil, ErrNoLiveSessionSandbox
	}
	return transport.RoundTripPreview(ctx, handle, port, req)
}

func (d *WebsocketDialer) roundTripPreview(ctx context.Context, handle RemoteSandboxHandle, port int, req *http.Request) (*http.Response, error) {
	if d == nil || d.httpTransport == nil || d.sandboxDomain == "" || handle == nil || !ValidProjectPort(port) {
		return nil, ErrPreviewUnsupported
	}
	out := req.Clone(ctx)
	out.URL.Scheme = "https"
	if d.scheme == "ws" {
		out.URL.Scheme = "http"
	}
	out.URL.Host = fmt.Sprintf("%d-%s.%s", port, handle.ID(), d.sandboxDomain)
	out.Host = out.URL.Host
	out.RequestURI = ""
	out.Header.Del(InboundTokenHeader)
	if token := InboundTokenOf(handle); token != "" {
		out.Header.Set(InboundTokenHeader, token)
	}
	return d.httpTransport.RoundTrip(out)
}

func (c *CubeRemoteClient) RoundTripPreview(ctx context.Context, handle RemoteSandboxHandle, port int, req *http.Request) (*http.Response, error) {
	return c.wsDialer.roundTripPreview(ctx, handle, port, req)
}
func (c *E2BRemoteClient) RoundTripPreview(ctx context.Context, handle RemoteSandboxHandle, port int, req *http.Request) (*http.Response, error) {
	return c.wsDialer.roundTripPreview(ctx, handle, port, req)
}
func (c *langfuseRemoteClient) RoundTripPreview(ctx context.Context, handle RemoteSandboxHandle, port int, req *http.Request) (*http.Response, error) {
	if inner, ok := c.inner.(RemotePreviewTransport); ok {
		return inner.RoundTripPreview(ctx, handle, port, req)
	}
	return nil, ErrPreviewUnsupported
}
