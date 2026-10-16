package handler

import (
	"errors"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
)

func NewProgrammingHandlerWithTests(sessions interfaces.SessionService, workspace *service.PinnedSessionSandbox,
	starter *service.SandboxTerminalService, agents interfaces.CustomAgentService, shares interfaces.AgentShareService,
	tests *service.WorkspaceTestService) *ProgrammingHandler {
	h := NewProgrammingHandlerWithPreview(sessions, workspace, starter, agents, shares)
	h.tests = tests
	return h
}

func (h *ProgrammingHandler) testAvailable(c *gin.Context) bool {
	if _, ok := h.loadOwnedSession(c); !ok {
		return false
	}
	if h.tests == nil {
		c.JSON(503, gin.H{"error": "Project validation is unavailable"})
		return false
	}
	return true
}

func (h *ProgrammingHandler) TestProjects(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	data, err := h.tests.Discover(c.Request.Context(), programmingSessionID(c))
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": data})
}

func (h *ProgrammingHandler) TestTargets(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	data, err := h.tests.Discover(c.Request.Context(), programmingSessionID(c))
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	targets := []types.WorkspaceTestTarget{}
	for _, p := range data.Projects {
		if c.Query("project_id") == "" || c.Query("project_id") == p.ID {
			targets = append(targets, p.Targets...)
		}
	}
	c.JSON(200, gin.H{"success": true, "data": targets})
}

func (h *ProgrammingHandler) StartTestRun(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	var req types.WorkspaceTestRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
	if c.ShouldBindJSON(&req) != nil {
		c.Status(400)
		return
	}
	ctx := c.Request.Context()
	session, _ := h.sessions.GetOwnedSession(ctx, programmingSessionID(c))
	pin := service.SandboxPin{}
	if session != nil && session.SandboxConfigID == "" {
		var agent *types.CustomAgent
		if req.AgentSourceTenantID == 0 && h.previewAgents != nil {
			agent, _ = h.previewAgents.GetAgentByID(ctx, req.AgentID)
		}
		if agent == nil && h.previewShares != nil {
			tenant, _ := types.TenantIDFromContext(ctx)
			agent, _ = h.previewShares.GetSharedAgentForTenant(ctx, tenant, types.TenantRoleFromContext(ctx), req.AgentID, req.AgentSourceTenantID)
		}
		if agent != nil {
			pin = service.SandboxPin{ConfigID: agent.Config.SandboxConfigID, TenantID: agent.TenantID}
		}
	}
	run, err := h.tests.Start(ctx, programmingSessionID(c), req, pin)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, service.ErrWorkspaceTestBusy) {
			status = http.StatusConflict
		}
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"success": true, "data": run})
}

func (h *ProgrammingHandler) TestRuns(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	runs, err := h.tests.List(c.Request.Context(), programmingSessionID(c))
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": runs})
}

func (h *ProgrammingHandler) TestRun(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	run, err := h.tests.Refresh(c.Request.Context(), programmingSessionID(c), c.Param("run_id"))
	if err != nil {
		c.JSON(404, gin.H{"error": "Validation run not found"})
		return
	}
	// Logs are bounded in the guest and in the persisted result. Polling can
	// request a character cursor rather than transferring the tail repeatedly.
	if raw := c.Query("log_cursor"); raw != "" {
		if cursor, err := strconv.Atoi(raw); err == nil && cursor >= 0 && cursor <= len(run.Result.Log) {
			run.Result.Log = run.Result.Log[cursor:]
		}
	}
	c.JSON(200, gin.H{"success": true, "data": run})
}

func (h *ProgrammingHandler) CancelTestRun(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	run, err := h.tests.Cancel(c.Request.Context(), programmingSessionID(c), c.Param("run_id"))
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"success": true, "data": run})
}
