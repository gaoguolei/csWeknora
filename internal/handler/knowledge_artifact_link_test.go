package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type artifactLinkMessages struct {
	interfaces.MessageService
	called bool
	err    error
}

func (s *artifactLinkMessages) LinkArtifactKnowledge(_ context.Context, session, message string, index int, knowledge string) error {
	s.called = session == "session" && message == "message" && index == 0 && knowledge == "document"
	return s.err
}

func TestArtifactKnowledgeLinkHTTPValidatesTheSourceAndMapsOwnershipFailures(t *testing.T) {
	for _, mode := range []string{"linked", "missing-index", "not-owner"} {
		t.Run(mode, func(t *testing.T) {
			messages := &artifactLinkMessages{}
			if mode == "not-owner" {
				messages.err = errors.ErrSessionNotFound
			}
			h := &KnowledgeHandler{cfg: transferHandlerConfig(), kgService: &replaceFileKnowledge{}, kbService: transferHandlerKB(), messageService: messages}
			r := transferHandlerRouter(nil)
			r.POST("/knowledge/:id/artifact-link", h.LinkKnowledgeArtifact)
			body := `{"session_id":"session","message_id":"message","index":0}`
			want := http.StatusOK
			if mode == "missing-index" {
				body = `{"session_id":"session","message_id":"message"}`
				want = http.StatusBadRequest
			}
			if mode == "not-owner" {
				want = http.StatusNotFound
			}
			req := httptest.NewRequest(http.MethodPost, "/knowledge/document/artifact-link", strings.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			r.ServeHTTP(response, req)
			require.Equal(t, want, response.Code, response.Body.String())
			require.Equal(t, mode != "missing-index", messages.called)
		})
	}
}
