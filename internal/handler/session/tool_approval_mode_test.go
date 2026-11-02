package session

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/stream"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type permissionSessionStub struct {
	interfaces.SessionService
	denied bool
	saved  *types.SessionLastRequestState
}

func (s *permissionSessionStub) GetOwnedSession(context.Context, string) (*types.Session, error) {
	if s.denied {
		return nil, errors.New("not owner")
	}
	return &types.Session{LastRequestState: &types.SessionLastRequestState{AgentID: "agent", ToolApprovalMode: types.ToolApprovalAuto}}, nil
}
func (s *permissionSessionStub) UpdateSessionLastRequestState(_ context.Context, _ string, state *types.SessionLastRequestState) error {
	s.saved = state
	return nil
}

func TestLiveToolApprovalModeEndpointWaitsForWorker(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr := stream.NewMemoryStreamManager()
	require.NoError(t, mgr.SetLiveRun(ctx, "s", "a", "request"))
	h := &Handler{sessionService: &permissionSessionStub{}, messageService: &steerMessageLookupStub{msg: &types.Message{ID: "a", SessionID: "s"}}, streamManager: mgr}
	policy := types.NewToolApprovalPolicy(types.ToolApprovalAuto)
	h.startStopWatcher(ctx, "s", "a", event.NewEventBus(), func(_ context.Context, mode types.ToolApprovalMode) { policy.Set(mode) })
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.PUT("/sessions/:id/tool-approval-mode", h.UpdateToolApprovalMode)
	response := httptest.NewRecorder()
	request := httptest.NewRequest("PUT", "/sessions/s/tool-approval-mode", bytes.NewBufferString(`{"mode":"manual","expected_assistant_message_id":"a"}`))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code, response.Body.String())
	require.Equal(t, types.ToolApprovalManual, policy.Current(), "success must mean the running worker applied it")
	require.Contains(t, response.Body.String(), `"applied":true`)
}

func TestLiveToolApprovalModeValidationOwnershipAndTurnScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		denied     bool
		live       bool
		status     int
	}{
		{"invalid", `{"mode":"unrestricted"}`, false, false, 400},
		{"non-owner", `{"mode":"auto"}`, true, false, 404},
		{"stale-turn", `{"mode":"manual","expected_assistant_message_id":"old"}`, false, true, 409},
		{"idle", `{"mode":"manual"}`, false, false, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mgr := stream.NewMemoryStreamManager()
			if tc.live {
				require.NoError(t, mgr.SetLiveRun(context.Background(), "s", "a", "r"))
			}
			sessions := &permissionSessionStub{denied: tc.denied}
			h := &Handler{sessionService: sessions, messageService: &steerMessageLookupStub{msg: &types.Message{ID: "a", SessionID: "s"}}, streamManager: mgr}
			router := gin.New()
			router.Use(middleware.ErrorHandler())
			router.PUT("/sessions/:id/tool-approval-mode", h.UpdateToolApprovalMode)
			response := httptest.NewRecorder()
			request := httptest.NewRequest("PUT", "/sessions/s/tool-approval-mode", bytes.NewBufferString(tc.body))
			request.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(response, request)
			require.Equal(t, tc.status, response.Code, response.Body.String())
			if tc.name == "idle" {
				require.Equal(t, types.ToolApprovalManual, sessions.saved.ToolApprovalMode)
				require.Equal(t, "agent", sessions.saved.AgentID)
			}
		})
	}
}

func TestLivePolicyIsSharedWithQAAndInheritedByFollowUp(t *testing.T) {
	policy := types.NewToolApprovalPolicy(types.ToolApprovalAuto)
	rc := &qaRequestContext{assistantMessage: &types.Message{ID: "a"}, toolApprovalMode: types.ToolApprovalAuto, toolApprovalPolicy: policy}
	request := rc.buildQARequest()
	policy.Set(types.ToolApprovalManual)
	require.Equal(t, types.ToolApprovalManual, request.ToolApprovalPolicy.Current())
	require.Equal(t, types.ToolApprovalManual, rc.currentToolApprovalMode())
}
