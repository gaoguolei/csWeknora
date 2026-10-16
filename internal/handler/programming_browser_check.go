package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *ProgrammingHandler) RecordBrowserTestCheck(c *gin.Context) {
	if !h.testAvailable(c) {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
	var req struct {
		RunID   string `json:"run_id"`
		Outcome string `json:"outcome"`
		Detail  string `json:"detail"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.Status(400)
		return
	}
	run, err := h.tests.RecordBrowserCheck(c.Request.Context(), programmingSessionID(c), req.RunID, req.Outcome, req.Detail)
	if err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, gin.H{"success": true, "data": run})
}
