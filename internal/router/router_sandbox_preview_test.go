package router

import (
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/handler"
	sessionhandler "github.com/Tencent/WeKnora/internal/handler/session"
	"github.com/gin-gonic/gin"
)

func TestSandboxRoutesExposeWebPreviewWithoutRemoteDesktop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	g := &rbacGuards{}
	RegisterSessionRoutes(v1, &sessionhandler.Handler{}, nil, g)
	RegisterSandboxTerminalRoutes(r, &sessionhandler.Handler{})
	RegisterProgrammingRoutes(v1, &handler.ProgrammingHandler{}, g)
	foundPreview, foundTerminal := false, false
	for _, route := range r.Routes() {
		if strings.Contains(route.Path, "/sandbox/desktop") || strings.HasSuffix(route.Path, "/programming/preview") {
			t.Fatalf("retired desktop route is still registered: %s %s", route.Method, route.Path)
		}
		foundPreview = foundPreview || strings.HasSuffix(route.Path, "/programming/web-preview")
		foundTerminal = foundTerminal || strings.HasSuffix(route.Path, "/sandbox/terminal")
	}
	if !foundPreview || !foundTerminal {
		t.Fatalf("remaining capabilities missing: web preview=%v terminal=%v", foundPreview, foundTerminal)
	}
}
