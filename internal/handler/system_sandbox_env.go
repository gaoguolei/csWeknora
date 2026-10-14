package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
)

// systemSandboxEnvService is the super-admin-facing slice of the global sandbox
// secret surface. Unlike the /me endpoints these operate on one platform-wide
// store shared by every workspace, so no identity or tenant is derived from the
// request; the routes are gated to SystemAdmin.
type systemSandboxEnvService interface {
	ListGlobal(ctx context.Context) ([]service.GlobalEnvView, error)
	SetGlobal(ctx context.Context, name, value string) error
	DeleteGlobal(ctx context.Context, name string) error
}

// SystemSandboxEnvHandler serves the /system/admin/sandbox-env endpoints.
//
// It returns name + set/unset + updated_at only — never a value. Writes store
// the value encrypted server-side; a read-back would defeat the point of a
// secret store, so there is none.
type SystemSandboxEnvHandler struct {
	service systemSandboxEnvService
}

func NewSystemSandboxEnvHandler(s *service.UserEnvService) *SystemSandboxEnvHandler {
	return &SystemSandboxEnvHandler{service: s}
}

// systemSandboxEnvRequest is the write body. It carries a name and, for a set,
// the plaintext value; the value is encrypted before it is persisted.
type systemSandboxEnvRequest struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// List godoc
// @Summary      List global sandbox secrets
// @Description  List every platform-wide sandbox/skill secret the super admin set, reporting only the name, whether a value is stored, and when it last changed. Values are never returned.
// @Tags         SystemAdmin
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "Global secret views"
// @Failure      403  {object}  apperrors.AppError      "Forbidden"
// @Security     Bearer
// @Router       /system/admin/sandbox-env [get]
func (h *SystemSandboxEnvHandler) List(c *gin.Context) {
	views, err := h.service.ListGlobal(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": views})
}

// Set godoc
// @Summary      Set a global sandbox secret
// @Description  Store or rotate one platform-wide secret. It is encrypted at rest and injected into every workspace's skill and sandbox runs, overriding nothing the members can set.
// @Tags         SystemAdmin
// @Accept       json
// @Produce      json
// @Param        request  body      systemSandboxEnvRequest  true  "Name and value"
// @Success      200      {object}  map[string]interface{}   "Stored"
// @Failure      400      {object}  apperrors.AppError       "Invalid request"
// @Failure      403      {object}  apperrors.AppError       "Forbidden"
// @Security     Bearer
// @Router       /system/admin/sandbox-env [put]
func (h *SystemSandboxEnvHandler) Set(c *gin.Context) {
	var req systemSandboxEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.SetGlobal(c.Request.Context(), req.Name, req.Value); err != nil {
		respondEnvVarError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Delete godoc
// @Summary      Delete a global sandbox secret
// @Description  Remove one platform-wide secret. After this, no value is injected for that name unless a container-baked variable already carries it.
// @Tags         SystemAdmin
// @Accept       json
// @Produce      json
// @Param        request  body      systemSandboxEnvRequest  true  "Name"
// @Success      200      {object}  map[string]interface{}   "Deleted"
// @Failure      404      {object}  apperrors.AppError       "Not set"
// @Failure      403      {object}  apperrors.AppError       "Forbidden"
// @Security     Bearer
// @Router       /system/admin/sandbox-env [delete]
func (h *SystemSandboxEnvHandler) Delete(c *gin.Context) {
	var req systemSandboxEnvRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	if err := h.service.DeleteGlobal(c.Request.Context(), req.Name); err != nil {
		respondEnvVarError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
