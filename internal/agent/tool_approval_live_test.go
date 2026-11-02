package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/agent/approval"
	"github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/event"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/require"
)

func TestLivePermissionChangesAffectNextToolAndReleaseCurrentWait(t *testing.T) {
	engine := newTestEngine(t, &mockChat{})
	engine.config.ToolApprovalMode = types.ToolApprovalAuto
	policy := types.NewToolApprovalPolicy(types.ToolApprovalAuto)
	engine.config.ToolApprovalPolicy = policy
	engine.toolRegistry = tools.NewToolRegistry()
	engine.toolRegistry.RegisterTool(newCountingTool(tools.ToolShellExec))
	engine.SetToolApprovalGate(approval.NewGate(nil, nil, nil))
	requests := 0
	engine.eventBus.On(event.EventToolApprovalRequired, func(context.Context, event.Event) error {
		requests++
		// Switching to auto during this prompt must continue without a separate approval.
		policy.Set(types.ToolApprovalAuto)
		return nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	ctx = tools.WithToolExecContext(ctx, &tools.ToolExecContext{EventBus: engine.eventBus, SessionID: "s", AssistantMessageID: "a"})
	_, err := engine.approveBuiltinTool(ctx, tools.ToolShellExec, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Zero(t, requests)
	policy.Set(types.ToolApprovalManual)
	_, err = engine.approveBuiltinTool(ctx, tools.ToolShellExec, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, 1, requests, "the engine must use the changed policy rather than the original auto setting")
	_, err = engine.approveBuiltinTool(ctx, tools.ToolShellExec, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Equal(t, 1, requests, "auto remains active for subsequent commands")
}
