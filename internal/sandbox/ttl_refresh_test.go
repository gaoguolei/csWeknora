package sandbox

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type recordingTTLClient struct {
	*fakeRemoteClient
	called bool
	handle RemoteSandboxHandle
}

func (c *recordingTTLClient) StartTTLRefresh(_ context.Context, handle RemoteSandboxHandle) {
	c.called = true
	c.handle = handle
}

func TestTaskTTLRefreshSurvivesTracingWrapper(t *testing.T) {
	inner := &recordingTTLClient{fakeRemoteClient: newFakeRemoteClient(SandboxTypeCube)}
	inner.capabilities.SupportsTimeoutRefresh = true
	wrapped := wrapLangfuseRemoteClient(inner)
	refresher, ok := TTLRefresherFrom(wrapped)
	require.True(t, ok)
	handle := &fakeRemoteHandle{id: "validation-sandbox", provider: SandboxTypeCube}
	refresher.StartTTLRefresh(context.Background(), handle)
	require.True(t, inner.called)
	require.Same(t, handle, inner.handle)

	inner.capabilities.SupportsTimeoutRefresh = false
	_, ok = TTLRefresherFrom(wrapped)
	require.False(t, ok, "wrapping must not invent timeout renewal support")
}
