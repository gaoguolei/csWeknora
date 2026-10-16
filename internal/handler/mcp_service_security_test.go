package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/mcp"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type securityMCPService struct {
	interfaces.MCPServiceService
	err error
}

func (s *securityMCPService) CreateMCPService(context.Context, *types.MCPService) error {
	return s.err
}

func (s *securityMCPService) UpdateMCPService(context.Context, *types.MCPService, map[string]bool) error {
	return s.err
}

func TestMCPSaveSecurityErrorsReturnBadRequest(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut} {
		for _, tc := range []struct {
			name   string
			err    error
			status int
		}{
			{"policy", fmt.Errorf("revalidation: %w", mcp.ErrOutboundURLBlocked), http.StatusBadRequest},
			{"whitelist", fmt.Errorf("dial: %w", secutils.ErrSSRFHostNotWhitelisted), http.StatusBadRequest},
			{"redirect", fmt.Errorf("OAuth discovery: %w", secutils.ErrSSRFRedirectBlocked), http.StatusBadRequest},
			{"internal", errors.New("database unavailable"), http.StatusInternalServerError},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				gin.SetMode(gin.TestMode)
				r := gin.New()
				r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
					c.Set(types.TenantIDContextKey.String(), uint64(7))
				})
				h := &MCPServiceHandler{mcpServiceService: &securityMCPService{err: tc.err}}
				r.POST("/mcp-services", h.CreateMCPService)
				r.PUT("/mcp-services/:id", h.UpdateMCPService)
				path := "/mcp-services"
				if method == http.MethodPut {
					path += "/saved"
				}
				// Partial updates can omit the URL; service-layer validation still
				// checks the stored address against the current whitelist.
				req := httptest.NewRequest(method, path, bytes.NewBufferString(`{"name":"External MCP","enabled":true}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				require.Equal(t, tc.status, w.Code, w.Body.String())
				var response struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				require.Contains(t, response.Error.Message, tc.err.Error())
			})
		}
	}
}

func TestMCPOAuthWhitelistRejectionReturnsBadRequest(t *testing.T) {
	t.Setenv("SSRF_DNS_WHITELIST_ONLY", "true")
	t.Setenv("SSRF_WHITELIST", "")
	t.Setenv("SSRF_WHITELIST_EXTRA", "")
	secutils.ResetSSRFWhitelistForTest()
	t.Cleanup(secutils.ResetSSRFWhitelistForTest)
	url := "https://blocked.example.com/mcp"
	svc := &usageMCPService{service: &types.MCPService{
		ID: "saved", URL: &url, AuthConfig: &types.MCPAuthConfig{AuthType: types.MCPAuthOAuth},
	}}
	h := NewMCPOAuthHandler(mcp.NewOAuthManager(nil, nil, nil), nil, svc, nil)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.ErrorHandler(), func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		c.Request = c.Request.WithContext(types.WithPrincipal(c.Request.Context(), types.Principal{Type: types.PrincipalWebUser, ID: "user"}))
	})
	r.POST("/mcp-services/:id/oauth/authorize-url", h.AuthorizeURL)
	req := httptest.NewRequest(http.MethodPost, "/mcp-services/saved/oauth/authorize-url", bytes.NewBufferString(`{"redirect_uri":"https://app.example.com/api/v1/mcp-oauth/callback"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "blocked.example.com")
	require.Contains(t, w.Body.String(), "SSRF whitelist")
}
