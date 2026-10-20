package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// Only the source download is supplied by the fixture. Saving executes the
// real application replacement, storage, source update and parsing pipeline.
type agentFileSourceFixture struct {
	interfaces.KnowledgeService
	h *replaceFileHarness
}

func (s *agentFileSourceFixture) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	copy := s.h.repo.row
	return &copy, nil
}

func (s *agentFileSourceFixture) GetKnowledgeFile(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader(replaceFileOldContent)), s.h.original.FileName, nil
}

func TestKnowledgeFileAgentToolSavesThroughTheApplicationPipeline(t *testing.T) {
	h := newReplaceFileHarness(t)
	source := &agentFileSourceFixture{KnowledgeService: h.svc, h: h}
	tool := tools.NewEditKnowledgeFileTool(source, types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledge, KnowledgeBaseID: h.original.KnowledgeBaseID,
		TenantID: 7, KnowledgeIDs: []string{h.original.ID},
	}}, []string{h.original.KnowledgeBaseID}, func(context.Context, *types.Knowledge) (context.Context, error) { return h.ctx, nil })
	args, err := json.Marshal(map[string]interface{}{
		"knowledge_id": h.original.ID, "action": "edit",
		"sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(replaceFileOldContent))),
		"edits":  []map[string]string{{"old_string": "old", "new_string": "updated by conversation"}},
	})
	require.NoError(t, err)
	registry := tools.NewToolRegistry()
	registry.RegisterTool(tool)
	result, err := registry.ExecuteTool(h.ctx, tool.Name(), args)
	require.NoError(t, err)
	require.True(t, result.Success, result.Error)
	require.Equal(t, h.original.ID, h.repo.row.ID)
	require.Equal(t, h.original.FileName, h.repo.row.FileName)
	require.Equal(t, h.original.FolderPath, h.repo.row.FolderPath)
	require.Equal(t, md5Hex("updated by conversation"), h.repo.row.FileHash)
	require.Equal(t, types.ParseStatusPending, h.repo.row.ParseStatus)
	require.Len(t, h.tasks.payloads, 1)
	require.Equal(t, h.original.ID, h.tasks.payloads[0].KnowledgeID)
	require.Equal(t, []string{"save", "dequeue:knowledge-1", "enqueue", "delete:old/file.md"}, h.events)
}
