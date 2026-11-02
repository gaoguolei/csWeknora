package session

import (
	"context"
	"time"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// UpdateToolApprovalMode uses the same owner scope and distributed control
// stream as StopSession. Success for a live turn requires acknowledgement from
// its worker; a setting must not claim to be active while tools use the old one.
func (h *Handler) UpdateToolApprovalMode(c *gin.Context) {
	ctx := c.Request.Context()
	sessionID := c.Param("id")
	var request struct {
		Mode                       types.ToolApprovalMode `json:"mode"`
		ExpectedAssistantMessageID string                 `json:"expected_assistant_message_id"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Mode == "" || !request.Mode.Valid() {
		_ = c.Error(apperrors.NewBadRequestError("invalid tool approval mode"))
		return
	}
	session, err := h.sessionService.GetOwnedSession(ctx, sessionID)
	if err != nil {
		_ = c.Error(apperrors.NewNotFoundError("Session not found"))
		return
	}
	assistantID, err := h.liveAgentRun(ctx, sessionID)
	if err != nil {
		_ = c.Error(apperrors.NewServiceUnavailableError("Failed to look up active turn"))
		return
	}
	if assistantID != "" && request.ExpectedAssistantMessageID != "" && assistantID != request.ExpectedAssistantMessageID {
		_ = c.Error(apperrors.NewConflictError("The active turn changed; retry the permission change"))
		return
	}
	if assistantID == "" {
		if session.LastRequestState != nil {
			state := *session.LastRequestState
			state.ToolApprovalMode = request.Mode
			if err := h.sessionService.UpdateSessionLastRequestState(ctx, sessionID, &state); err != nil {
				_ = c.Error(apperrors.NewServiceUnavailableError("Failed to save permissions"))
				return
			}
		}
		c.JSON(200, gin.H{"success": true, "data": gin.H{"mode": request.Mode, "applied": false}})
		return
	}
	id := uuid.NewString()
	if err := h.streamManager.AppendEvent(ctx, sessionID, assistantID, interfaces.StreamEvent{
		ID: id, Type: types.ResponseTypeToolApprovalMode, Timestamp: time.Now(),
		Data: map[string]interface{}{"tool_approval_mode": string(request.Mode)},
	}); err != nil {
		_ = c.Error(apperrors.NewServiceUnavailableError("Failed to update permissions"))
		return
	}
	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	offset := 0
	for {
		events, next, err := h.streamManager.GetEvents(waitCtx, sessionID, assistantID, offset)
		if err == nil {
			offset = next
			for _, evt := range events {
				if evt.ID == id+"-applied" && evt.Type == types.ResponseTypeToolApprovalModeApplied {
					c.JSON(200, gin.H{"success": true, "data": gin.H{"mode": request.Mode, "applied": true}})
					return
				}
			}
		}
		select {
		case <-waitCtx.Done():
			_ = c.Error(apperrors.NewServiceUnavailableError("Permission update was not acknowledged; retry"))
			return
		case <-ticker.C:
		}
	}
}
