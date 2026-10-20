package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/access"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

type replaceFileKnowledge struct {
	interfaces.KnowledgeService
	called    bool
	duplicate bool
	content   []byte
}

func (s *replaceFileKnowledge) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	return &types.Knowledge{ID: "document", TenantID: 7, KnowledgeBaseID: "source", Type: "file", FileName: "cases.xlsx"}, nil
}

func (s *replaceFileKnowledge) ReplaceKnowledgeFile(ctx context.Context, id string, file *multipart.FileHeader, name string, metadata map[string]string) (*types.Knowledge, error) {
	s.called = true
	if !access.HasKBGrant(ctx, "source", 7, types.OrgRoleEditor) {
		return nil, access.ErrForbidden
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	s.content, err = io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	knowledge := &types.Knowledge{ID: id, FileName: name, KnowledgeBaseID: "source", TenantID: 7}
	if s.duplicate {
		return knowledge, types.NewDuplicateFileError(knowledge)
	}
	return knowledge, nil
}

func TestReplaceKnowledgeFileHTTPPreservesIdentityAndWriteGrant(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(map[bool]string{false: "changed", true: "identical"}[duplicate], func(t *testing.T) {
			svc := &replaceFileKnowledge{duplicate: duplicate}
			h := &KnowledgeHandler{cfg: transferHandlerConfig(), kgService: svc, kbService: transferHandlerKB()}
			r := transferHandlerRouter(nil)
			r.POST("/knowledge/:id/file", h.ReplaceKnowledgeFile)
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("file", "other-name.xlsx")
			require.NoError(t, err)
			_, err = part.Write([]byte("edited workbook"))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			request := httptest.NewRequest(http.MethodPost, "/knowledge/document/file", &body)
			request.Header.Set("Content-Type", writer.FormDataContentType())
			response := httptest.NewRecorder()
			r.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			require.True(t, svc.called)
			require.Equal(t, "edited workbook", string(svc.content))
			require.Contains(t, response.Body.String(), `"id":"document"`)
			require.Contains(t, response.Body.String(), `"file_name":"cases.xlsx"`)
		})
	}
}

func TestReplaceKnowledgeFileHTTPRejectsMissingFileAndOutOfScopeKB(t *testing.T) {
	for _, scopedOut := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing file", true: "outside API key scope"}[scopedOut], func(t *testing.T) {
			svc := &replaceFileKnowledge{}
			h := &KnowledgeHandler{cfg: transferHandlerConfig(), kgService: svc, kbService: transferHandlerKB()}
			var scope *types.TenantAPIKeyScope
			want := http.StatusBadRequest
			if scopedOut {
				scope = &types.TenantAPIKeyScope{Capabilities: types.StringArray{"ingest"}, KnowledgeBaseIDs: types.StringArray{"other"}}
				want = http.StatusForbidden
			}
			r := transferHandlerRouter(scope)
			r.POST("/knowledge/:id/file", h.ReplaceKnowledgeFile)
			response := httptest.NewRecorder()
			r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/knowledge/document/file", nil))
			require.Equal(t, want, response.Code, response.Body.String())
			require.False(t, svc.called)
		})
	}
}

func TestReplaceKnowledgeFileHTTPRequiresOwnership(t *testing.T) {
	svc := &replaceFileKnowledge{}
	kb := &stubKBService{get: func(_ context.Context, id string) (*types.KnowledgeBase, error) {
		return &types.KnowledgeBase{ID: id, TenantID: 7, CreatorID: "someone-else"}, nil
	}}
	h := &KnowledgeHandler{cfg: transferHandlerConfig(), kgService: svc, kbService: kb}
	r := transferHandlerRouter(nil)
	r.POST("/knowledge/:id/file", h.ReplaceKnowledgeFile)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/knowledge/document/file", nil))
	require.Equal(t, http.StatusForbidden, response.Code, response.Body.String())
	require.False(t, svc.called)
}
