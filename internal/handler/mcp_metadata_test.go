package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestMCPMetadataAppErrorDoesNotLeakUpstreamDetails(t *testing.T) {
	notFound := mcpMetadataAppError(types.ErrMCPServiceNotFound, false)
	require.Equal(t, http.StatusNotFound, notFound.HTTPCode)
	require.Equal(t, "MCP service not found", notFound.Message)

	unauthorized := mcpMetadataAppError(types.ErrMCPOAuthPrincipalRequired, true)
	require.Equal(t, http.StatusUnauthorized, unauthorized.HTTPCode)

	conflict := mcpMetadataAppError(types.ErrMCPMetadataConnectionChanged, true)
	require.Equal(t, http.StatusConflict, conflict.HTTPCode)

	unavailable := mcpMetadataAppError(types.ErrMCPMetadataStorage, false)
	require.Equal(t, http.StatusServiceUnavailable, unavailable.HTTPCode)

	refresh := mcpMetadataAppError(errors.New("dial tcp 10.1.2.3:443 https://secret.internal/mcp"), true)
	require.Equal(t, http.StatusBadRequest, refresh.HTTPCode)
	require.Equal(t, "Failed to refresh MCP tools. Check the connection and try again.", refresh.Message)
	require.NotContains(t, refresh.Error(), "secret.internal")
	require.NotContains(t, refresh.Error(), "10.1.2.3")
}

// An outbound policy block must not fall through to the generic refresh text:
// "check the connection" points users at the upstream when the address itself
// is what the server refused.
func TestMCPMetadataAppErrorExplainsOutboundPolicyBlocks(t *testing.T) {
	// A direct IP is refused by the URL layer before DNS, so this stays
	// hermetic. It is the plain SSRF block, not whitelist-only mode.
	directIP := "http://10.1.2.3:8080/mcp"
	blocked := mcp.ValidateServiceOutboundURLs(&types.MCPService{URL: &directIP})
	require.ErrorIs(t, blocked, mcp.ErrOutboundURLBlocked)
	require.NotErrorIs(t, blocked, secutils.ErrSSRFHostNotWhitelisted)

	appErr := mcpMetadataAppError(blocked, true)
	require.Equal(t, http.StatusBadRequest, appErr.HTTPCode)
	require.NotEqual(t, "Failed to refresh MCP tools. Check the connection and try again.", appErr.Message)
	require.Contains(t, appErr.Message, "SSRF_WHITELIST_EXTRA")
	require.NotContains(t, appErr.Message, "10.1.2.3")
}

func TestMCPMetadataAppErrorNamesWhitelistOnlyMode(t *testing.T) {
	t.Setenv("SSRF_DNS_WHITELIST_ONLY", "true")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)

	host := "https://blocked.example.com/mcp"
	blocked := mcp.ValidateServiceOutboundURLs(&types.MCPService{URL: &host})
	require.ErrorIs(t, blocked, secutils.ErrSSRFHostNotWhitelisted)

	appErr := mcpMetadataAppError(blocked, true)
	require.Equal(t, http.StatusBadRequest, appErr.HTTPCode)
	require.Contains(t, appErr.Message, "SSRF_DNS_WHITELIST_ONLY")
	require.Contains(t, appErr.Message, "SSRF_WHITELIST_EXTRA")
	require.NotContains(t, appErr.Message, "blocked.example.com")
}

func TestMayWriteSharedMCPMetadata(t *testing.T) {
	require.False(t, mayWriteSharedMCPMetadata(context.Background()))
	require.False(t, mayWriteSharedMCPMetadata(types.WithCaller(
		context.Background(),
		types.Caller{Role: types.TenantRoleViewer},
	)))
	require.True(t, mayWriteSharedMCPMetadata(types.WithCaller(
		context.Background(),
		types.Caller{Role: types.TenantRoleAdmin},
	)))
	require.True(t, mayWriteSharedMCPMetadata(types.WithTenantAPIKeyScope(
		context.Background(),
		types.TenantAPIKeyScope{KeyID: 1, FullAccess: true},
	)))
	require.True(t, mayWriteSharedMCPMetadata(
		context.WithValue(context.Background(), types.SystemAdminContextKey, true),
	))
}
