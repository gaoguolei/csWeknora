package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Tencent/WeKnora/internal/types"
)

type WorkspaceTestsArguments struct {
	Action    string   `json:"action"`
	RunID     string   `json:"run_id"`
	ProjectID string   `json:"project_id"`
	TargetID  string   `json:"target_id"`
	TargetIDs []string `json:"target_ids"`
	Timeout   int      `json:"timeout"`
}

type WorkspaceTestsTool struct {
	execute func(context.Context, WorkspaceTestsArguments) (any, error)
}

func NewWorkspaceTestsTool(execute func(context.Context, WorkspaceTestsArguments) (any, error)) *WorkspaceTestsTool {
	return &WorkspaceTestsTool{execute: execute}
}
func (*WorkspaceTestsTool) Name() string { return "workspace_tests" }
func (*WorkspaceTestsTool) Description() string {
	return "Discover and run project validation in the current session sandbox. Actions: discover, run, status, history, cancel. Use this for structured tests/builds instead of guessing shell commands. Run returns run_id; poll status until completed/cancelled/timed_out/error. Read actual checks and cases before claiming success; unknown/no_tests/blocked are not passes. HTML and JSON reports are automatically saved to session artifacts. Source changes invalidate older reports."
}
func (*WorkspaceTestsTool) Parameters() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"action":{"type":"string","enum":["discover","run","status","history","cancel"]},"run_id":{"type":"string"},"project_id":{"type":"string","description":"Project ID from discover; empty runs all projects"},"target_id":{"type":"string"},"target_ids":{"type":"array","items":{"type":"string"}},"timeout":{"type":"integer","minimum":10,"maximum":900}},"required":["action"]}`)
}
func (t *WorkspaceTestsTool) Execute(ctx context.Context, raw json.RawMessage) (*types.ToolResult, error) {
	var args WorkspaceTestsArguments
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("invalid validation arguments: %w", err)
	}
	if (args.Action == "status" || args.Action == "cancel") && args.RunID == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	result, err := t.execute(ctx, args)
	if err != nil {
		return &types.ToolResult{Success: false, Error: err.Error(), Output: err.Error()}, nil
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	var object map[string]interface{}
	_ = json.Unmarshal(data, &object)
	return &types.ToolResult{Success: true, Output: string(data), Data: object}, nil
}
