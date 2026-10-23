package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
)

func (e *AgentEngine) SetToolApprovalGate(gate approval.MCPApproval) {
	e.toolApprovalGate = gate
}

// MCP tools gate the concrete server call themselves, including calls through
// discovery proxies. Builtins are gated here before starting their exec timeout.
func (e *AgentEngine) approveBuiltinTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	if e.config.ToolApprovalMode != types.ToolApprovalManual || agenttools.CanRunConcurrently(name) {
		return args, nil
	}
	switch name {
	case agenttools.ToolThinking, agenttools.ToolTodoWrite, agenttools.ToolDiscoverMCPTools, agenttools.ToolCallMCPTool:
		return args, nil
	}
	tool, err := e.toolRegistry.GetTool(name)
	if err != nil {
		return nil, err
	}
	switch tool.(type) {
	case *agenttools.MCPTool, *agenttools.MCPRegisteredTool:
		return args, nil
	}
	meta, ok := agenttools.ToolExecFromContext(ctx)
	if !ok || meta.EventBus == nil || e.toolApprovalGate == nil {
		return nil, fmt.Errorf("manual tool approval is unavailable; execution blocked")
	}
	tenantID := types.CallerFromContext(ctx).TenantID
	decision, err := e.toolApprovalGate.RequestAndWait(ctx, approval.PendingRequest{
		TenantID: tenantID, UserID: meta.UserID,
		SessionID: meta.SessionID, AssistantMessageID: meta.AssistantMessageID,
		RequestID: meta.RequestID, EventBus: meta.EventBus,
		ServiceName: "WeKnora", MCPToolName: name, RegisteredToolName: name,
		Description: tool.Description(), Args: args, ToolCallID: meta.ToolCallID,
	})
	if err != nil {
		return nil, fmt.Errorf("tool approval failed: %w", err)
	}
	if !decision.Approved {
		reason := decision.Reason
		if reason == "" {
			reason = "tool execution rejected by user"
		}
		return nil, fmt.Errorf("tool approval denied: %s", reason)
	}
	if len(decision.ModifiedArgs) > 0 {
		var object map[string]any
		if err := json.Unmarshal(decision.ModifiedArgs, &object); err != nil || object == nil {
			return nil, fmt.Errorf("approved arguments must be a non-null JSON object")
		}
		return decision.ModifiedArgs, nil
	}
	return args, nil
}
