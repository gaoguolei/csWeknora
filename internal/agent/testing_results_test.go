package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func testResultToolResponse(id, name, args string) mockResponse {
	return mockResponse{chunks: []types.StreamResponse{{Done: true, FinishReason: "tool_calls", ToolCalls: []types.LLMToolCall{{ID: id, Function: types.FunctionCall{Name: name, Arguments: args}}}}}}
}

func TestTestingRoundLimitSavesResultsBeforeToolFreeSummary(t *testing.T) {
	csv := "Case ID,Status,Actual result\nTC-07,待确认,三个子场景 URL 与结果数已核对\n"
	args, err := json.Marshal(map[string]string{"path": "/workspace/output/test-reports/follow-up.csv", "content": csv})
	require.NoError(t, err)
	model := &mockChat{responses: []mockResponse{
		testResultToolResponse("check", agenttools.ToolShellExec, `{}`),
		testResultToolResponse("save", agenttools.ToolWriteSandboxFile, string(args)),
		{chunks: []types.StreamResponse{{Content: "已保存最新证据，TC-07 仍待确认。", Done: true, FinishReason: "stop"}}},
	}}
	engine := newTestEngine(t, model, withMaxIterations(1))
	engine.config.TestingMode = true
	engine.toolRegistry = agenttools.NewToolRegistry()
	check := newCountingTool(agenttools.ToolShellExec)
	save := &approvalRecordingTool{BaseTool: agenttools.NewBaseTool(agenttools.ToolWriteSandboxFile, "Save", json.RawMessage(`{"type":"object"}`))}
	engine.toolRegistry.RegisterTool(check)
	engine.toolRegistry.RegisterTool(save)
	state, err := engine.executeLoop(context.Background(), &types.AgentState{}, "复验 TC-07", emptyMessages(), nil, "session", "message")
	require.NoError(t, err)
	require.True(t, save.called, "reaching the round limit must not disable tools before the save attempt")
	require.JSONEq(t, string(args), string(save.args))
	require.True(t, state.IsComplete)
	require.Len(t, model.calls, 3)
	require.NotEmpty(t, model.opts[1].Tools)
	require.NotEqual(t, "none", model.opts[1].ToolChoice)
	require.Contains(t, model.calls[1][0].Content, "bounded result-save phase")
	require.Contains(t, state.FinalAnswer, "已保存最新证据")
	require.False(t, engine.testingResultFinalization)
}

func TestTestingResultSavePhaseIsBounded(t *testing.T) {
	responses := []mockResponse{}
	for i := 0; i < 1+testingResultSaveRounds; i++ {
		responses = append(responses, testResultToolResponse("call", agenttools.ToolShellExec, `{}`))
	}
	responses = append(responses, mockResponse{chunks: []types.StreamResponse{{Content: "结果尚未保存。", Done: true, FinishReason: "stop"}}})
	model := &mockChat{responses: responses}
	engine := newTestEngine(t, model, withMaxIterations(1))
	engine.config.TestingMode = true
	engine.toolRegistry = agenttools.NewToolRegistry()
	engine.toolRegistry.RegisterTool(newCountingTool(agenttools.ToolShellExec))
	state, err := engine.executeLoop(context.Background(), &types.AgentState{}, "test", emptyMessages(), nil, "session", "message")
	require.NoError(t, err)
	require.True(t, state.IsComplete)
	require.Len(t, model.calls, 2+testingResultSaveRounds)
	require.Equal(t, "none", model.opts[len(model.opts)-1].ToolChoice)
}

func TestTestingPersistenceGuidanceIsScopedAndDoesNotMutateHistory(t *testing.T) {
	engine := newTestEngine(t, &mockChat{}, withMaxIterations(4))
	messages := emptyMessages()
	require.Equal(t, messages, engine.withTestingResultPersistence(messages, 2))
	engine.config.TestingMode = true
	guided := engine.withTestingResultPersistence(messages, 2)
	require.Contains(t, guided[0].Content, "Only 2 execution rounds remain")
	require.Contains(t, guided[0].Content, "Actual result evidence must still be updated")
	require.False(t, strings.Contains(messages[0].Content, "Testing result persistence"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, engine.persistTestingResults(ctx, &types.AgentState{}, "test", &messages, nil, "session", "message", &loopGuards{}))
}

func TestTestingResultSaveFailureCanRetryBeforeSummary(t *testing.T) {
	model := &mockChat{responses: []mockResponse{
		testResultToolResponse("check", agenttools.ToolShellExec, `{}`),
		testResultToolResponse("failed-save", agenttools.ToolWriteSandboxFile, `{}`),
		testResultToolResponse("retry-save", agenttools.ToolWriteSandboxFile, `{}`),
		{chunks: []types.StreamResponse{{Content: "结果文件已保存。", Done: true, FinishReason: "stop"}}},
	}}
	engine := newTestEngine(t, model, withMaxIterations(1))
	engine.config.TestingMode = true
	engine.toolRegistry = agenttools.NewToolRegistry()
	engine.toolRegistry.RegisterTool(newCountingTool(agenttools.ToolShellExec))
	writes := 0
	engine.toolRegistry.RegisterTool(&orderedTestTool{
		BaseTool: agenttools.NewBaseTool(agenttools.ToolWriteSandboxFile, "save", json.RawMessage(`{"type":"object"}`)),
		run: func(context.Context) *types.ToolResult {
			writes++
			if writes == 1 {
				return &types.ToolResult{Success: false, Output: "write failed: temporary storage error"}
			}
			return &types.ToolResult{Success: true, Output: "saved test-results.csv"}
		},
	})
	state, err := engine.executeLoop(context.Background(), &types.AgentState{}, "test", emptyMessages(), nil, "session", "message")
	require.NoError(t, err)
	require.Equal(t, 2, writes)
	require.True(t, state.IsComplete)
	require.Len(t, model.calls, 4)
	foundFailure := false
	for _, message := range model.calls[2] {
		if message.Role == "tool" && strings.Contains(message.Content, "temporary storage error") {
			foundFailure = true
		}
	}
	require.True(t, foundFailure, "retry must receive the real file-save failure")
}
