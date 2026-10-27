package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"

	"github.com/gin-gonic/gin"
)

// listUsersService is a minimal fake UserService for the ListUsers handler.
type listUsersService struct {
	interfaces.UserService
	users              []*types.User
	hasMore            bool
	err                error
	gotOff             int
	gotLim             int
	gotExcludeOpsAdmin bool
}

func (s *listUsersService) ListAllUsers(
	_ context.Context, offset, limit int, excludeOperationsAdmins bool,
) ([]*types.User, bool, error) {
	s.gotOff = offset
	s.gotLim = limit
	s.gotExcludeOpsAdmin = excludeOperationsAdmins
	return s.users, s.hasMore, s.err
}

func listUsersRouter(h *SystemHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/system/admin/users", h.ListUsers)
	return r
}

func TestListUsers_RedactedAndPaginated(t *testing.T) {
	users := []*types.User{
		{ID: "u1", Username: "alice", Email: "alice@example.com", IsActive: true, IsSystemAdmin: true},
		{ID: "u2", Username: "bob", Email: "bob@example.com", IsActive: true, IsSystemAdmin: false},
	}
	svc := &listUsersService{users: users, hasMore: true}
	h := &SystemHandler{userSvc: svc}
	r := listUsersRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/system/admin/users?offset=10&limit=50", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Users   []map[string]any `json:"users"`
		Offset  int              `json:"offset"`
		Limit   int              `json:"limit"`
		HasMore bool             `json:"has_more"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if resp.Offset != 10 || resp.Limit != 50 || !resp.HasMore {
		t.Fatalf("unexpected pagination meta: %+v", resp)
	}
	if len(resp.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Users))
	}
	for _, u := range resp.Users {
		if _, ok := u["password"]; ok {
			t.Fatalf("password leaked: %v", u)
		}
		if _, ok := u["Password"]; ok {
			t.Fatalf("Password leaked: %v", u)
		}
	}
	if svc.gotOff != 10 || svc.gotLim != 50 {
		t.Fatalf("service got wrong args: off=%d lim=%d", svc.gotOff, svc.gotLim)
	}
	// No operations-admin marker on the request context: the caller is a super
	// admin, so system-administrator rows must be filtered out of the page.
	if !svc.gotExcludeOpsAdmin {
		t.Fatal("super admin caller must hide system administrator rows")
	}
}

func TestListUsers_SystemAdminSeesEveryRole(t *testing.T) {
	svc := &listUsersService{}
	h := &SystemHandler{userSvc: svc}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.OperationsAdminContextKey, true)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.GET("/system/admin/users", h.ListUsers)

	req := httptest.NewRequest(http.MethodGet, "/system/admin/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if svc.gotExcludeOpsAdmin {
		t.Fatal("system administrator caller must see every role")
	}
}

func TestListUsers_LimitCapped(t *testing.T) {
	svc := &listUsersService{users: nil, hasMore: false}
	h := &SystemHandler{userSvc: svc}
	r := listUsersRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/system/admin/users?limit=9999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if svc.gotLim != 200 {
		t.Fatalf("expected limit capped to 200, got %d", svc.gotLim)
	}
}
