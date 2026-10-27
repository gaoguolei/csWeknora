package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
)

var errToolApprovalRejected = errors.New("tool approval denied")

func (e *AgentEngine) SetToolApprovalGate(gate approval.MCPApproval) {
	e.toolApprovalGate = gate
}

// MCP tools gate the concrete server call themselves, including calls through
// discovery proxies. Builtins are gated here before starting their exec timeout.
func (e *AgentEngine) approveBuiltinTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	mode := e.config.ToolApprovalMode
	if mode == types.ToolApprovalPlan {
		var err error
		args, err = e.approveTaskPlan(ctx, name, args)
		if err != nil {
			return nil, err
		}
	}
	if (mode != types.ToolApprovalManual && mode != types.ToolApprovalAcceptEdits) || agenttools.CanRunConcurrently(name) {
		return args, nil
	}
	if mode == types.ToolApprovalAcceptEdits {
		switch name {
		case agenttools.ToolWriteSandboxFile, agenttools.ToolEditSandboxFile, agenttools.ToolEditKnowledgeFile:
			return args, nil
		}
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
		ServiceName: "ThinkerFlow", MCPToolName: name, RegisteredToolName: name,
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
		if decision.ContextCanceled || decision.TimedOut {
			return nil, fmt.Errorf("tool approval denied: %s", reason)
		}
		return nil, fmt.Errorf("%w: %s", errToolApprovalRejected, reason)
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

// A rejection is a control decision, not a tool failure for the model to retry.
// Finish directly so no recovery round can re-issue the denied action or leak
// the provider's tool-call markup into the answer.
func (e *AgentEngine) finishRejectedToolApproval(ctx context.Context, step types.AgentStep, state *types.AgentState, sessionID string) bool {
	for _, call := range step.ToolCalls {
		if call.Result == nil || !call.Result.ApprovalRejected {
			continue
		}
		if ctx.Err() != nil {
			return true
		}
		answer := "You rejected this operation. Execution has stopped. You can send new instructions."
		if strings.HasPrefix(types.LanguageFromContextOrDefault(ctx), "zh") {
			answer = "已拒绝本次操作，执行已停止。你可以发送新的指令。"
		}
		answerID := generateEventID("answer")
		_ = e.eventBus.Emit(ctx, event.Event{
			ID:        answerID,
			Type:      event.EventAgentFinalAnswer,
			SessionID: sessionID,
			Data:      event.AgentFinalAnswerData{Content: answer},
		})
		e.closeAnswerStream(ctx, sessionID, answerID, false)
		state.FinalAnswer = answer
		state.IsComplete = true
		return true
	}
	return false
}
