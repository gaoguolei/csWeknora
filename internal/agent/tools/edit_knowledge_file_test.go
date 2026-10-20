package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/modelcontext"
	"github.com/Tencent/WeKnora/internal/sandbox"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

type editableKnowledgeService struct {
	interfaces.KnowledgeService
	knowledge *types.Knowledge
	content   []byte
	reads     int
	writes    int
	writeErr  error
}

func (s *editableKnowledgeService) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	if s.knowledge.ID != id {
		return nil, errors.New("not found")
	}
	return s.knowledge, nil
}

func (s *editableKnowledgeService) GetKnowledgeFile(ctx context.Context, id string) (io.ReadCloser, string, error) {
	if types.MustTenantIDFromContext(ctx) != s.knowledge.TenantID || id != s.knowledge.ID {
		return nil, "", errors.New("incorrect resource tenant")
	}
	s.reads++
	return io.NopCloser(bytes.NewReader(s.content)), s.knowledge.FileName, nil
}

func (s *editableKnowledgeService) ReplaceKnowledgeFile(ctx context.Context, id string, file *multipart.FileHeader,
	filename string, _ map[string]string,
) (*types.Knowledge, error) {
	if s.writeErr != nil {
		return nil, s.writeErr
	}
	if types.MustTenantIDFromContext(ctx) != s.knowledge.TenantID || id != s.knowledge.ID || filename != s.knowledge.FileName {
		return nil, errors.New("incorrect replacement target")
	}
	reader, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = reader.Close() }()
	content, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	s.content = content
	s.writes++
	s.knowledge.ParseStatus = types.ParseStatusPending
	return s.knowledge, nil
}

func knowledgeEditorFixture(filename string, content []byte) (*EditKnowledgeFileTool, *editableKnowledgeService) {
	svc := &editableKnowledgeService{
		knowledge: &types.Knowledge{ID: "doc", TenantID: 7, KnowledgeBaseID: "kb", Type: "file", FileName: filename},
		content:   content,
	}
	tool := NewEditKnowledgeFileTool(svc, types.SearchTargets{{
		Type: types.SearchTargetTypeKnowledgeBase, KnowledgeBaseID: "kb", TenantID: 7,
	}}, []string{"kb"}, func(ctx context.Context, k *types.Knowledge) (context.Context, error) {
		return types.WithExecutionTenant(ctx, k.TenantID), nil
	})
	return tool, svc
}

func executeKnowledgeEdit(t *testing.T, tool *EditKnowledgeFileTool, input editKnowledgeFileInput) *types.ToolResult {
	t.Helper()
	input.KnowledgeID = "doc"
	args, err := json.Marshal(input)
	require.NoError(t, err)
	registry := NewToolRegistry()
	registry.RegisterTool(tool)
	result, err := registry.ExecuteTool(t.Context(), ToolEditKnowledgeFile, args)
	require.NoError(t, err)
	return result
}

func TestEditKnowledgeFileTextPreservesIdentityAndStartsIndexing(t *testing.T) {
	tool, svc := knowledgeEditorFixture("cases.csv", []byte("编号,优先级\nTC-1,P2\n"))
	read := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "read"})
	require.True(t, read.Success, read.Error)
	require.Equal(t, string(svc.content), read.Data["content"])
	updated := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{
		Action: "edit", SHA256: read.Data["sha256"].(string),
		Edits: []SandboxEdit{{OldString: "TC-1,P2", NewString: "TC-1,P1"}},
	})
	require.True(t, updated.Success, updated.Error)
	require.Equal(t, "编号,优先级\nTC-1,P1\n", string(svc.content))
	require.Equal(t, 1, svc.writes)
	require.Equal(t, "doc", updated.Data["knowledge_id"])
	require.Equal(t, types.ParseStatusPending, updated.Data["parse_status"])
	require.Equal(t, true, updated.Data["changed"])
}

func TestEditKnowledgeFileRejectsVersionConflictsAndInvalidMatches(t *testing.T) {
	for _, test := range []struct {
		name string
		sha  string
		edit SandboxEdit
	}{
		{"missing version", "", SandboxEdit{OldString: "P2", NewString: "P1"}},
		{"stale version", knowledgeFileSHA256([]byte("old")), SandboxEdit{OldString: "P2", NewString: "P1"}},
		{"missing match", knowledgeFileSHA256([]byte("P2 P2")), SandboxEdit{OldString: "P3", NewString: "P1"}},
		{"ambiguous match", knowledgeFileSHA256([]byte("P2 P2")), SandboxEdit{OldString: "P2", NewString: "P1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			tool, svc := knowledgeEditorFixture("cases.txt", []byte("P2 P2"))
			result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "edit", SHA256: test.sha, Edits: []SandboxEdit{test.edit}})
			require.False(t, result.Success)
			require.Zero(t, svc.writes)
			require.Equal(t, "P2 P2", string(svc.content))
		})
	}
}

func TestEditKnowledgeFileHonorsDocumentScopeAndLiveWritePermission(t *testing.T) {
	for _, mode := range []string{"no scope", "other KB", "other document", "read only", "permission revoked", "no authorizer"} {
		t.Run(mode, func(t *testing.T) {
			tool, svc := knowledgeEditorFixture("cases.txt", []byte("original"))
			switch mode {
			case "no scope":
				tool.targets = nil
			case "other KB":
				tool.targets[0].KnowledgeBaseID = "other"
			case "other document":
				tool.targets[0].KnowledgeIDs = []string{"other"}
			case "read only":
				tool.writableKBIDs = nil
			case "permission revoked":
				tool.authorize = func(ctx context.Context, _ *types.Knowledge) (context.Context, error) {
					return ctx, errors.New("permission revoked")
				}
			case "no authorizer":
				tool.authorize = nil
			}
			result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "read"})
			require.False(t, result.Success)
			require.Zero(t, svc.reads, "even the original file cannot be downloaded outside the authorized scope")
			require.Zero(t, svc.writes)
		})
	}
}

func knowledgeWorkbookFixture(t *testing.T) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	require.NoError(t, book.SetCellStr("Sheet1", "A1", "编号"))
	require.NoError(t, book.SetCellStr("Sheet1", "B1", "优先级"))
	require.NoError(t, book.SetCellStr("Sheet1", "A2", "TC-1"))
	require.NoError(t, book.SetCellStr("Sheet1", "B2", "P2"))
	require.NoError(t, book.SetCellFormula("Sheet1", "C2", "1+1"))
	style, err := book.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	require.NoError(t, err)
	require.NoError(t, book.SetCellStyle("Sheet1", "B2", "B2", style))
	_, err = book.NewSheet("说明")
	require.NoError(t, err)
	require.NoError(t, book.SetCellStr("说明", "A1", "Preserve this sheet"))
	buffer, err := book.WriteToBuffer()
	require.NoError(t, err)
	return buffer.Bytes()
}

func TestEditKnowledgeFileXLSXPreservesOtherSheetsStylesAndFormulas(t *testing.T) {
	original := knowledgeWorkbookFixture(t)
	tool, svc := knowledgeEditorFixture("cases.xlsx", original)
	read := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "read", Offset: 1, Limit: 1})
	require.True(t, read.Success, read.Error)
	require.Equal(t, []string{"Sheet1", "说明"}, read.Data["sheets"])
	rows := read.Data["rows"].([]map[string]interface{})
	require.Equal(t, 2, rows[0]["row"])
	require.Equal(t, "P2", rows[0]["values"].([]string)[1])
	require.Equal(t, "=1+1", rows[0]["formulas"].(map[string]string)["C2"])
	result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{
		Action: "edit", SHA256: read.Data["sha256"].(string),
		Cells: []KnowledgeCellEdit{{Sheet: "Sheet1", Cell: "B2", OldValue: "P2", NewValue: "P1"},
			{Sheet: "Sheet1", Cell: "A3", OldValue: "", NewValue: "TC-2"}},
	})
	require.True(t, result.Success, result.Error)
	before, err := openKnowledgeWorkbook(original)
	require.NoError(t, err)
	defer func() { _ = before.Close() }()
	after, err := openKnowledgeWorkbook(svc.content)
	require.NoError(t, err)
	defer func() { _ = after.Close() }()
	value, err := after.GetCellValue("Sheet1", "B2")
	require.NoError(t, err)
	require.Equal(t, "P1", value)
	value, err = after.GetCellValue("Sheet1", "A3")
	require.NoError(t, err)
	require.Equal(t, "TC-2", value)
	formula, err := after.GetCellFormula("Sheet1", "C2")
	require.NoError(t, err)
	require.Equal(t, "1+1", formula)
	oldStyle, err := before.GetCellStyle("Sheet1", "B2")
	require.NoError(t, err)
	newStyle, err := after.GetCellStyle("Sheet1", "B2")
	require.NoError(t, err)
	require.Equal(t, oldStyle, newStyle)
	value, err = after.GetCellValue("说明", "A1")
	require.NoError(t, err)
	require.Equal(t, "Preserve this sheet", value)
}

func TestEditKnowledgeFileXLSXInvalidCellLeavesOriginalUntouched(t *testing.T) {
	for _, invalid := range []KnowledgeCellEdit{
		{Sheet: "Sheet1", Cell: "B3", OldValue: "not present", NewValue: "value"},
		{Sheet: "Sheet1", Cell: "B2", OldValue: "P1", NewValue: "again"},
		{Sheet: "missing", Cell: "A1", OldValue: "", NewValue: "value"},
	} {
		original := knowledgeWorkbookFixture(t)
		tool, svc := knowledgeEditorFixture("cases.xlsx", original)
		result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "edit", SHA256: knowledgeFileSHA256(original),
			Cells: []KnowledgeCellEdit{{Sheet: "Sheet1", Cell: "B2", OldValue: "P2", NewValue: "P1"}, invalid},
		})
		require.False(t, result.Success)
		require.Zero(t, svc.writes)
		require.Equal(t, original, svc.content)
	}
}

func TestEditKnowledgeFileBinarySandboxRoundTripDoesNotOverwriteWorkingCopy(t *testing.T) {
	tool, svc := knowledgeEditorFixture("cases.docx", []byte("original binary\x00"))
	store := &fakeSandboxFileSink{}
	tool.WithSandbox(store, "session")
	read := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "read"})
	require.True(t, read.Success, read.Error)
	filePath := read.Data["source_path"].(string)
	replacement := []byte("edited binary\x00")
	store.files[filePath] = replacement
	readAgain := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "read"})
	require.True(t, readAgain.Success)
	require.Equal(t, replacement, store.files[filePath], "reading again must preserve sandbox work")
	result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "replace_file",
		SHA256: read.Data["sha256"].(string), SourcePath: filePath,
	})
	require.True(t, result.Success, result.Error)
	require.Equal(t, replacement, svc.content)
	require.Equal(t, 1, svc.writes)
}

type knowledgeWorkspaceStore struct {
	fakeSandboxFileSink
}

func (*knowledgeWorkspaceStore) ListSessionFiles(context.Context, string, string) ([]sandbox.RemoteDirEntry, error) {
	return nil, nil
}

func TestEditKnowledgeFileSandboxPathSurvivesModelHandlesAndWritesBack(t *testing.T) {
	const knowledgeID = "11111111-2222-4333-8444-555555555555"
	tool, svc := knowledgeEditorFixture("cases.md", []byte("original source"))
	svc.knowledge.ID = knowledgeID
	store := &knowledgeWorkspaceStore{}
	tool.WithSandbox(store, "session")
	registry := modelcontext.NewRegistry(true)
	registry.RegisterDocument("another-document")
	require.Equal(t, "d2", registry.RegisterDocument(knowledgeID))
	readArgs, err := json.Marshal(editKnowledgeFileInput{KnowledgeID: knowledgeID, Action: "read"})
	require.NoError(t, err)
	read, err := tool.Execute(t.Context(), readArgs)
	require.NoError(t, err)
	require.True(t, read.Success, read.Error)
	actualPath := read.Data["source_path"].(string)

	// Test the exact result that reaches the model, not just the unencoded
	// ToolResult. Previously the UUID inside this path became a fake /d2/.
	modelOutput := registry.ModelToolResultForTool(ToolEditKnowledgeFile, read)
	var modelData struct {
		SourcePath string `json:"source_path"`
	}
	require.NoError(t, json.Unmarshal([]byte(modelOutput[strings.IndexByte(modelOutput, '{'):]), &modelData))
	require.Equal(t, actualPath, modelData.SourcePath)
	require.NotContains(t, actualPath, knowledgeID)
	require.NotContains(t, modelData.SourcePath, "/d2/")

	ctx := WithToolExecContext(t.Context(), &ToolExecContext{SessionID: "session"})
	reader := NewReadFileTool(store)
	reader.BindSession("session")
	fileArgs, err := json.Marshal(ReadFileInput{Path: modelData.SourcePath})
	require.NoError(t, err)
	fileRead, err := reader.Execute(ctx, fileArgs)
	require.NoError(t, err)
	require.True(t, fileRead.Success, fileRead.Error)
	require.Contains(t, fileRead.Output, "original source")

	store.files[modelData.SourcePath] = []byte("edited in the sandbox")
	writeArgs, err := json.Marshal(editKnowledgeFileInput{
		KnowledgeID: "d2", Action: "replace_file", SHA256: read.Data["sha256"].(string), SourcePath: modelData.SourcePath,
	})
	require.NoError(t, err)
	calls := []types.LLMToolCall{{Function: types.FunctionCall{Name: ToolEditKnowledgeFile, Arguments: string(writeArgs)}}}
	registry.DecodeToolCalls(calls)
	written, err := tool.Execute(ctx, json.RawMessage(calls[0].Function.Arguments))
	require.NoError(t, err)
	require.True(t, written.Success, written.Error)
	require.Equal(t, "edited in the sandbox", string(svc.content))
	require.Equal(t, knowledgeID, written.Data["knowledge_id"])
	require.Equal(t, 1, svc.writes)
}

func TestEditKnowledgeFileDoesNotReportFailedSaveAsSuccess(t *testing.T) {
	tool, svc := knowledgeEditorFixture("cases.txt", []byte("old"))
	svc.writeErr = errors.New("storage unavailable")
	result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "edit", SHA256: knowledgeFileSHA256(svc.content),
		Edits: []SandboxEdit{{OldString: "old", NewString: "new"}},
	})
	require.False(t, result.Success)
	require.Contains(t, result.Error, "file was not saved")
	require.Zero(t, svc.writes)
	require.Equal(t, "old", string(svc.content))
}

func TestEditKnowledgeFileRechecksPermissionImmediatelyBeforeSave(t *testing.T) {
	tool, svc := knowledgeEditorFixture("cases.txt", []byte("old"))
	calls := 0
	tool.authorize = func(ctx context.Context, k *types.Knowledge) (context.Context, error) {
		calls++
		if calls == 2 {
			return ctx, errors.New("write permission revoked")
		}
		return types.WithExecutionTenant(ctx, k.TenantID), nil
	}
	result := executeKnowledgeEdit(t, tool, editKnowledgeFileInput{Action: "edit", SHA256: knowledgeFileSHA256(svc.content),
		Edits: []SandboxEdit{{OldString: "old", NewString: "new"}},
	})
	require.False(t, result.Success)
	require.Contains(t, result.Error, "permission revoked")
	require.Equal(t, 1, svc.reads)
	require.Zero(t, svc.writes)
}
