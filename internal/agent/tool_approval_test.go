package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

type approvalRecordingTool struct {
	tools.BaseTool
	called bool
	args   json.RawMessage
}

func TestRejectedApprovalStopsTurnAndSkipsRemainingTools(t *testing.T) {
	for _, mode := range []types.ToolApprovalMode{types.ToolApprovalManual, types.ToolApprovalPlan} {
		for _, parallel := range []bool{false, true} {
			t.Run(string(mode)+fmt.Sprintf("/parallel=%v", parallel), func(t *testing.T) {
				name, args := tools.ToolShellExec, `{}`
				if mode == types.ToolApprovalPlan {
					name, args = tools.ToolTodoWrite, `{"steps":[{"id":"1","description":"Inspect the project","status":"pending"}]}`
				}
				model := &mockChat{responses: []mockResponse{
					{chunks: []types.StreamResponse{{Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{
						{ID: "before", Function: types.FunctionCall{Name: tools.ToolReadFile, Arguments: `{}`}},
						{ID: "denied", Function: types.FunctionCall{Name: name, Arguments: args}},
						{ID: "after-read", Function: types.FunctionCall{Name: tools.ToolReadFile, Arguments: `{}`}},
						{ID: "after-write", Function: types.FunctionCall{Name: tools.ToolWriteSandboxFile, Arguments: `{}`}},
					}}}},
					{chunks: []types.StreamResponse{{Content: `<｜DSML｜calls><｜DSML｜invoke name="shell_exec">`, Done: true}}},
				}}
				engine := newTestEngine(t, model)
				engine.config.ToolApprovalMode = mode
				engine.config.ParallelToolCalls = parallel
				engine.toolRegistry = tools.NewToolRegistry()
				read, denied, write := newCountingTool(tools.ToolReadFile), newCountingTool(name), newCountingTool(tools.ToolWriteSandboxFile)
				for _, tool := range []types.Tool{read, denied, write} {
					engine.toolRegistry.RegisterTool(tool)
				}
				gate := approval.NewGate(nil, nil, nil)
				engine.SetToolApprovalGate(gate)
				approvals, results, completions, answerDone := 0, 0, 0, 0
				var answer string
				engine.eventBus.On(event.EventToolApprovalRequired, func(ctx context.Context, evt event.Event) error {
					approvals++
					request := evt.Data.(event.ToolApprovalRequiredData)
					principal, _ := types.PrincipalFromContext(ctx)
					return gate.Resolve(request.TenantID, principal.StorageID(), request.PendingID, approval.Decision{Approved: false})
				})
				engine.eventBus.On(event.EventAgentToolResult, func(context.Context, event.Event) error { results++; return nil })
				engine.eventBus.On(event.EventAgentComplete, func(context.Context, event.Event) error { completions++; return nil })
				engine.eventBus.On(event.EventAgentFinalAnswer, func(_ context.Context, evt event.Event) error {
					data := evt.Data.(event.AgentFinalAnswerData)
					answer += data.Content
					if data.Done {
						answerDone++
					}
					return nil
				})
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				ctx = context.WithValue(ctx, types.LanguageContextKey, "zh-CN")
				state := &types.AgentState{}
				_, err := engine.executeLoop(ctx, state, "start", []chat.Message{{Role: "user", Content: "start"}}, nil, "session", "message")
				require.NoError(t, err)
				require.True(t, state.IsComplete)
				require.Equal(t, 1, model.callCount, "rejection must not start another model round or final synthesis")
				require.Equal(t, 1, approvals, "later tools must not request approval again")
				require.Equal(t, 1, read.calls, "reads after rejection must be skipped")
				require.Zero(t, denied.calls)
				require.Zero(t, write.calls)
				require.Len(t, state.RoundSteps, 1)
				require.Len(t, state.RoundSteps[0].ToolCalls, 4)
				require.Equal(t, 4, results, "all pending tool cards must receive a result")
				require.Equal(t, 1, completions)
				require.Equal(t, 1, answerDone)
				require.Equal(t, state.FinalAnswer, answer)
				require.Contains(t, answer, "已拒绝")
				require.NotContains(t, answer, "DSML")
			})
		}
	}
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
				require.Equal(t, outcome == "reject", call.Result.ApprovalRejected, "cancellation and invalid arguments are not human rejections")
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
	for _, mode := range []types.ToolApprovalMode{"", types.ToolApprovalAuto, types.ToolApprovalManual, types.ToolApprovalAcceptEdits, types.ToolApprovalBypass} {
		for _, name := range []string{tools.ToolReadFile, tools.ToolShellExec, tools.ToolWriteSandboxFile} {
			t.Run(string(mode)+"/"+name, func(t *testing.T) {
				engine := newTestEngine(t, &mockChat{})
				engine.config.ToolApprovalMode = mode
				engine.toolRegistry = tools.NewToolRegistry()
				tool := &approvalRecordingTool{BaseTool: tools.NewBaseTool(name, "", json.RawMessage(`{"type":"object"}`))}
				engine.toolRegistry.RegisterTool(tool)
				call := engine.runToolCall(context.Background(), types.LLMToolCall{ID: "call", Function: types.FunctionCall{Name: name, Arguments: `{}`}}, 0, 0, 0, "session", "answer")
				blocked := (mode == types.ToolApprovalManual && name != tools.ToolReadFile) || (mode == types.ToolApprovalAcceptEdits && name == tools.ToolShellExec)
				require.Equal(t, !blocked, tool.called)
				require.Equal(t, !blocked, call.Result.Success)
			})
		}
	}
}

func TestToolFailureAfterApprovalCanStillRecover(t *testing.T) {
	model := &mockChat{responses: []mockResponse{
		{chunks: []types.StreamResponse{{Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{
			{ID: "call", Function: types.FunctionCall{Name: tools.ToolShellExec, Arguments: `{}`}},
		}}}},
		{chunks: []types.StreamResponse{{Content: "The command failed.", Done: true}}},
	}}
	engine := newTestEngine(t, model)
	engine.config.ToolApprovalMode = types.ToolApprovalManual
	engine.toolRegistry = tools.NewToolRegistry()
	engine.toolRegistry.RegisterTool(&orderedTestTool{
		BaseTool: tools.NewBaseTool(tools.ToolShellExec, "", json.RawMessage(`{"type":"object"}`)),
		run: func(context.Context) *types.ToolResult {
			return &types.ToolResult{Success: false, Error: "tool approval denied: text returned by the command"}
		},
	})
	gate := approval.NewGate(nil, nil, nil)
	engine.SetToolApprovalGate(gate)
	engine.eventBus.On(event.EventToolApprovalRequired, func(ctx context.Context, evt event.Event) error {
		request := evt.Data.(event.ToolApprovalRequiredData)
		principal, _ := types.PrincipalFromContext(ctx)
		return gate.Resolve(request.TenantID, principal.StorageID(), request.PendingID, approval.Decision{Approved: true})
	})
	state := &types.AgentState{}
	_, err := engine.executeLoop(context.Background(), state, "start", emptyMessages(), nil, "session", "message")
	require.NoError(t, err)
	require.Equal(t, 2, model.callCount)
	require.True(t, state.IsComplete)
	require.Equal(t, "The command failed.", state.FinalAnswer)
}

func TestBypassPermissionsCompletesWithAnOrdinaryUnfinishedChecklist(t *testing.T) {
	model := &mockChat{responses: []mockResponse{
		{chunks: []types.StreamResponse{{Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{
			{ID: "todo", Function: types.FunctionCall{Name: tools.ToolTodoWrite, Arguments: `{"task":"Build page","steps":[{"id":"1","description":"Build page","status":"in_progress"}]}`}},
			{ID: "shell", Function: types.FunctionCall{Name: tools.ToolShellExec, Arguments: `{}`}},
		}}}},
		{chunks: []types.StreamResponse{{Content: "Done.", Done: true}}},
	}}
	engine := newTestEngine(t, model)
	engine.config.ToolApprovalMode = types.ToolApprovalBypass
	engine.toolRegistry = tools.NewToolRegistry()
	engine.toolRegistry.RegisterTool(tools.NewTodoWriteTool())
	shell := newCountingTool(tools.ToolShellExec)
	engine.toolRegistry.RegisterTool(shell)
	gate := approval.NewGate(nil, nil, nil)
	engine.SetToolApprovalGate(gate)
	approvals := 0
	engine.eventBus.On(event.EventToolApprovalRequired, func(ctx context.Context, evt event.Event) error {
		approvals++
		request := evt.Data.(event.ToolApprovalRequiredData)
		principal, _ := types.PrincipalFromContext(ctx)
		return gate.Resolve(request.TenantID, principal.StorageID(), request.PendingID, approval.Decision{Approved: false})
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state := &types.AgentState{}
	_, err := engine.executeLoop(ctx, state, "start", emptyMessages(), nil, "session", "message")
	require.NoError(t, err)
	require.True(t, state.IsComplete)
	require.Equal(t, "Done.", state.FinalAnswer)
	require.Zero(t, approvals)
	require.Equal(t, 1, shell.calls)
	require.Len(t, state.RoundSteps[0].ToolCalls, 2)
	result := state.RoundSteps[0].ToolCalls[0].Result
	require.True(t, result.Success, result.Error)
	require.Equal(t, false, result.Data["plan_approved"], "bypass todos must not become a persistent approved plan")
	require.False(t, engine.planMustContinue(), "a stale ordinary checklist must not keep bypass execution alive")
}
