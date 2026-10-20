package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestListMessageArtifactsReturnsKnowledgeLinkWithoutStorageURL(t *testing.T) {
	h := &Handler{
		sessionService: &stubSessionServiceForArtifacts{getSession: func(context.Context, string) (*types.Session, error) {
			return &types.Session{ID: "session", TenantID: 42}, nil
		}},
		messageService: &stubMessageServiceForArtifacts{getMessage: func(context.Context, string, string) (*types.Message, error) {
			return &types.Message{Artifacts: types.MessageArtifacts{{FileName: "cases.xlsx", URL: "private://source", KnowledgeID: "knowledge", KnowledgeBaseID: "kb"}}}, nil
		}},
	}
	response := httptest.NewRecorder()
	newArtifactTestRouter(h).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/sessions/session/messages/message/artifacts", nil))
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), `"knowledge_id":"knowledge"`)
	require.Contains(t, response.Body.String(), `"knowledge_base_id":"kb"`)
	require.NotContains(t, response.Body.String(), "private://source")
}
