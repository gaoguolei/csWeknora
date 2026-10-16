package sandbox

import "context"

// RemoteTTLRefresher keeps a sandbox alive for a caller-owned task lifetime.
type RemoteTTLRefresher interface {
	StartTTLRefresh(ctx context.Context, handle RemoteSandboxHandle)
}

// TTLRefresherFrom narrows a client to task lifetime renewal.
//
// SupportsTimeoutRefresh is the advertised capability (Docker is false: it
// has no SetTimeout). The type assertion finds the method through wrappers
// such as Langfuse. Both signals must agree before renewal is enabled.
func TTLRefresherFrom(client RemoteSandboxClient) (RemoteTTLRefresher, bool) {
	if client == nil {
		return nil, false
	}
	refresher, ok := client.(RemoteTTLRefresher)
	if !ok {
		return nil, false
	}
	if !client.Capabilities().SupportsTimeoutRefresh {
		return nil, false
	}
	return refresher, true
}
