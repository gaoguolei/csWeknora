package mcp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
	secutils "github.com/Tencent/WeKnora/internal/utils"
)

// ErrOutboundURLBlocked marks a URL the outbound security policy refused. It
// wraps the underlying SSRF reason, so callers can tell "the address is blocked
// by policy, fix the address or the whitelist" apart from "the address is fine
// but the upstream did not answer" — the two need different words for the user.
var ErrOutboundURLBlocked = errors.New("outbound URL blocked by security policy")

// ValidateServiceOutboundURLs validates every URL that the MCP transport or
// OAuth discovery flow may contact. It is intentionally called both at
// persistence boundaries and immediately before client construction so stale
// or imported rows cannot bypass the current SSRF policy.
func ValidateServiceOutboundURLs(service *types.MCPService) error {
	if service == nil {
		return fmt.Errorf("MCP service is required")
	}
	if service.URL != nil {
		serviceURL := strings.TrimSpace(*service.URL)
		if serviceURL != "" {
			if err := secutils.ValidateURLForSSRF(serviceURL); err != nil {
				return fmt.Errorf("%w: MCP service URL failed SSRF validation: %w", ErrOutboundURLBlocked, err)
			}
		}
	}
	if service.AuthConfig != nil {
		metadataURL := strings.TrimSpace(service.AuthConfig.AuthServerMetadataURL)
		if metadataURL != "" {
			if err := secutils.ValidateURLForSSRF(metadataURL); err != nil {
				return fmt.Errorf(
					"%w: MCP OAuth metadata URL failed SSRF validation: %w", ErrOutboundURLBlocked, err,
				)
			}
		}
	}
	return nil
}
