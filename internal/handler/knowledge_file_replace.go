package handler

import (
	"fmt"
	"net/http"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/utils"
	"github.com/gin-gonic/gin"
)

// ReplaceKnowledgeFile preserves the document identity and reindexes its edited source.
func (h *KnowledgeHandler) ReplaceKnowledgeFile(c *gin.Context) {
	knowledge, _, err := h.resolveKnowledgeAndValidateKBAccess(c, c.Param("id"), types.OrgRoleEditor)
	if err != nil {
		_ = c.Error(err)
		return
	}
	_, tenantID, err := h.requireKnowledgeWriteAccess(c, knowledge.KnowledgeBaseID)
	if err != nil {
		_ = c.Error(err)
		return
	}
	ctx := types.WithExecutionTenant(c.Request.Context(), tenantID)
	maxSizeMB := utils.GetMaxFileSizeMB()
	maxSize := maxSizeMB * 1024 * 1024
	limitUploadBody(c, maxSize)
	file, err := c.FormFile("file")
	if err != nil {
		if isRequestBodyTooLarge(err) {
			_ = c.Error(errors.NewBadRequestError(fmt.Sprintf("文件大小不能超过%dMB", maxSizeMB)))
		} else {
			_ = c.Error(errors.NewBadRequestError("file is required"))
		}
		return
	}
	if file.Size > maxSize {
		_ = c.Error(errors.NewBadRequestError(fmt.Sprintf("文件大小不能超过%dMB", maxSizeMB)))
		return
	}
	updated, err := h.kgService.ReplaceKnowledgeFile(ctx, knowledge.ID, file, knowledge.FileName, nil)
	if err != nil {
		if _, duplicate := err.(*types.DuplicateKnowledgeError); !duplicate {
			_ = c.Error(err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": updated})
}
