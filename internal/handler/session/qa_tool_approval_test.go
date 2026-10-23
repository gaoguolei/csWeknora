package session

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestQAToolApprovalValidationAndRestoration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []string{"knowledge-chat", "agent-chat"} {
		for _, mode := range []types.ToolApprovalMode{"", types.ToolApprovalAuto, types.ToolApprovalManual, "unknown"} {
			t.Run(endpoint+"/"+string(mode), func(t *testing.T) {
				sessions := &queryValidationSessionStub{}
				h := &Handler{sessionService: sessions}
				router := gin.New()
				router.Use(middleware.ErrorHandler())
				router.POST("/knowledge-chat/:session_id", h.KnowledgeQA)
				router.POST("/agent-chat/:session_id", h.AgentQA)
				body, err := json.Marshal(CreateKnowledgeQARequest{Query: "question", ToolApprovalMode: mode})
				require.NoError(t, err)
				request := httptest.NewRequest(http.MethodPost, "/"+endpoint+"/session", bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				response := httptest.NewRecorder()
				router.ServeHTTP(response, request)
				if mode == "unknown" {
					require.Equal(t, http.StatusBadRequest, response.Code)
					require.Contains(t, response.Body.String(), "tool_approval_mode")
					require.Zero(t, sessions.lookupCalls)
					return
				}
				require.Equal(t, 1, sessions.lookupCalls)
				rc := &qaRequestContext{sessionID: "session", session: &types.Session{ID: "session"}, assistantMessage: &types.Message{ID: "answer"}, toolApprovalMode: mode}
				require.Equal(t, mode, rc.buildQARequest().ToolApprovalMode)
				store := &reasoningSessionStub{}
				(&Handler{sessionService: store}).persistLastRequestState(context.Background(), rc, qaModeAgent)
				raw, err := store.state.Value()
				require.NoError(t, err)
				var restored types.SessionLastRequestState
				require.NoError(t, restored.Scan(raw))
				require.Equal(t, mode, restored.ToolApprovalMode)
			})
		}
	}
}
