package tools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/xuri/excelize/v2"
)

const maxKnowledgeEditFileBytes = 20 * 1024 * 1024

var knowledgeFileMutations = &fileMutationQueue{locks: map[string]*fileMutationLock{}}

type KnowledgeCellEdit struct {
	Sheet    string `json:"sheet"`
	Cell     string `json:"cell"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
}

type editKnowledgeFileInput struct {
	KnowledgeID string              `json:"knowledge_id"`
	Action      string              `json:"action"`
	SHA256      string              `json:"sha256,omitempty"`
	Sheet       string              `json:"sheet,omitempty"`
	Offset      int                 `json:"offset,omitempty"`
	Limit       int                 `json:"limit,omitempty"`
	Edits       []SandboxEdit       `json:"edits,omitempty"`
	Cells       []KnowledgeCellEdit `json:"cells,omitempty"`
	SourcePath  string              `json:"source_path,omitempty"`
}

// EditKnowledgeFileTool changes original knowledge files, not their extracted
// chunks. The application service owns storage, rollback and re-indexing.
type EditKnowledgeFileTool struct {
	BaseTool
	knowledgeService interfaces.KnowledgeService
	targets          types.SearchTargets
	writableKBIDs    []string
	authorize        func(context.Context, *types.Knowledge) (context.Context, error)
	store            SandboxFileSink
	sessionID        string
}

func NewEditKnowledgeFileTool(
	service interfaces.KnowledgeService,
	targets types.SearchTargets,
	writableKBIDs []string,
	authorize func(context.Context, *types.Knowledge) (context.Context, error),
) *EditKnowledgeFileTool {
	return &EditKnowledgeFileTool{
		BaseTool: NewBaseTool(ToolEditKnowledgeFile,
			`Read and modify an existing knowledge-base source file in the user's authorized scope.
Always action=read first and pass its sha256 on every edit or replacement. Find documents with list_documents/search_knowledge; knowledge_id is their dN handle.
For UTF-8 text, action=edit uses edits[{old_string,new_string,replace_all?}]; matches must be exact and unique unless replace_all=true.
For .xlsx, read returns sheet names and rows with 1-based row numbers and values in column order (A,B,...). Read another page with offset=next_offset. action=edit uses cells[{sheet,cell,old_value,new_value}], e.g. cell="D7". An old_value for a formula must be "=" followed by the formula shown in formulas. New values are literal text; existing formulas/styles in other cells are preserved.
When a sandbox is available, read also stages the original file and returns source_path. Edit that copy using shell/file tools (e.g. openpyxl/python-docx), then action=replace_file with its source_path to save any supported file format.
Copy source_path exactly. Document handles such as dN select knowledge records; they are not filesystem directories. Never construct a file path from a document handle.
Only change what the user requested. A successful write preserves the knowledge ID and starts asynchronous re-indexing; do not claim indexing has completed.`,
			json.RawMessage(`{
 "type":"object",
 "properties":{
  "knowledge_id":{"type":"string","minLength":1,"description":"dN document handle from retrieval or runtime context"},
  "action":{"type":"string","enum":["read","edit","replace_file"]},
  "sha256":{"type":"string","description":"Original file version from the most recent read; required for edit/replace_file"},
  "sheet":{"type":"string","description":"Sheet to read; defaults to first sheet"},
  "offset":{"type":"integer","minimum":0,"description":"0-based row offset for XLSX, character offset for text"},
  "limit":{"type":"integer","minimum":1,"maximum":100,"description":"XLSX rows per read (default 30, max 100)"},
  "edits":{"type":"array","maxItems":100,"items":{"type":"object","properties":{"old_string":{"type":"string"},"new_string":{"type":"string"},"replace_all":{"type":"boolean"}},"required":["old_string","new_string"]}},
  "cells":{"type":"array","maxItems":200,"items":{"type":"object","properties":{"sheet":{"type":"string"},"cell":{"type":"string"},"old_value":{"type":"string"},"new_value":{"type":"string"}},"required":["sheet","cell","old_value","new_value"]}},
  "source_path":{"type":"string","description":"Session sandbox file of the same extension to save with replace_file"}
 },"required":["knowledge_id","action"]}`)),
		knowledgeService: service, targets: targets, writableKBIDs: slices.Clone(writableKBIDs), authorize: authorize,
	}
}

func (t *EditKnowledgeFileTool) WithSandbox(store SandboxFileSink, sessionID string) {
	t.store, t.sessionID = store, sessionID
}

func knowledgeEditError(err error) (*types.ToolResult, error) {
	return &types.ToolResult{Success: false, Error: err.Error()}, nil
}

func (t *EditKnowledgeFileTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	var input editKnowledgeFileInput
	if err := json.Unmarshal(args, &input); err != nil {
		return knowledgeEditError(fmt.Errorf("invalid file edit arguments"))
	}
	input.KnowledgeID = strings.TrimSpace(input.KnowledgeID)
	if input.Action != "read" && input.Action != "edit" && input.Action != "replace_file" {
		return knowledgeEditError(fmt.Errorf("action must be read, edit or replace_file"))
	}
	if input.Offset < 0 || input.Limit < 0 || len(input.Edits) > 100 || len(input.Cells) > 200 {
		return knowledgeEditError(fmt.Errorf("invalid paging or too many edits"))
	}
	if t.authorize == nil {
		return knowledgeEditError(fmt.Errorf("knowledge file editing is not enabled for this agent"))
	}
	release := knowledgeFileMutations.lockFile(input.KnowledgeID)
	defer release()
	knowledge, err := authorizeKnowledgeInSearchTargets(ctx, t.targets, input.KnowledgeID, t.knowledgeService)
	if err != nil {
		return knowledgeEditError(err)
	}
	if !slices.Contains(t.writableKBIDs, knowledge.KnowledgeBaseID) {
		return knowledgeEditError(fmt.Errorf("this knowledge base is read-only for the current conversation"))
	}
	if knowledge.Type != "file" {
		return knowledgeEditError(fmt.Errorf("this tool edits uploaded file knowledge only"))
	}
	scoped, err := t.authorize(ctx, knowledge)
	if err != nil {
		return knowledgeEditError(err)
	}
	reader, filename, err := t.knowledgeService.GetKnowledgeFile(scoped, knowledge.ID)
	if err != nil {
		return knowledgeEditError(fmt.Errorf("cannot read knowledge source file: %w", err))
	}
	original, err := io.ReadAll(io.LimitReader(reader, maxKnowledgeEditFileBytes+1))
	_ = reader.Close()
	if err != nil || len(original) > maxKnowledgeEditFileBytes {
		return knowledgeEditError(fmt.Errorf("cannot read file or file exceeds 20 MiB"))
	}
	version := knowledgeFileSHA256(original)
	data := map[string]interface{}{
		"knowledge_id": knowledge.ID, "knowledge_base_id": knowledge.KnowledgeBaseID,
		"file_name": filename, "sha256": version, "changed": false,
	}
	if input.Action == "read" {
		return t.read(ctx, knowledge, filename, original, input, data)
	}
	if input.SHA256 == "" || input.SHA256 != version {
		return knowledgeEditError(fmt.Errorf("file version changed or sha256 is missing; read the original again before editing"))
	}
	var replacement []byte
	switch input.Action {
	case "replace_file":
		if len(input.Edits) != 0 || len(input.Cells) != 0 {
			return knowledgeEditError(fmt.Errorf("replace_file accepts source_path, not edits or cells"))
		}
		replacement, err = t.replacement(ctx, filename, input.SourcePath)
	case "edit":
		if input.SourcePath != "" || (len(input.Edits) == 0) == (len(input.Cells) == 0) {
			return knowledgeEditError(fmt.Errorf("edit requires exactly one of edits or cells, without source_path"))
		}
		if strings.EqualFold(path.Ext(filename), ".xlsx") && len(input.Cells) > 0 {
			replacement, err = editKnowledgeWorkbook(original, input.Cells)
		} else if knowledgeFileIsText(filename, original) && len(input.Edits) > 0 {
			var edited string
			edited, _, err = applySandboxEdits(string(original), input.Edits)
			replacement = []byte(edited)
		} else {
			err = fmt.Errorf("use cells for XLSX, edits for UTF-8 text, or replace_file with a sandbox file for other formats")
		}
	}
	if err != nil {
		return knowledgeEditError(err)
	}
	if len(replacement) > maxKnowledgeEditFileBytes {
		return knowledgeEditError(fmt.Errorf("replacement exceeds 20 MiB"))
	}
	if bytes.Equal(original, replacement) {
		return knowledgeEditResult(data, "The requested content already matches; no file was changed.")
	}
	// Recheck permissions after preparation, before the application service
	// commits the source and enqueues indexing.
	scoped, err = t.authorize(ctx, knowledge)
	if err != nil {
		return knowledgeEditError(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", path.Base(filename))
	if err != nil {
		return knowledgeEditError(err)
	}
	if _, err = part.Write(replacement); err != nil {
		return knowledgeEditError(err)
	}
	if err = writer.Close(); err != nil {
		return knowledgeEditError(err)
	}
	form, err := multipart.NewReader(&body, writer.Boundary()).ReadForm(maxKnowledgeEditFileBytes)
	if err != nil {
		return knowledgeEditError(err)
	}
	defer func() { _ = form.RemoveAll() }()
	updated, err := t.knowledgeService.ReplaceKnowledgeFile(scoped, knowledge.ID, form.File["file"][0], filename, nil)
	var duplicate *types.DuplicateKnowledgeError
	if err != nil && !errors.As(err, &duplicate) {
		return knowledgeEditError(fmt.Errorf("file was not saved: %w", err))
	}
	data["sha256"] = knowledgeFileSHA256(replacement)
	data["changed"] = err == nil
	if updated != nil {
		data["parse_status"] = updated.ParseStatus
	}
	return knowledgeEditResult(data, "Knowledge source file saved with the same knowledge ID. Re-indexing runs asynchronously; do not report indexing as completed.")
}

func knowledgeFileSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func knowledgeFileIsText(filename string, data []byte) bool {
	switch strings.ToLower(path.Ext(filename)) {
	case ".txt", ".md", ".markdown", ".csv", ".tsv", ".json", ".yaml", ".yml", ".xml", ".html", ".htm", ".rst", ".log", ".feature":
		return utf8.Valid(data) && !bytes.ContainsRune(data, 0)
	default:
		return false
	}
}

func knowledgeEditResult(data map[string]interface{}, message string) (*types.ToolResult, error) {
	encoded, _ := json.Marshal(data)
	// Keep the version ahead of potentially long previews, including when the
	// registry has to truncate the remaining output to its context budget.
	header := fmt.Sprintf("\nknowledge_id=%v sha256=%v\n", data["knowledge_id"], data["sha256"])
	return &types.ToolResult{Success: true, Output: message + header + string(encoded), Data: data}, nil
}

func (t *EditKnowledgeFileTool) read(ctx context.Context, knowledge *types.Knowledge, filename string, original []byte,
	input editKnowledgeFileInput, data map[string]interface{},
) (*types.ToolResult, error) {
	if t.store != nil && t.sessionID != "" {
		layout, err := lookupWorkspaceLayout(ctx, t.sessionID, t.store)
		if err != nil {
			return knowledgeEditError(err)
		}
		version := knowledgeFileSHA256(original)
		// Executable paths must not contain durable document IDs: the model
		// context codec would replace those substrings with dN handles.
		documentKey := "doc-" + knowledgeFileSHA256([]byte(knowledge.ID))[:16]
		filePath := path.Join(layout.Root, "knowledge", documentKey, version[:12], path.Base(filename))
		stat, statErr := t.store.StatSessionFile(ctx, t.sessionID, filePath)
		if statErr != nil || stat == nil {
			if err := t.store.WriteSessionWorkspaceFile(ctx, t.sessionID, filePath, original); err != nil {
				return knowledgeEditError(fmt.Errorf("cannot stage original file in sandbox: %w", err))
			}
		}
		data["source_path"] = filePath
	}
	budget := max(256, OutputBudget(ctx)-1500)
	switch {
	case strings.EqualFold(path.Ext(filename), ".xlsx"):
		if err := readKnowledgeWorkbook(original, input, data, budget); err != nil {
			return knowledgeEditError(err)
		}
	case knowledgeFileIsText(filename, original):
		runes := []rune(string(original))
		if input.Offset > len(runes) {
			return knowledgeEditError(fmt.Errorf("offset is beyond the end of the file"))
		}
		end := min(len(runes), input.Offset+budget/2)
		data["content"] = string(runes[input.Offset:end])
		data["offset"] = input.Offset
		data["next_offset"] = end
		data["has_more"] = end < len(runes)
	default:
		data["note"] = "Edit the sandbox copy with the file's native library, then use replace_file. A sandbox must be configured on this agent for binary files other than XLSX."
	}
	return knowledgeEditResult(data, "Original source file. Keep sha256 for the edit; offsets are rows for XLSX and characters for text.")
}

func openKnowledgeWorkbook(data []byte) (*excelize.File, error) {
	return excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit: 128 * 1024 * 1024, UnzipXMLSizeLimit: 8 * 1024 * 1024,
	})
}

func readKnowledgeWorkbook(original []byte, input editKnowledgeFileInput, data map[string]interface{}, budget int) error {
	book, err := openKnowledgeWorkbook(original)
	if err != nil {
		return fmt.Errorf("cannot open XLSX workbook: %w", err)
	}
	defer func() { _ = book.Close() }()
	names := book.GetSheetList()
	data["sheets"] = names
	sheet := input.Sheet
	if sheet == "" && len(names) > 0 {
		sheet = names[0]
	}
	rows, err := book.Rows(sheet)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	limit := input.Limit
	if limit == 0 {
		limit = 30
	}
	limit = min(limit, 100)
	output := []map[string]interface{}{}
	position, used, hasMore := 0, 0, false
	for rows.Next() {
		position++
		if position <= input.Offset {
			continue
		}
		if len(output) >= limit {
			hasMore = true
			break
		}
		values, err := rows.Columns(excelize.Options{RawCellValue: true})
		if err != nil {
			return err
		}
		formulas := map[string]string{}
		for col := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, position)
			formula, err := book.GetCellFormula(sheet, cell)
			if err != nil {
				return err
			}
			if formula != "" {
				formulas[cell] = "=" + formula
			}
		}
		row := map[string]interface{}{"row": position, "values": values, "formulas": formulas}
		encoded, _ := json.Marshal(row)
		size := utf8.RuneCount(encoded)
		if used+size > budget {
			if len(output) == 0 {
				return fmt.Errorf("this worksheet row exceeds the read budget; use the sandbox copy to inspect it")
			}
			hasMore = true
			break
		}
		output = append(output, row)
		used += size
	}
	if err := rows.Error(); err != nil {
		return err
	}
	data["sheet"], data["rows"] = sheet, output
	data["next_offset"] = input.Offset + len(output)
	data["has_more"] = hasMore
	return nil
}

func editKnowledgeWorkbook(original []byte, cells []KnowledgeCellEdit) ([]byte, error) {
	book, err := openKnowledgeWorkbook(original)
	if err != nil {
		return nil, err
	}
	defer func() { _ = book.Close() }()
	seen := map[string]bool{}
	for _, edit := range cells {
		col, row, err := excelize.CellNameToCoordinates(edit.Cell)
		if err != nil {
			return nil, err
		}
		cell, err := excelize.CoordinatesToCellName(col, row)
		if err != nil {
			return nil, err
		}
		key := edit.Sheet + "\x00" + cell
		if seen[key] {
			return nil, fmt.Errorf("duplicate cell edit: %s!%s", edit.Sheet, cell)
		}
		seen[key] = true
		value, err := book.GetCellValue(edit.Sheet, cell, excelize.Options{RawCellValue: true})
		if err != nil {
			return nil, err
		}
		formula, err := book.GetCellFormula(edit.Sheet, cell)
		if err != nil {
			return nil, err
		}
		if formula != "" {
			value = "=" + formula
		}
		if value != edit.OldValue {
			return nil, fmt.Errorf("cell %s!%s does not match old_value; read the file again", edit.Sheet, cell)
		}
		if err := book.SetCellStr(edit.Sheet, cell, edit.NewValue); err != nil {
			return nil, err
		}
	}
	buffer, err := book.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (t *EditKnowledgeFileTool) replacement(ctx context.Context, filename, sourcePath string) ([]byte, error) {
	if t.store == nil || t.sessionID == "" {
		return nil, fmt.Errorf("replace_file requires a sandbox configured on this agent")
	}
	if sourcePath == "" || !strings.EqualFold(path.Ext(filename), path.Ext(sourcePath)) {
		return nil, fmt.Errorf("source_path must be a sandbox file with the same extension as the original")
	}
	stat, err := t.store.StatSessionFile(ctx, t.sessionID, sourcePath)
	if err != nil || stat == nil || stat.Type != "file" || stat.Size > maxKnowledgeEditFileBytes {
		return nil, fmt.Errorf("replacement is missing, not a file, or exceeds 20 MiB")
	}
	data, err := t.store.ReadSessionFile(ctx, t.sessionID, sourcePath)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > maxKnowledgeEditFileBytes {
		return nil, fmt.Errorf("replacement is empty or exceeds 20 MiB")
	}
	return data, nil
}
