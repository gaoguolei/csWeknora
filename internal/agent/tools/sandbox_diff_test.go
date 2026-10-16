package tools

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeSandboxFileCallArgsStripsWriteBody(t *testing.T) {
	content := strings.Repeat("x\n", 15)
	out := SanitizeSandboxFileCallArgs(ToolWriteSandboxFile, map[string]any{
		"path":    "/workspace/output/a.py",
		"content": content,
		"mode":    "overwrite",
	})
	if _, ok := out["content"]; ok {
		t.Fatal("write progress must not carry the file body")
	}
	if out["path"] != "/workspace/output/a.py" {
		t.Fatalf("path = %v", out["path"])
	}
	if out["added_lines"] != 15 {
		t.Fatalf("added_lines = %v, want 15", out["added_lines"])
	}
	if out["removed_lines"] != 0 {
		t.Fatalf("removed_lines = %v", out["removed_lines"])
	}
	preview, _ := out["preview"].(string)
	if strings.Count(preview, "\n")+1 != sandboxFilePreviewMaxLines {
		t.Fatalf("preview lines = %q", preview)
	}
}

func TestSanitizeSandboxFileCallArgsStripsEditBodies(t *testing.T) {
	out := SanitizeSandboxFileCallArgs(ToolEditSandboxFile, map[string]any{
		"path": "/workspace/a.py",
		"edits": []any{
			map[string]any{"old_string": "a\nb\n", "new_string": "a\nb\nc\n"},
		},
	})
	if _, ok := out["edits"]; ok {
		t.Fatal("edit progress must not carry edits")
	}
	if out["removed_lines"] != 2 {
		t.Fatalf("removed_lines = %v, want 2", out["removed_lines"])
	}
	if out["added_lines"] != 3 {
		t.Fatalf("added_lines = %v, want 3", out["added_lines"])
	}
}

func TestSanitizeSandboxFileCallArgsLeavesOtherToolsAlone(t *testing.T) {
	args := map[string]any{"command": "ls"}
	out := SanitizeSandboxFileCallArgs(ToolShellExec, args)
	if out["command"] != "ls" {
		t.Fatalf("unrelated args = %#v", out)
	}
}

func TestAttachSandboxDiffContentKeepsBoundedSides(t *testing.T) {
	data := map[string]any{"path": "/workspace/a.py"}
	attachSandboxDiffContent(data, "before\n", "after\n")
	if data["diff_before"] != "before\n" || data["diff_after"] != "after\n" {
		t.Fatalf("diff payload = %#v", data)
	}
}

func TestAttachSandboxDiffContentDropsOversizedSides(t *testing.T) {
	data := map[string]any{"path": "/workspace/a.py"}
	attachSandboxDiffContent(data, strings.Repeat("x\n", sandboxDiffPayloadMaxBytes), "after\n")
	if _, ok := data["diff_before"]; ok {
		t.Fatal("oversized before side must be dropped")
	}
	if _, ok := data["diff_after"]; ok {
		t.Fatal("after side must be dropped with the before side")
	}
}

// The edit result carries both sides so the code view can render a real diff
// without re-reading the sandbox.
func TestEditSandboxFileResultCarriesDiffSides(t *testing.T) {
	path := "/workspace/run.py"
	original := "DEBUG = True\n"
	editor := &fakeSandboxFileEditor{files: map[string][]byte{path: []byte(original)}}

	result, err := NewEditSandboxFileTool(editor).Execute(sandboxFileTestContext(), json.RawMessage(
		`{"path":"/workspace/run.py","edits":[{"old_string":"DEBUG = True","new_string":"DEBUG = False"}]}`))

	require.NoError(t, err)
	require.True(t, result.Success, result.Error)
	assert.Equal(t, original, result.Data["diff_before"])
	assert.Equal(t, "DEBUG = False\n", result.Data["diff_after"])
	_, hasContent := result.Data["content"]
	assert.False(t, hasContent)
}

// A fresh write has no before side; an overwrite of an existing file does.
func TestWriteSandboxFileResultCarriesDiffSides(t *testing.T) {
	sink := &fakeSandboxFileSink{}
	first, err := NewWriteSandboxFileTool(sink, 0).Execute(
		sandboxFileTestContext(), mustWriteSandboxArgs("/workspace/scratch.py", "print(1)\n"),
	)
	require.NoError(t, err)
	require.True(t, first.Success, first.Error)
	assert.Equal(t, "", first.Data["diff_before"])
	assert.Equal(t, "print(1)\n", first.Data["diff_after"])

	second, err := NewWriteSandboxFileTool(sink, 0).Execute(
		sandboxFileTestContext(), mustWriteSandboxArgs("/workspace/scratch.py", "print(2)\n"),
	)
	require.NoError(t, err)
	require.True(t, second.Success, second.Error)
	assert.Equal(t, "print(1)\n", second.Data["diff_before"])
	assert.Equal(t, "print(2)\n", second.Data["diff_after"])
}
