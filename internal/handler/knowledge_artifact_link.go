package handler

import (
	goerrors "errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

func (h *KnowledgeHandler) LinkKnowledgeArtifact(c *gin.Context) {
	var request struct {
		SessionID string `json:"session_id" binding:"required"`
		MessageID string `json:"message_id" binding:"required"`
		Index     *int   `json:"index" binding:"required"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Index == nil || *request.Index < 0 {
		_ = c.Error(errors.NewBadRequestError("session_id, message_id and a valid index are required"))
		return
	}
	knowledge, _, err := h.resolveKnowledgeAndValidateKBAccess(c, c.Param("id"), types.OrgRoleEditor)
	if err != nil {
		_ = c.Error(err)
		return
	}
	if _, _, err := h.requireKnowledgeWriteAccess(c, knowledge.KnowledgeBaseID); err != nil {
		_ = c.Error(err)
		return
	}
	if err := h.messageService.LinkArtifactKnowledge(c.Request.Context(), request.SessionID, request.MessageID, *request.Index, knowledge.ID); err != nil {
		if goerrors.Is(err, errors.ErrSessionNotFound) {
			err = errors.NewNotFoundError("session not found")
		}
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"knowledge_id": knowledge.ID, "knowledge_base_id": knowledge.KnowledgeBaseID}})
}
