package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// roleChangeService is a minimal fake UserService for the SetUserRole handler.
type roleChangeService struct {
	interfaces.UserService

	target *types.User

	updated    *types.User
	revokedSys *types.User
	revokedOps *types.User

	revokeSystemErr error
	revokeOpsErr    error
	updateErr       error
}

func (s *roleChangeService) GetUserByID(context.Context, string) (*types.User, error) {
	return s.target, nil
}

func (s *roleChangeService) UpdateUser(_ context.Context, user *types.User) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = user
	return nil
}

func (s *roleChangeService) RevokeSystemAdmin(_ context.Context, _, _ string) (*types.User, error) {
	if s.revokeSystemErr != nil {
		return nil, s.revokeSystemErr
	}
	s.target.IsSystemAdmin = false
	s.revokedSys = s.target
	return s.target, nil
}

func (s *roleChangeService) RevokeOperationsAdmin(_ context.Context, _, _ string) (*types.User, error) {
	if s.revokeOpsErr != nil {
		return nil, s.revokeOpsErr
	}
	s.target.IsOperationsAdmin = false
	s.revokedOps = s.target
	return s.target, nil
}

func roleChangeRouter(h *SystemHandler, callerID string, callerIsOperationsAdmin bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, callerID)
		if callerIsOperationsAdmin {
			ctx = context.WithValue(ctx, types.OperationsAdminContextKey, true)
		}
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.POST("/system/admin/users/role", h.SetUserRole)
	return r
}

func doRoleChange(r *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/system/admin/users/role", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestSetUserRole_RejectsSelfChange(t *testing.T) {
	svc := &roleChangeService{target: &types.User{ID: "me", Username: "me"}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "me", false)

	w := doRoleChange(r, `{"user_id":"me","role":"user"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Nil(t, svc.updated)
	require.Nil(t, svc.revokedSys)
	require.Nil(t, svc.revokedOps)
}

func TestSetUserRole_RejectsUnknownRole(t *testing.T) {
	svc := &roleChangeService{target: &types.User{ID: "u1", Username: "bob"}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "admin", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"root"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Nil(t, svc.updated)
}

func TestSetUserRole_SuperAdminCannotGrantSystemAdmin(t *testing.T) {
	svc := &roleChangeService{target: &types.User{ID: "u1", Username: "bob"}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"system_admin"}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Nil(t, svc.updated)
}

func TestSetUserRole_SuperAdminCannotTouchSystemAdminRow(t *testing.T) {
	svc := &roleChangeService{target: &types.User{
		ID: "u1", Username: "bob", IsOperationsAdmin: true,
	}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"user"}`)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Nil(t, svc.revokedOps)
	require.Nil(t, svc.revokedSys)
}

func TestSetUserRole_SuperAdminPromotesPeer(t *testing.T) {
	svc := &roleChangeService{target: &types.User{ID: "u1", Username: "bob"}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"super_admin"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.updated)
	require.True(t, svc.updated.IsSystemAdmin)
	require.False(t, svc.updated.IsOperationsAdmin)
}

func TestSetUserRole_SystemAdminPromotesUser(t *testing.T) {
	svc := &roleChangeService{target: &types.User{ID: "u1", Username: "bob"}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "ops", true)

	w := doRoleChange(r, `{"user_id":"u1","role":"system_admin"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.updated)
	require.True(t, svc.updated.IsOperationsAdmin)
}

func TestSetUserRole_DemotesSuperAdminToUser(t *testing.T) {
	svc := &roleChangeService{target: &types.User{
		ID: "u1", Username: "bob", IsSystemAdmin: true,
	}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"user"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.revokedSys)
	require.Nil(t, svc.updated)
}

func TestSetUserRole_SystemAdminDemotesPeerToSuperAdmin(t *testing.T) {
	svc := &roleChangeService{target: &types.User{
		ID: "u1", Username: "bob", IsOperationsAdmin: true,
	}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "ops", true)

	w := doRoleChange(r, `{"user_id":"u1","role":"super_admin"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.revokedOps)
	require.NotNil(t, svc.updated)
	require.True(t, svc.updated.IsSystemAdmin)
	require.False(t, svc.updated.IsOperationsAdmin)
}

func TestSetUserRole_LastAdminGuard(t *testing.T) {
	svc := &roleChangeService{
		target:          &types.User{ID: "u1", Username: "bob", IsSystemAdmin: true},
		revokeSystemErr: repository.ErrLastSystemAdmin,
	}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"user"}`)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Nil(t, svc.updated)
}

func TestSetUserRole_Idempotent(t *testing.T) {
	svc := &roleChangeService{target: &types.User{
		ID: "u1", Username: "bob", IsSystemAdmin: true,
	}}
	h := &SystemHandler{userSvc: svc}
	r := roleChangeRouter(h, "super", false)

	w := doRoleChange(r, `{"user_id":"u1","role":"super_admin"}`)

	require.Equal(t, http.StatusOK, w.Code)
	require.Nil(t, svc.updated)
	require.Nil(t, svc.revokedSys)
	require.Nil(t, svc.revokedOps)
}
