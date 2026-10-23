package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type approvalRecordingTool struct {
	tools.BaseTool
	called bool
	args   json.RawMessage
}

func (t *approvalRecordingTool) Execute(ctx context.Context, args json.RawMessage) (*types.ToolResult, error) {
	t.called = true
	t.args = append(json.RawMessage(nil), args...)
	return &types.ToolResult{Success: true}, nil
}

func TestManualApprovalBlocksExecutionUntilDecision(t *testing.T) {
	for _, outcome := range []string{"approve", "reject", "cancel", "delayed", "modified", "invalid"} {
		t.Run(outcome, func(t *testing.T) {
			engine := newTestEngine(t, &mockChat{})
			engine.config.ToolApprovalMode = types.ToolApprovalManual
			engine.toolRegistry = tools.NewToolRegistry()
			tool := &approvalRecordingTool{BaseTool: tools.NewBaseTool(tools.ToolShellExec, "Run a command", json.RawMessage(`{"type":"object","required":["command"],"properties":{"command":{"type":"string"}}}`))}
			engine.toolRegistry.RegisterTool(tool)
			// No MCP checker is needed to enforce a user-requested builtin approval.
			gate := approval.NewGate(&config.Config{Agent: &config.AgentConfig{ToolApprovalTimeoutSeconds: 1}}, nil, nil)
			engine.SetToolApprovalGate(gate)
			pending := make(chan event.ToolApprovalRequiredData, 1)
			engine.eventBus.On(event.EventToolApprovalRequired, func(_ context.Context, evt event.Event) error {
				pending <- evt.Data.(event.ToolApprovalRequiredData)
				return nil
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
			ctx = types.WithExecutionTenant(ctx, 99)
			ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "alice"})
			done := make(chan types.ToolCall, 1)
			go func() {
				done <- engine.runToolCall(ctx, types.LLMToolCall{ID: "call", Function: types.FunctionCall{Name: tools.ToolShellExec, Arguments: `{"command":"pwd"}`}}, 0, 0, 0, "session", "answer")
			}()
			var request event.ToolApprovalRequiredData
			select {
			case request = <-pending:
			case <-ctx.Done():
				t.Fatal("approval was not requested")
			}
			require.False(t, tool.called, "tool must not run before authorization")
			require.Equal(t, "call", request.ToolCallID)
			require.Equal(t, uint64(7), request.TenantID, "approval belongs to the caller even for a shared agent")
			require.JSONEq(t, `{"command":"pwd"}`, request.ArgsJSON)
			if outcome == "cancel" {
				cancel()
			} else {
				if outcome == "delayed" {
					select {
					case <-done:
						t.Fatal("tool approval expired before the human decision")
					case <-time.After(1100 * time.Millisecond):
					}
				}
				decision := approval.Decision{Approved: outcome != "reject"}
				if outcome == "modified" {
					decision.ModifiedArgs = json.RawMessage(`{"command":"ls"}`)
				}
				if outcome == "invalid" {
					decision.ModifiedArgs = json.RawMessage(`{"unexpected":true}`)
				}
				principal, _ := types.PrincipalFromContext(ctx)
				require.NoError(t, gate.Resolve(7, principal.StorageID(), request.PendingID, decision))
			}
			var call types.ToolCall
			select {
			case call = <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("approval did not unblock")
			}
			if outcome == "reject" || outcome == "cancel" || outcome == "invalid" {
				require.False(t, tool.called)
				require.False(t, call.Result.Success)
			} else {
				require.True(t, tool.called)
				require.True(t, call.Result.Success, call.Result.Error)
				command := "pwd"
				if outcome == "modified" {
					command = "ls"
				}
				require.Equal(t, command, call.Args["command"])
				require.JSONEq(t, `{"command":"`+command+`"}`, string(tool.args))
			}
		})
	}
}

func TestApprovalModeKeepsReadsAutomaticAndFailsClosedWithoutGate(t *testing.T) {
	for _, mode := range []types.ToolApprovalMode{"", types.ToolApprovalAuto, types.ToolApprovalManual} {
		for _, name := range []string{tools.ToolReadFile, tools.ToolShellExec} {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				engine := newTestEngine(t, &mockChat{})
				engine.config.ToolApprovalMode = mode
				engine.toolRegistry = tools.NewToolRegistry()
				tool := &approvalRecordingTool{BaseTool: tools.NewBaseTool(name, "", json.RawMessage(`{"type":"object"}`))}
				engine.toolRegistry.RegisterTool(tool)
				call := engine.runToolCall(context.Background(), types.LLMToolCall{ID: "call", Function: types.FunctionCall{Name: name, Arguments: `{}`}}, 0, 0, 0, "session", "answer")
				blocked := mode == types.ToolApprovalManual && name == tools.ToolShellExec
				require.Equal(t, !blocked, tool.called)
				require.Equal(t, !blocked, call.Result.Success)
			})
		}
	}
}
