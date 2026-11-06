package agent

import (
	"context"
	"fmt"

	agenttools "github.com/Tencent/WeKnora/internal/agent/tools"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

// Keep already-obtained evidence out of the tool-free synthesis path until
// the testing assistant has had a bounded opportunity to save it. These rounds
// use the ordinary executor, including approval, cancellation and error handling.
const testingResultSaveRounds = 3

func (e *AgentEngine) persistTestingResults(ctx context.Context, state *types.AgentState, query string, messages *[]chat.Message, tools []chat.Tool, sessionID, messageID string, guards *loopGuards) error {
	if !e.config.TestingMode || state.IsComplete || ctx.Err() != nil || countTotalToolCalls(state.RoundSteps) == 0 || len(tools) == 0 {
		return nil
	}
	// Do not offer another browser/execution exploration cycle. Keep only
	// tools needed to write a result file or inspect/repair that write.
	saveTools := make([]chat.Tool, 0, len(tools))
	canWrite := false
	for _, tool := range tools {
		switch tool.Function.Name {
		case agenttools.ToolWriteSandboxFile, agenttools.ToolEditSandboxFile, agenttools.ToolShellExec:
			canWrite = true
			saveTools = append(saveTools, tool)
		case agenttools.ToolReadFile:
			saveTools = append(saveTools, tool)
		}
	}
	if !canWrite {
		return nil
	}
	e.testingResultFinalization = true
	defer func() { e.testingResultFinalization = false }()
	for attempt := 0; attempt < testingResultSaveRounds && !state.IsComplete && ctx.Err() == nil; attempt++ {
		outcome, err := e.runReActIteration(ctx, state, messages, saveTools, sessionID, messageID, query, guards)
		if err != nil {
			return err
		}
		if outcome == iterOutcomeBreak {
			break
		}
		if outcome == iterOutcomeNext {
			state.CurrentRound++
		}
	}
	return ctx.Err()
}

func (e *AgentEngine) withTestingResultPersistence(messages []chat.Message, iteration int) []chat.Message {
	if !e.config.TestingMode {
		return messages
	}
	instruction := "Testing result persistence: after each executed test batch or follow-up verification, save the observed per-case results with tools before doing further exploration. " +
		"The case workbench reads saved files, not answer prose. Preserve original case IDs. A case can remain 待确认 while its Actual result evidence must still be updated. " +
		"Use write_sandbox_file to save a small UTF-8 CSV under the runtime workspace artifact output test-reports/ directory with columns Case ID, Status, Actual result. " +
		"Use only observed evidence; never infer a pass. Reuse the same CSV for this turn's cumulative results, preserving earlier rows. Optional Excel/HTML updates come after this checkpoint. " +
		"Do not require spreadsheet libraries just to save CSV. Only claim saved results after a successful tool response."
	if e.testingResultFinalization {
		instruction += "\nThe main execution-round limit has been reached. This is a bounded result-save phase: stop new tests and browser exploration. " +
			"Use the existing tool evidence to save or verify this turn's latest results now, including pending-confirmation evidence. " +
			"Do not ask the user to save results or claim tools are unavailable: the supplied tools remain callable under their normal permissions. " +
			"If no tests were executed or no write capability is available, explain that explicitly without inventing evidence. Finish once persistence succeeds, or report the concrete save failure."
	} else if limit := e.executionIterationLimit(); limit > 0 && limit-iteration <= testingResultSaveRounds {
		instruction += fmt.Sprintf("\nOnly %d execution rounds remain. Save current results now before spending any more rounds on optional checks.", max(0, limit-iteration))
	}
	// Ephemeral runtime guidance survives compaction without being persisted as
	// a user message or mutating the caller's system prompt.
	copied := append([]chat.Message(nil), messages...)
	for i := range copied {
		if copied[i].Role == "system" {
			copied[i].Content += "\n\n" + instruction
			return copied
		}
	}
	return append([]chat.Message{{Role: "system", Content: instruction}}, copied...)
}
